package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/forkid"
	ethproto "github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/rlp"

	sdkrlpx "github.com/nethoxa-labs/raidan-sdk/rlpx"
	ethrpc "github.com/nethoxa-labs/raidan-sdk/rpc"
	"github.com/nethoxa-labs/raidan-sdk/session"
)

// Status68 is the ETH/68 Status wire layout. Field order is part of the
// protocol and must remain unchanged.
type Status68 struct {
	ProtocolVersion uint32
	NetworkID       uint64
	TD              *big.Int
	Head            common.Hash
	Genesis         common.Hash
	ForkID          forkid.ID
}

// ExchangeStatus sends the canonical Status for this connection and waits for
// the peer's Status. It completes a DialPreStatus connection without rebuilding
// chain or capability state in the caller.
func (c *PreStatusConn) ExchangeStatus(timeout time.Duration) error {
	payload, err := encodeStatus(c.ethVersion, &c.params)
	if err != nil {
		return fmt.Errorf("encode status: %w", err)
	}
	if err := c.SendETHRaw(EthStatus, payload); err != nil {
		return fmt.Errorf("write status: %w", err)
	}
	if err := c.fd.SetReadDeadline(time.Now().Add(session.Timeout(c.ctx, timeout))); err != nil {
		return fmt.Errorf("set status deadline: %w", err)
	}
	defer func() { _ = c.fd.SetReadDeadline(time.Time{}) }()
	for {
		code, data, _, err := c.rlpxConn.Read()
		if err != nil {
			return fmt.Errorf("read status: %w", err)
		}
		switch code {
		case c.ethOffset + EthStatus:
			if _, err := decodeAndValidateStatus(c.ethVersion, data, &c.params); err != nil {
				return fmt.Errorf("validate status: %w", err)
			}
			return nil
		case sdkrlpx.DiscMsg:
			_, reason := sdkrlpx.DecodeDisconnectReason(data)
			return errors.New("peer disconnected during status exchange: " + reason)
		case sdkrlpx.PingMsg:
			if _, err := c.rlpxConn.Write(sdkrlpx.PongMsg, []byte{0xC0}); err != nil {
				return fmt.Errorf("write pong: %w", err)
			}
		case sdkrlpx.PongMsg:
			// Keepalive acknowledgements are not part of the ETH status exchange.
		}
	}
}

func encodeStatus(ethVersion uint, params *ethrpc.ChainParams) ([]byte, error) {
	if ethVersion >= 69 {
		return rlp.EncodeToBytes(&ethproto.StatusPacket{
			ProtocolVersion: uint32(ethVersion),
			NetworkID:       params.NetworkID,
			Genesis:         params.Genesis,
			ForkID:          params.ForkID,
			EarliestBlock:   0,
			LatestBlock:     params.HeadNumber,
			LatestBlockHash: params.HeadHash,
		})
	}
	return rlp.EncodeToBytes(&Status68{
		ProtocolVersion: uint32(ethVersion),
		NetworkID:       params.NetworkID,
		TD:              big.NewInt(0),
		Head:            params.HeadHash,
		Genesis:         params.Genesis,
		ForkID:          params.ForkID,
	})
}

func decodeAndValidateStatus(ethVersion uint, data []byte, params *ethrpc.ChainParams) (forkid.ID, error) {
	if params == nil {
		return forkid.ID{}, errors.New("chain parameters are nil")
	}
	if ethVersion == 68 {
		var status Status68
		if err := rlp.DecodeBytes(data, &status); err != nil {
			return forkid.ID{}, fmt.Errorf("decode eth/68 status: %w", err)
		}
		if err := validateStatusIdentity(ethVersion, status.ProtocolVersion, status.NetworkID, status.Genesis, status.ForkID, params); err != nil {
			return retryForkID(status.ForkID, err)
		}
		return status.ForkID, nil
	}
	if ethVersion < 69 || ethVersion > 72 {
		return forkid.ID{}, fmt.Errorf("unsupported ETH protocol version %d", ethVersion)
	}
	var status ethproto.StatusPacket
	if err := rlp.DecodeBytes(data, &status); err != nil {
		return forkid.ID{}, fmt.Errorf("decode eth/%d status: %w", ethVersion, err)
	}
	if err := validateStatusIdentity(ethVersion, status.ProtocolVersion, status.NetworkID, status.Genesis, status.ForkID, params); err != nil {
		return retryForkID(status.ForkID, err)
	}
	if status.EarliestBlock > status.LatestBlock {
		return forkid.ID{}, fmt.Errorf("invalid block range: earliest %d exceeds latest %d", status.EarliestBlock, status.LatestBlock)
	}
	if status.LatestBlockHash == (common.Hash{}) {
		return forkid.ID{}, errors.New("invalid block range: zero latest block hash")
	}
	return status.ForkID, nil
}

func validateStatusIdentity(ethVersion uint, protocolVersion uint32, networkID uint64, genesis common.Hash, remoteFork forkid.ID, params *ethrpc.ChainParams) error {
	if uint(protocolVersion) != ethVersion {
		return fmt.Errorf("protocol version %d does not match negotiated eth/%d", protocolVersion, ethVersion)
	}
	if networkID != params.NetworkID {
		return fmt.Errorf("network ID %d does not match %d", networkID, params.NetworkID)
	}
	if genesis != params.Genesis {
		return fmt.Errorf("genesis %s does not match %s", genesis, params.Genesis)
	}
	if err := params.ValidateForkID(remoteFork); err != nil {
		return &forkIDRejectedError{cause: err}
	}
	return nil
}

type forkIDRejectedError struct{ cause error }

func (e *forkIDRejectedError) Error() string { return "fork ID rejected: " + e.cause.Error() }
func (e *forkIDRejectedError) Unwrap() error { return e.cause }

func retryForkID(remote forkid.ID, err error) (forkid.ID, error) {
	var rejected *forkIDRejectedError
	if errors.As(err, &rejected) {
		return remote, err
	}
	return forkid.ID{}, err
}

func waitStatusOutcome(ctx context.Context, conn WireReadWriter, fd ReadDeadlineSetter, timeout time.Duration) (closed bool, reason string) {
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

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
