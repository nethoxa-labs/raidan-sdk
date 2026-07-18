package discovery

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

func readUDPWithContext(ctx context.Context, fd *net.UDPConn, buffer []byte, timeout time.Duration) (int, *net.UDPAddr, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, nil, err
	}
	remaining := session.Timeout(ctx, timeout)
	if remaining <= 0 {
		if err := ctx.Err(); err != nil {
			return 0, nil, err
		}
		return 0, nil, context.DeadlineExceeded
	}
	if err := fd.SetReadDeadline(time.Now().Add(remaining)); err != nil {
		return 0, nil, fmt.Errorf("set read deadline: %w", err)
	}
	stop := context.AfterFunc(ctx, func() {
		_ = fd.SetReadDeadline(time.Now())
	})
	defer stop()
	if err := ctx.Err(); err != nil {
		_ = fd.SetReadDeadline(time.Now())
	}
	n, from, err := fd.ReadFromUDP(buffer)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return 0, nil, ctxErr
	}
	return n, from, err
}
