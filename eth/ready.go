package eth

import (
	"context"
	"fmt"
	"time"
)

// UnsupportedCapabilityError reports a peer that cannot run the requested
// ETH protocol version. The caller can classify this result as not applicable.
type UnsupportedCapabilityError struct {
	Negotiated uint
	Required   uint
}

func (e *UnsupportedCapabilityError) Error() string {
	return fmt.Sprintf("peer negotiated eth/%d (need eth/%d)", e.Negotiated, e.Required)
}

// DialReady negotiates an ETH capability and completes the canonical Status
// exchange. The returned connection is ready for protocol requests.
func DialReady(ctx context.Context, target, rpc string, want uint) (*PreStatusConn, error) {
	if want < 68 {
		return nil, fmt.Errorf("unsupported eth version %d (minimum 68)", want)
	}
	conn, err := DialPreStatus(ctx, target, rpc, Config{MaxVersion: want})
	if err != nil {
		return nil, err
	}
	if conn.ETHVersion() < want {
		got := conn.ETHVersion()
		conn.Close()
		return nil, &UnsupportedCapabilityError{Negotiated: got, Required: want}
	}
	if err := conn.ExchangeStatus(10 * time.Second); err != nil {
		conn.Close()
		return nil, fmt.Errorf("status exchange: %w", err)
	}
	return conn, nil
}
