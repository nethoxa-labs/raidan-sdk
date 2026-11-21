package reqresp

import (
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// TransportEndpoints are the socket endpoints advertised by one libp2p peer.
type TransportEndpoints struct {
	TCP  string
	UDP  string
	QUIC bool
}

// ResolveTransportEndpoints extracts TCP and UDP socket endpoints from a
// normalized libp2p address.
func ResolveTransportEndpoints(p2pAddr string, peerID peer.ID) (TransportEndpoints, error) {
	address, err := Multiaddr(p2pAddr, peerID)
	if err != nil {
		return TransportEndpoints{}, err
	}
	host := ""
	for _, code := range []int{ma.P_IP4, ma.P_IP6, ma.P_DNS4, ma.P_DNS6} {
		if value, valueErr := address.ValueForProtocol(code); valueErr == nil {
			host = value
			break
		}
	}
	if host == "" {
		return TransportEndpoints{}, fmt.Errorf("peer multiaddress has no host")
	}
	var endpoints TransportEndpoints
	if port, portErr := address.ValueForProtocol(ma.P_TCP); portErr == nil {
		endpoints.TCP = net.JoinHostPort(host, port)
	}
	if port, portErr := address.ValueForProtocol(ma.P_UDP); portErr == nil {
		endpoints.UDP = net.JoinHostPort(host, port)
	}
	if _, quicErr := address.ValueForProtocol(ma.P_QUIC_V1); quicErr == nil {
		endpoints.QUIC = true
	}
	return endpoints, nil
}
