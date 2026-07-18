package eth

import (
	"context"
	"fmt"
	"time"

	sdkrlpx "github.com/nethoxa-labs/raidan-sdk/rlpx"
	"github.com/nethoxa-labs/raidan-sdk/session"
)

// WaitStatus waits for a raw ETH Status message while servicing base-protocol
// pings. It supports caller-controlled status exchanges that cannot use
// PreStatusConn.ExchangeStatus.
func WaitStatus(ctx context.Context, conn WireReadWriter, fd ReadDeadlineSetter, timeout time.Duration) error {
	if err := fd.SetReadDeadline(time.Now().Add(session.Timeout(ctx, timeout))); err != nil {
		return fmt.Errorf("set status deadline: %w", err)
	}
	defer func() { _ = fd.SetReadDeadline(time.Time{}) }()
	for {
		code, data, _, err := conn.Read()
		if err != nil {
			return fmt.Errorf("read status: %w", err)
		}
		switch code {
		case sdkrlpx.DiscMsg:
			_, reason := sdkrlpx.DecodeDisconnectReason(data)
			return fmt.Errorf("peer disconnected before status: %s", reason)
		case 16: // First negotiated ETH message code: Status.
			return nil
		case sdkrlpx.PingMsg:
			if _, err := conn.Write(sdkrlpx.PongMsg, []byte{0xC0}); err != nil {
				return fmt.Errorf("write pong: %w", err)
			}
		case sdkrlpx.PongMsg:
			// Keepalive acknowledgements are not ETH status messages.
		}
	}
}

// WaitClose reports whether a peer closes or sends Disconnect before timeout.
func WaitClose(ctx context.Context, conn WireReadWriter, fd ReadDeadlineSetter, timeout time.Duration) bool {
	closed, _ := WaitCloseReason(ctx, conn, fd, timeout)
	return closed
}

// WaitCloseReason is WaitClose plus the decoded Disconnect reason, when sent.
func WaitCloseReason(ctx context.Context, conn WireReadWriter, fd ReadDeadlineSetter, timeout time.Duration) (bool, string) {
	if err := fd.SetReadDeadline(time.Now().Add(session.Timeout(ctx, timeout))); err != nil {
		return false, ""
	}
	defer func() { _ = fd.SetReadDeadline(time.Time{}) }()
	for {
		code, data, _, err := conn.Read()
		if err != nil {
			if isTimeoutError(err) {
				return false, ""
			}
			return true, ""
		}
		switch code {
		case sdkrlpx.DiscMsg:
			_, reason := sdkrlpx.DecodeDisconnectReason(data)
			return true, reason
		case sdkrlpx.PingMsg:
			if _, err := conn.Write(sdkrlpx.PongMsg, []byte{0xC0}); err != nil {
				return true, ""
			}
		case sdkrlpx.PongMsg:
			// Ignore keepalive acknowledgements while waiting for closure.
		}
	}
}
