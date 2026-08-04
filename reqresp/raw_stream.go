package reqresp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

// OpenStream opens one raw protocol stream on this connected peer session.
// The caller owns the returned stream.
func (s *Session) OpenStream(ctx context.Context, protocolID string, timeout time.Duration) (network.Stream, error) {
	if s == nil || s.host == nil {
		return nil, errors.New("consensus session is nil")
	}
	if protocolID == "" {
		return nil, errors.New("protocol ID is empty")
	}
	if ctx == nil {
		ctx = s.ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	timeout = session.Timeout(ctx, timeout)
	openCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	s.pinExplicitAddrs()
	stream, err := s.host.NewStream(openCtx, s.peerID, protocol.ID(protocolID))
	if err != nil {
		return nil, fmt.Errorf("open stream %s: %w", protocolID, err)
	}
	if err := stream.SetDeadline(time.Now().Add(timeout)); err != nil {
		_ = stream.Reset()
		return nil, fmt.Errorf("set stream deadline: %w", err)
	}
	return stream, nil
}
