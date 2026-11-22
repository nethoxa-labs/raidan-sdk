package discovery

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/nethoxa-labs/raidan-sdk/session"
)

// readUDPWithContext reads one datagram. With delivery events enabled it
// records the datagram as a reply of protocol.
func readUDPWithContext(ctx context.Context, protocol string, fd *net.UDPConn, buffer []byte, timeout time.Duration) (int, *net.UDPAddr, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return 0, nil, err
	}
	remaining := session.Remaining(ctx, timeout)
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
	if err == nil && session.EventsEnabled() {
		session.Emit(session.Event{Event: session.EventReply, Protocol: protocol, Addr: from.String(), Kind: "datagram", Size: n})
	}
	return n, from, err
}

// writeUDP sends one datagram to the peer. With delivery events enabled it
// records the datagram as sent, with its kind (the packet type or "raw").
func writeUDP(fd *net.UDPConn, protocol, kind string, data []byte, to *net.UDPAddr) error {
	_, err := fd.WriteToUDP(data, to)
	if session.EventsEnabled() {
		if err != nil {
			session.EmitError(protocol, "", "write "+to.String(), err)
		} else {
			session.Emit(session.Event{Event: session.EventSent, Protocol: protocol, Addr: to.String(), Kind: kind, Size: len(data)})
		}
	}
	return err
}

// emitUDPConnected records an opened discovery socket to the peer.
func emitUDPConnected(protocol, peer string, to *net.UDPAddr) {
	if session.EventsEnabled() {
		session.Emit(session.Event{Event: session.EventConnected, Protocol: protocol, Peer: peer, Addr: to.String()})
	}
}
