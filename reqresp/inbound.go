package reqresp

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/golang/snappy"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

// AcceptOptions controls one inbound request/response stream.
type AcceptOptions struct {
	NoBody     bool
	MaxPayload uint64
	Timeout    time.Duration
}

// InboundRequest is one request stream initiated by the connected peer.
type InboundRequest struct {
	ctx      context.Context
	protocol string
	payload  []byte
	stream   network.Stream
}

// Protocol returns the negotiated request/response protocol identifier.
func (r *InboundRequest) Protocol() string {
	if r == nil {
		return ""
	}
	return r.protocol
}

// Payload returns a copy of the decoded request body.
func (r *InboundRequest) Payload() []byte {
	if r == nil {
		return nil
	}
	return append([]byte(nil), r.payload...)
}

// WriteResponse writes one canonical response chunk.
func (r *InboundRequest) WriteResponse(chunk ResponseChunk) error {
	if r == nil || r.stream == nil {
		return errors.New("inbound request is nil")
	}
	if err := WriteResponseChunk(r.stream, chunk); err != nil {
		_ = r.stream.Reset()
		return err
	}
	session.ObserveWrite(r.ctx, session.Write{
		Protocol:   "reqresp",
		ProtocolID: r.protocol,
		Payload:    append([]byte(nil), chunk.Payload...),
	})
	return nil
}

// WriteRaw writes exact caller-owned response bytes.
func (r *InboundRequest) WriteRaw(payload []byte) error {
	if r == nil || r.stream == nil {
		return errors.New("inbound request is nil")
	}
	if err := writeFull(r.stream, payload); err != nil {
		_ = r.stream.Reset()
		return fmt.Errorf("write raw response: %w", err)
	}
	session.ObserveWrite(r.ctx, session.Write{
		Protocol:   "reqresp",
		ProtocolID: r.protocol,
		Payload:    append([]byte(nil), payload...),
		Raw:        true,
	})
	return nil
}

// Close completes the response stream.
func (r *InboundRequest) Close() error {
	if r == nil || r.stream == nil {
		return nil
	}
	return r.stream.Close()
}

// Reset aborts the response stream.
func (r *InboundRequest) Reset() error {
	if r == nil || r.stream == nil {
		return nil
	}
	return r.stream.Reset()
}

// Accept waits for one peer-initiated request on proto.
func (s *Session) Accept(ctx context.Context, proto string, options AcceptOptions) (*InboundRequest, error) {
	if s == nil || s.host == nil {
		return nil, errors.New("consensus session is nil")
	}
	if proto == "" {
		return nil, errors.New("request protocol is empty")
	}
	if ctx == nil {
		ctx = s.ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if options.Timeout <= 0 {
		options.Timeout = 8 * time.Second
	}
	if options.MaxPayload == 0 {
		options.MaxPayload = MaxPayloadSize
	}
	if options.MaxPayload > MaxPayloadSize {
		return nil, fmt.Errorf("max request payload %d exceeds hard ceiling %d", options.MaxPayload, MaxPayloadSize)
	}

	waitCtx, cancel := context.WithTimeout(ctx, session.Remaining(ctx, options.Timeout))
	defer cancel()

	requests := make(chan *InboundRequest, 1)
	errorsCh := make(chan error, 1)
	protocolID := protocol.ID(proto)
	s.host.SetStreamHandler(protocolID, func(stream network.Stream) {
		_ = stream.SetDeadline(time.Now().Add(session.Remaining(ctx, options.Timeout)))
		payload := []byte(nil)
		var err error
		if !options.NoBody {
			payload, err = ReadSSZSnappy(stream, options.MaxPayload)
		}
		if err != nil {
			_ = stream.Reset()
			select {
			case errorsCh <- fmt.Errorf("read inbound request: %w", err):
			default:
			}
			return
		}
		request := &InboundRequest{ctx: ctx, protocol: proto, payload: payload, stream: stream}
		select {
		case requests <- request:
		default:
			_ = stream.Reset()
		}
	})
	defer s.host.RemoveStreamHandler(protocolID)

	select {
	case request := <-requests:
		return request, nil
	case err := <-errorsCh:
		return nil, err
	case <-waitCtx.Done():
		return nil, fmt.Errorf("wait for inbound %s request: %w", proto, waitCtx.Err())
	}
}

// ReadSSZSnappy decodes one bounded length-prefixed framed-snappy body.
func ReadSSZSnappy(r io.Reader, maxPayload uint64) ([]byte, error) {
	if r == nil {
		return nil, errors.New("request reader is nil")
	}
	if maxPayload == 0 || maxPayload > MaxPayloadSize {
		return nil, fmt.Errorf("max request payload %d is outside 1..%d", maxPayload, MaxPayloadSize)
	}
	reader := bufio.NewReader(r)
	length, err := binary.ReadUvarint(reader)
	if err != nil {
		return nil, fmt.Errorf("read request length prefix: %w", err)
	}
	if length > maxPayload {
		return nil, fmt.Errorf("request length %d exceeds max %d", length, maxPayload)
	}
	if length == 0 {
		return nil, nil
	}
	compressedLimit, err := maxCompressedLen(length)
	if err != nil {
		return nil, err
	}
	payload := make([]byte, int(length))
	if _, err := io.ReadFull(snappy.NewReader(io.LimitReader(reader, compressedLimit)), payload); err != nil {
		return nil, fmt.Errorf("read snappy request payload: %w", err)
	}
	return payload, nil
}

// WriteResponseChunk writes one canonical response chunk.
func WriteResponseChunk(w io.Writer, chunk ResponseChunk) error {
	if w == nil {
		return errors.New("response writer is nil")
	}
	if chunk.Code == CodeSuccess && len(chunk.Context) != 0 && len(chunk.Context) != ContextBytesLen {
		return fmt.Errorf("response context length %d must be %d", len(chunk.Context), ContextBytesLen)
	}
	if err := writeFull(w, []byte{chunk.Code}); err != nil {
		return fmt.Errorf("write response code: %w", err)
	}
	if chunk.Code == CodeSuccess && len(chunk.Context) != 0 {
		if err := writeFull(w, chunk.Context); err != nil {
			return fmt.Errorf("write response context: %w", err)
		}
	}
	if err := WriteSSZSnappy(w, chunk.Payload); err != nil {
		return fmt.Errorf("write response payload: %w", err)
	}
	return nil
}
