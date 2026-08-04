package gossipsub

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

const rawRPCTimeout = 8 * time.Second

// OpenRPCStream opens one negotiated gossipsub stream. The caller owns it.
func OpenRPCStream(ctx context.Context, h host.Host, peerID peer.ID) (network.Stream, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if h == nil {
		return nil, errors.New("nil gossipsub host")
	}
	if peerID == "" {
		return nil, errors.New("empty gossipsub peer ID")
	}
	timeout := session.Timeout(ctx, rawRPCTimeout)
	if timeout <= 0 {
		return nil, context.DeadlineExceeded
	}
	openCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	stream, err := h.NewStream(
		openCtx,
		peerID,
		pubsub.GossipSubID_v13,
		pubsub.GossipSubID_v12,
		pubsub.GossipSubID_v11,
		pubsub.GossipSubID_v10,
	)
	if err != nil {
		return nil, fmt.Errorf("open gossipsub stream: %w", err)
	}
	if err := stream.SetDeadline(time.Now().Add(timeout)); err != nil {
		_ = stream.Reset()
		return nil, fmt.Errorf("set gossipsub stream deadline: %w", err)
	}
	return stream, nil
}

// SendRPC writes one caller-encoded protobuf RPC over a negotiated gossipsub
// stream. The supplied bytes exclude the unsigned-varint stream prefix.
func SendRPC(ctx context.Context, h host.Host, peerID peer.ID, rpc []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if h == nil {
		return errors.New("nil gossipsub host")
	}
	if peerID == "" {
		return errors.New("empty gossipsub peer ID")
	}
	sendCtx, cancel := context.WithTimeout(ctx, session.Timeout(ctx, rawRPCTimeout))
	defer cancel()
	stream, err := OpenRPCStream(sendCtx, h, peerID)
	if err != nil {
		return err
	}
	defer func() { _ = stream.Close() }()

	session.ObserveWrite(sendCtx, session.Write{
		Protocol:   "gossipsub",
		ProtocolID: string(stream.Protocol()),
		Payload:    rpc,
		Raw:        true,
	})
	var prefix [10]byte
	prefixLength := binary.PutUvarint(prefix[:], uint64(len(rpc)))
	if err := writeRPC(stream, prefix[:prefixLength]); err != nil {
		_ = stream.Reset()
		return fmt.Errorf("write gossipsub RPC length: %w", err)
	}
	if err := writeRPC(stream, rpc); err != nil {
		_ = stream.Reset()
		return fmt.Errorf("write gossipsub RPC: %w", err)
	}
	if err := stream.CloseWrite(); err != nil {
		_ = stream.Reset()
		return fmt.Errorf("close gossipsub write side: %w", err)
	}
	return nil
}

func writeRPC(writer io.Writer, payload []byte) error {
	for len(payload) > 0 {
		n, err := writer.Write(payload)
		if n < 0 || n > len(payload) {
			return fmt.Errorf("invalid write count %d for %d-byte buffer", n, len(payload))
		}
		payload = payload[n:]
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
