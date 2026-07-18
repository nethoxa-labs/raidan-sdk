package eth

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/rlp"

	sdkrlpx "github.com/nethoxa-labs/raidan-sdk/rlpx"
	"github.com/nethoxa-labs/raidan-sdk/session"
)

const solicitedResponseWait = 5 * time.Second

// DecodeRequestID decodes the leading request ID from an ETH packet.
func DecodeRequestID(data []byte) (uint64, error) {
	content, _, err := rlp.SplitList(data)
	if err != nil {
		return 0, err
	}
	id, _, err := rlp.SplitUint64(content)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// WaitForRequestID waits for a request on a ready ETH connection.
func (c *Conn) WaitForRequestID(requestCode uint64, label string, timeout time.Duration) (uint64, []byte, error) {
	if timeout <= 0 {
		timeout = solicitedResponseWait
	}
	data, err := c.WaitForMsg(timeout, c.ETHOffset()+requestCode)
	if err != nil {
		return 0, nil, fmt.Errorf("target did not send %s within %s: %w", label, timeout, err)
	}
	id, err := DecodeRequestID(data)
	if err != nil {
		return 0, nil, fmt.Errorf("decode %s request id: %w", label, err)
	}
	return id, data, nil
}

// WaitForRequestID waits for an ETH request on the low-level connection.
func (c *PreStatusConn) WaitForRequestID(requestCode uint64, label string, timeout time.Duration) (uint64, []byte, error) {
	return waitForWireRequestID(c.ctx, c.rlpxConn, c.fd, c.ethOffset+requestCode, label, timeout)
}

func waitForWireRequestID(ctx context.Context, conn WireReadWriter, fd ReadDeadlineSetter, wireCode uint64, label string, timeout time.Duration) (uint64, []byte, error) {
	if timeout <= 0 {
		timeout = solicitedResponseWait
	}
	if err := fd.SetReadDeadline(time.Now().Add(session.Timeout(ctx, timeout))); err != nil {
		return 0, nil, fmt.Errorf("set %s deadline: %w", label, err)
	}
	defer func() { _ = fd.SetReadDeadline(time.Time{}) }()
	for {
		code, data, _, err := conn.Read()
		if err != nil {
			return 0, nil, fmt.Errorf("target did not send %s within %s: %w", label, timeout, err)
		}
		switch code {
		case sdkrlpx.DiscMsg:
			_, reason := sdkrlpx.DecodeDisconnectReason(data)
			return 0, nil, fmt.Errorf("peer disconnected while waiting for %s: %s", label, reason)
		case sdkrlpx.PingMsg:
			if _, err := conn.Write(sdkrlpx.PongMsg, []byte{0xC0}); err != nil {
				return 0, nil, fmt.Errorf("write pong while waiting for %s: %w", label, err)
			}
		case sdkrlpx.PongMsg:
			// Keepalive acknowledgements are not ETH requests.
		case wireCode:
			id, err := DecodeRequestID(data)
			if err != nil {
				return 0, nil, fmt.Errorf("decode %s request id: %w", label, err)
			}
			return id, data, nil
		}
	}
}

// RequestForResponse returns the request code paired with an ETH response.
func RequestForResponse(responseCode uint64) (uint64, string, error) {
	switch responseCode {
	case EthBlockHeaders:
		return EthGetBlockHeaders, "GetBlockHeaders", nil
	case EthBlockBodies:
		return EthGetBlockBodies, "GetBlockBodies", nil
	case EthPooledTransactions:
		return EthGetPooledTransactions, "GetPooledTransactions", nil
	case EthReceipts:
		return EthGetReceipts, "GetReceipts", nil
	case EthBlockAccessLists:
		return EthGetBlockAccessLists, "GetBlockAccessLists", nil
	case EthCells:
		return EthGetCells, "GetCells", nil
	default:
		return 0, "", fmt.Errorf("eth code 0x%x is not a known response code", responseCode)
	}
}
