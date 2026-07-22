package rlpx

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"net"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

// RawHello is the base-protocol Hello payload.
type RawHello struct {
	Version    uint64
	Name       string
	Caps       []p2p.Cap
	ListenPort uint64
	ID         []byte
	Rest       []rlp.RawValue `rlp:"tail"`
}

// DialAndHello performs RLPx authentication with key and writes exactly the
// caller-supplied Hello. No identity, name, capability, or version defaults are
// applied.
func DialAndHello(ctx context.Context, target string, key *ecdsa.PrivateKey, hello RawHello) (*Conn, net.Conn, error) {
	rc, fd, err := dial(ctx, target, key)
	if err != nil {
		return nil, nil, err
	}
	session.Step(ctx, "[+] RLPx handshake complete")
	data, err := rlp.EncodeToBytes(&hello)
	if err != nil {
		_ = fd.Close()
		return nil, nil, fmt.Errorf("encode hello: %w", err)
	}
	if _, err := rc.Write(HelloMsg, data); err != nil {
		_ = fd.Close()
		return nil, nil, fmt.Errorf("write hello: %w", err)
	}
	session.Step(ctx, "[+] Sent Hello with %d capabilities", len(hello.Caps))
	if err := clearDeadline(fd); err != nil {
		_ = fd.Close()
		return nil, nil, err
	}
	return rc, fd, nil
}
