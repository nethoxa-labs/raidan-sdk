package reqresp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

// RequestOptions controls one request/response exchange.
type RequestOptions struct {
	NoBody          bool
	RawBody         []byte
	HasRawBody      bool
	HasContext      bool
	Handshake       bool
	Timeout         time.Duration
	MaxPayload      uint64
	MaxTotalPayload uint64
	MaxChunks       uint64
	SkipResponse    bool
	// BeforeSend runs after setup and immediately before request bytes are written.
	BeforeSend func() error
}

// ResponseChunk is one decoded response frame.
type ResponseChunk struct {
	Code    byte
	Context []byte
	Payload []byte
}

// Request performs one exchange on an existing session.
func (s *Session) Request(proto string, payload []byte, options RequestOptions) (*ResponseChunk, error) {
	chunks, err := s.request(proto, payload, options, multiResponseProtocol(proto))
	if err != nil || len(chunks) == 0 {
		return nil, err
	}
	for _, chunk := range chunks {
		if chunk.Code != CodeSuccess {
			return chunk, nil
		}
	}
	return chunks[0], nil
}

func multiResponseProtocol(proto string) bool {
	return strings.Contains(proto, "_by_range/") || strings.Contains(proto, "_by_root/")
}

// RequestChunks performs one exchange and reads every response chunk until EOF.
func (s *Session) RequestChunks(proto string, payload []byte, options RequestOptions) ([]*ResponseChunk, error) {
	return s.request(proto, payload, options, true)
}

func (s *Session) request(proto string, payload []byte, options RequestOptions, all bool) ([]*ResponseChunk, error) {
	if options.Timeout == 0 {
		options.Timeout = 8 * time.Second
	}
	if options.MaxPayload == 0 {
		options.MaxPayload = MaxPayloadSize
	}
	if options.MaxTotalPayload == 0 {
		options.MaxTotalPayload = MaxTotalResponseSize
	}
	if options.MaxChunks == 0 {
		options.MaxChunks = MaxResponseChunks
	}
	ctx := s.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := session.Timeout(ctx, options.Timeout)
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	s.pinExplicitAddrs()
	stream, err := s.host.NewStream(requestCtx, s.peerID, protocol.ID(proto))
	if err != nil {
		return nil, fmt.Errorf("open stream %s: %w", proto, err)
	}
	defer func() { _ = stream.Close() }()
	if err := stream.SetDeadline(time.Now().Add(timeout)); err != nil {
		_ = stream.Reset()
		return nil, fmt.Errorf("set stream deadline: %w", err)
	}
	capturedBody := payload
	if options.NoBody {
		capturedBody = nil
	} else if options.HasRawBody {
		capturedBody = options.RawBody
	} else if framed, frameErr := RawSSZSnappy(payload); frameErr == nil {
		capturedBody = framed
	}
	if options.BeforeSend != nil {
		if err := options.BeforeSend(); err != nil {
			_ = stream.Reset()
			return nil, fmt.Errorf("before request send: %w", err)
		}
	}
	session.ObserveWrite(requestCtx, session.Write{Protocol: "reqresp", ProtocolID: proto, Payload: capturedBody})

	switch {
	case options.NoBody:
	case options.HasRawBody:
		if err := writeFull(stream, options.RawBody); err != nil {
			_ = stream.Reset()
			return nil, fmt.Errorf("write raw request: %w", err)
		}
	default:
		if err := WriteSSZSnappy(stream, payload); err != nil {
			_ = stream.Reset()
			return nil, err
		}
	}
	if err := stream.CloseWrite(); err != nil {
		_ = stream.Reset()
		return nil, fmt.Errorf("close write side: %w", err)
	}
	if options.SkipResponse {
		return nil, nil
	}
	responses, err := NewResponseReader(stream, options.HasContext, options.MaxPayload, options.MaxTotalPayload, options.MaxChunks)
	if err != nil {
		_ = stream.Reset()
		return nil, err
	}
	first, err := responses.Next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, ErrEmptyResponse
		}
		_ = stream.Reset()
		return nil, err
	}
	chunks := []*ResponseChunk{first}
	if !all || first.Code != CodeSuccess {
		return chunks, nil
	}
	for {
		chunk, err := responses.Next()
		if errors.Is(err, io.EOF) {
			return chunks, nil
		}
		if err != nil {
			_ = stream.Reset()
			return nil, err
		}
		chunks = append(chunks, chunk)
		if chunk.Code != CodeSuccess {
			return chunks, nil
		}
	}
}

// Request connects, performs one exchange, and closes the session.
func Request(ctx context.Context, beaconURL, p2pAddr, proto string, payload []byte, options RequestOptions) (*ResponseChunk, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	beforeSend := options.BeforeSend
	sendAttempted := false
	options.BeforeSend = func() error {
		if beforeSend != nil {
			if err := beforeSend(); err != nil {
				return err
			}
		}
		sendAttempted = true
		return nil
	}
	for attempt := 0; attempt < consensusRequestAttempts; attempt++ {
		peerSession, err := NewSession(ctx, beaconURL, p2pAddr)
		if err != nil {
			lastErr = err
			if attempt+1 == consensusRequestAttempts || !retryableConsensusConnectError(err) {
				return nil, err
			}
			if err := consensusRetrySleep(ctx, attempt); err != nil {
				return nil, err
			}
			continue
		}
		if options.Handshake {
			if err := peerSession.StatusWarmup(beaconURL); err != nil {
				_ = peerSession.Close()
				return nil, fmt.Errorf("status warmup: %w", err)
			}
		}
		response, err := peerSession.Request(proto, payload, options)
		_ = peerSession.Close()
		if err == nil || sendAttempted || attempt+1 == consensusRequestAttempts || !retryableConsensusConnectError(err) {
			return response, err
		}
		lastErr = err
		if err := consensusRetrySleep(ctx, attempt); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}

// RequestChunks connects, performs one multi-response exchange, and closes the session.
func RequestChunks(ctx context.Context, beaconURL, p2pAddr, proto string, payload []byte, options RequestOptions) ([]*ResponseChunk, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	beforeSend := options.BeforeSend
	sendAttempted := false
	options.BeforeSend = func() error {
		if beforeSend != nil {
			if err := beforeSend(); err != nil {
				return err
			}
		}
		sendAttempted = true
		return nil
	}
	var lastErr error
	for attempt := 0; attempt < consensusRequestAttempts; attempt++ {
		peerSession, err := NewSession(ctx, beaconURL, p2pAddr)
		if err != nil {
			lastErr = err
			if attempt+1 == consensusRequestAttempts || !retryableConsensusConnectError(err) {
				return nil, err
			}
			if err := consensusRetrySleep(ctx, attempt); err != nil {
				return nil, err
			}
			continue
		}
		if options.Handshake {
			if err := peerSession.StatusWarmup(beaconURL); err != nil {
				_ = peerSession.Close()
				return nil, fmt.Errorf("status warmup: %w", err)
			}
		}
		responses, err := peerSession.RequestChunks(proto, payload, options)
		_ = peerSession.Close()
		if err == nil || sendAttempted || attempt+1 == consensusRequestAttempts || !retryableConsensusConnectError(err) {
			return responses, err
		}
		lastErr = err
		if err := consensusRetrySleep(ctx, attempt); err != nil {
			return nil, err
		}
	}
	return nil, lastErr
}
