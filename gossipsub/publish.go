package gossipsub

import (
	"context"
	"errors"
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/nethoxa-labs/raidan-sdk/reqresp"
	"github.com/nethoxa-labs/raidan-sdk/session"
)

// MaxPayloadSize is the consensus gossip decoded-payload ceiling.
const MaxPayloadSize = 10 * 1024 * 1024

// maxWireMessageSize leaves bounded room for the pubsub RPC envelope around a
// maximum-size application payload. WithMaxMessageSize applies to the encoded
// RPC, not only Message.Data.
const maxWireMessageSize = MaxPayloadSize + 64*1024

// ErrNoPeers reports that no target peer subscribed to the requested topic.
var ErrNoPeers = errors.New("no gossipsub peers for topic")

// Options controls one gossip publication.
type Options struct {
	Handshake   bool
	WaitForPeer bool
	Timeout     time.Duration
	After       time.Duration
	// BeforePublish runs after the libp2p/gossipsub setup and immediately
	// before the target message is published.
	BeforePublish func() error
}

// Publish connects to a peer and publishes data on topic exactly once.
func Publish(ctx context.Context, beaconURL, p2pAddr, topic string, data []byte, opts Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(data) > MaxPayloadSize {
		return fmt.Errorf("gossip payload size %d exceeds maximum %d", len(data), MaxPayloadSize)
	}
	if opts.Timeout == 0 {
		opts.Timeout = 8 * time.Second
	}
	if opts.After == 0 {
		opts.After = 300 * time.Millisecond
	}
	publishCtx, cancel := context.WithTimeout(ctx, session.Timeout(ctx, opts.Timeout))
	defer cancel()

	s, err := reqresp.NewSession(publishCtx, beaconURL, p2pAddr)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()
	if opts.Handshake {
		if err := s.StatusWarmup(beaconURL); err != nil {
			return fmt.Errorf("status warmup: %w", err)
		}
	}
	ps, err := pubsub.NewGossipSub(
		publishCtx,
		s.Host(),
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign),
		pubsub.WithNoAuthor(),
		pubsub.WithMaxMessageSize(maxWireMessageSize),
	)
	if err != nil {
		return fmt.Errorf("new gossipsub: %w", err)
	}
	t, err := ps.Join(topic)
	if err != nil {
		return fmt.Errorf("join topic %q: %w", topic, err)
	}
	defer func() { _ = t.Close() }()
	if _, err := waitForGossipPeer(publishCtx, t, topic, opts); err != nil {
		return err
	}
	if opts.BeforePublish != nil {
		if err := opts.BeforePublish(); err != nil {
			return fmt.Errorf("before gossip publish: %w", err)
		}
	}
	session.ObserveWrite(publishCtx, session.Write{Protocol: "gossipsub", Topic: topic, Payload: data})
	if err := t.Publish(publishCtx, data); err != nil {
		return fmt.Errorf("publish %q: %w", topic, err)
	}
	timer := time.NewTimer(opts.After)
	defer timer.Stop()
	select {
	case <-publishCtx.Done():
		return publishCtx.Err()
	case <-timer.C:
		return nil
	}
}

func waitForGossipPeer(ctx context.Context, topic *pubsub.Topic, topicName string, opts Options) (int, error) {
	if !opts.WaitForPeer {
		return len(topic.ListPeers()), nil
	}
	wait := opts.Timeout / 2
	if wait <= 0 {
		wait = 4 * time.Second
	}
	wait = session.Timeout(ctx, wait)
	deadline := time.Now().Add(wait)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for len(topic.ListPeers()) == 0 && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-ticker.C:
		}
	}
	peers := len(topic.ListPeers())
	if peers == 0 {
		return 0, fmt.Errorf("%w %q after %s", ErrNoPeers, topicName, wait.Round(100*time.Millisecond))
	}
	return peers, nil
}
