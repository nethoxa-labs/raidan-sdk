package eth

import (
	"context"
	"fmt"
	"time"
)

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
		return nil, fmt.Errorf("peer negotiated eth/%d (need eth/%d)", got, want)
	}
	if err := conn.ExchangeStatus(10 * time.Second); err != nil {
		conn.Close()
		return nil, fmt.Errorf("status exchange: %w", err)
	}
	return conn, nil
}
