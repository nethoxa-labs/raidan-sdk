package gossipsub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"

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

type joinedTopic struct {
	topic        *pubsub.Topic
	subscription *pubsub.Subscription
}

// Publisher keeps one gossipsub router and its joined topics on an existing
// libp2p host. It does not own or close the host.
type Publisher struct {
	ctx    context.Context
	host   host.Host
	pubsub *pubsub.PubSub

	mu     sync.Mutex
	topics map[string]*joinedTopic
	closed bool
}

// NewPublisher starts a persistent gossipsub router on an already connected
// consensus host.
func NewPublisher(ctx context.Context, h host.Host) (*Publisher, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if h == nil {
		return nil, errors.New("nil gossipsub host")
	}
	ps, err := pubsub.NewGossipSub(
		ctx,
		h,
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign),
		pubsub.WithNoAuthor(),
		pubsub.WithMaxMessageSize(maxWireMessageSize),
	)
	if err != nil {
		return nil, fmt.Errorf("new gossipsub: %w", err)
	}
	return &Publisher{
		ctx:    ctx,
		host:   h,
		pubsub: ps,
		topics: make(map[string]*joinedTopic),
	}, nil
}

func (p *Publisher) join(topicName string) (*joinedTopic, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, errors.New("gossipsub publisher closed")
	}
	if joined := p.topics[topicName]; joined != nil {
		return joined, nil
	}
	topic, err := p.pubsub.Join(topicName)
	if err != nil {
		return nil, fmt.Errorf("join topic %q: %w", topicName, err)
	}
	subscription, err := topic.Subscribe()
	if err != nil {
		_ = topic.Close()
		return nil, fmt.Errorf("subscribe topic %q: %w", topicName, err)
	}
	joined := &joinedTopic{topic: topic, subscription: subscription}
	p.topics[topicName] = joined
	return joined, nil
}

// Publish publishes one message while retaining the peer connection, router,
// and topic membership for subsequent protocol actions.
func (p *Publisher) Publish(topicName string, data []byte, opts Options) error {
	if p == nil {
		return errors.New("nil gossipsub publisher")
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
	publishCtx, cancel := context.WithTimeout(p.ctx, session.Timeout(p.ctx, opts.Timeout))
	defer cancel()

	joined, err := p.join(topicName)
	if err != nil {
		return err
	}
	if _, err := waitForGossipPeer(publishCtx, joined.topic, topicName, opts); err != nil {
		return err
	}
	if opts.BeforePublish != nil {
		if err := opts.BeforePublish(); err != nil {
			return fmt.Errorf("before gossip publish: %w", err)
		}
	}
	session.ObserveWrite(publishCtx, session.Write{Protocol: "gossipsub", Topic: topicName, Payload: data})
	if err := joined.topic.Publish(publishCtx, data); err != nil {
		return fmt.Errorf("publish %q: %w", topicName, err)
	}
	if opts.After <= 0 {
		return nil
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

// Await reads the next message published by the remote peer on topicName.
func (p *Publisher) Await(topicName string, timeout time.Duration) ([]byte, error) {
	if p == nil {
		return nil, errors.New("nil gossipsub publisher")
	}
	joined, err := p.join(topicName)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	awaitCtx, cancel := context.WithTimeout(p.ctx, session.Timeout(p.ctx, timeout))
	defer cancel()
	for {
		message, err := joined.subscription.Next(awaitCtx)
		if err != nil {
			return nil, err
		}
		if message.ReceivedFrom == p.host.ID() {
			continue
		}
		return append([]byte(nil), message.Data...), nil
	}
}

// Close leaves all topics and cancels their subscriptions. The underlying host
// remains owned by the req/resp session that created it.
func (p *Publisher) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	var closeErr error
	for name, joined := range p.topics {
		joined.subscription.Cancel()
		if err := joined.topic.Close(); err != nil && closeErr == nil {
			closeErr = fmt.Errorf("close topic %q: %w", name, err)
		}
	}
	clear(p.topics)
	return closeErr
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
	publisher, err := NewPublisher(publishCtx, s.Host())
	if err != nil {
		return err
	}
	defer func() { _ = publisher.Close() }()
	return publisher.Publish(topic, data, opts)
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
