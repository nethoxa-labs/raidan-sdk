package reqresp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"
	libp2p "github.com/libp2p/go-libp2p"
	mplex "github.com/libp2p/go-libp2p-mplex"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

const (
	consensusDialTimeout       = 10 * time.Second
	consensusDialSpacing       = 350 * time.Millisecond
	consensusDialGateRetention = time.Minute
	consensusDialGateMaxKeys   = 1024
	consensusRequestAttempts   = 3
)

var consensusDialGate = struct {
	sync.Mutex
	next map[string]time.Time
}{next: make(map[string]time.Time)}

// Session is one connected consensus request/response peer.
type Session struct {
	ctx    context.Context
	host   host.Host
	peerID peer.ID
	addrs  []ma.Multiaddr
}

// NewSession connects a transient libp2p host to a consensus peer.
func NewSession(ctx context.Context, beaconURL, p2pAddr string) (*Session, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	dialCtx, cancel := context.WithTimeout(ctx, session.Timeout(ctx, consensusDialTimeout))
	defer cancel()

	peerID, err := PeerID(ctx, beaconURL, p2pAddr)
	if err != nil {
		return nil, err
	}
	addr, err := Multiaddr(p2pAddr, peerID)
	if err != nil {
		return nil, err
	}
	if err := waitConsensusDialTurn(dialCtx, p2pAddr); err != nil {
		return nil, err
	}
	info, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return nil, fmt.Errorf("parse peer multiaddr: %w", err)
	}
	explicitAddrs := append([]ma.Multiaddr(nil), info.Addrs...)
	h, err := libp2p.New(
		libp2p.NoTransports,
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.NoListenAddrs,
		libp2p.DefaultMuxers,
		libp2p.Muxer(mplex.ID, mplex.DefaultTransport),
		libp2p.UserAgent("raidan-sdk/0.1"),
	)
	if err != nil {
		return nil, fmt.Errorf("new libp2p host: %w", err)
	}
	h.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.TempAddrTTL)
	if err := h.Connect(dialCtx, *info); err != nil {
		_ = h.Close()
		return nil, fmt.Errorf("connect consensus peer: %w", err)
	}
	return &Session{ctx: ctx, host: h, peerID: info.ID, addrs: explicitAddrs}, nil
}

// Close releases the libp2p host and all of its streams.
func (s *Session) Close() error {
	if s == nil || s.host == nil {
		return nil
	}
	return s.host.Close()
}

// Host returns the connected libp2p host for protocols such as gossipsub that
// share the same consensus peer transport.
func (s *Session) Host() host.Host {
	if s == nil {
		return nil
	}
	return s.host
}

func (s *Session) pinExplicitAddrs() {
	if s == nil || s.host == nil || len(s.addrs) == 0 {
		return
	}
	s.host.Peerstore().ClearAddrs(s.peerID)
	s.host.Peerstore().AddAddrs(s.peerID, s.addrs, peerstore.TempAddrTTL)
}

func waitConsensusDialTurn(ctx context.Context, key string) error {
	wait := reserveConsensusDialTurn(key, time.Now())
	if wait == 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func reserveConsensusDialTurn(key string, now time.Time) time.Duration {
	consensusDialGate.Lock()
	defer consensusDialGate.Unlock()

	for candidate, next := range consensusDialGate.next {
		if !next.Add(consensusDialGateRetention).After(now) {
			delete(consensusDialGate.next, candidate)
		}
	}
	if _, exists := consensusDialGate.next[key]; !exists && len(consensusDialGate.next) >= consensusDialGateMaxKeys {
		var oldestKey string
		var oldest time.Time
		for candidate, next := range consensusDialGate.next {
			if oldestKey == "" || next.Before(oldest) {
				oldestKey, oldest = candidate, next
			}
		}
		delete(consensusDialGate.next, oldestKey)
	}

	var wait time.Duration
	if next := consensusDialGate.next[key]; next.After(now) {
		wait = next.Sub(now)
		now = next
	}
	consensusDialGate.next[key] = now.Add(consensusDialSpacing)
	return wait
}

func retryableConsensusConnectError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "connect consensus peer") ||
		strings.Contains(message, "open stream") && strings.Contains(message, "connection failed") ||
		strings.Contains(message, "failed to dial") ||
		strings.Contains(message, "failed to negotiate stream multiplexer") ||
		strings.Contains(message, "failed to negotiate security protocol") ||
		strings.Contains(message, "use of closed network connection") ||
		strings.Contains(message, "i/o timeout")
}

func consensusRetrySleep(ctx context.Context, attempt int) error {
	if ctx == nil {
		ctx = context.Background()
	}
	timer := time.NewTimer(time.Duration(attempt+1) * 500 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// PeerID resolves the peer ID from a full multiaddress or the beacon identity API.
func PeerID(ctx context.Context, beaconURL, p2pAddr string) (peer.ID, error) {
	if strings.HasPrefix(p2pAddr, "/") && strings.Contains(p2pAddr, "/p2p/") {
		addr, err := ma.NewMultiaddr(p2pAddr)
		if err == nil {
			info, err := peer.AddrInfoFromP2pAddr(addr)
			if err == nil {
				return info.ID, nil
			}
		}
	}
	var identity struct {
		Data struct {
			PeerID string `json:"peer_id"`
		} `json:"data"`
	}
	if err := consensusGetJSON(ctx, beaconURL, "/eth/v1/node/identity", &identity); err != nil {
		return "", fmt.Errorf("fetch node identity: %w", err)
	}
	if identity.Data.PeerID == "" {
		return "", errors.New("node identity returned empty peer_id")
	}
	id, err := peer.Decode(identity.Data.PeerID)
	if err != nil {
		return "", fmt.Errorf("decode peer id: %w", err)
	}
	return id, nil
}

// Multiaddr normalizes a libp2p multiaddress or host:port endpoint.
func Multiaddr(p2pAddr string, peerID peer.ID) (ma.Multiaddr, error) {
	if strings.HasPrefix(p2pAddr, "enr:") {
		node, err := enode.Parse(enode.ValidSchemes, p2pAddr)
		if err != nil {
			return nil, fmt.Errorf("parse consensus ENR: %w", err)
		}
		ip, port := node.IP(), node.TCP()
		if ip == nil || ip.IsUnspecified() {
			return nil, errors.New("consensus ENR has no routable IP address")
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("consensus ENR has invalid TCP port %d", port)
		}
		protocol := "ip6"
		if ip.To4() != nil {
			protocol = "ip4"
		}
		return ma.NewMultiaddr(fmt.Sprintf("/%s/%s/tcp/%d/p2p/%s", protocol, ip.String(), port, peerID))
	}
	if strings.HasPrefix(p2pAddr, "/") {
		if strings.Contains(p2pAddr, "/p2p/") {
			return ma.NewMultiaddr(p2pAddr)
		}
		return ma.NewMultiaddr(p2pAddr + "/p2p/" + peerID.String())
	}
	host, port, err := net.SplitHostPort(p2pAddr)
	if err != nil {
		return nil, fmt.Errorf("split p2p address %q: %w", p2pAddr, err)
	}
	protocol := "dns4"
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() != nil {
			protocol = "ip4"
		} else {
			protocol = "ip6"
		}
	}
	return ma.NewMultiaddr(fmt.Sprintf("/%s/%s/tcp/%s/p2p/%s", protocol, host, port, peerID))
}
