package session

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// EventsEnv names the environment variable that enables delivery events. When
// it holds a file path, the connection layers append one JSON object per line
// to that file describing what this program did to the peer and what the peer
// did back. When it is unset or empty, no event is built or written.
const EventsEnv = "RAIDAN_EVENTS"

// Event kinds.
const (
	// EventConnected is a completed connection or handshake with a peer.
	EventConnected = "connected"
	// EventSent is a message written to a peer.
	EventSent = "sent"
	// EventReply is a message or response received from a peer.
	EventReply = "reply"
	// EventDisconnected is the end of a connection, with the peer's reason when it gave one.
	EventDisconnected = "disconnected"
	// EventError is a failed dial, write, read or request.
	EventError = "error"
)

// Event is one delivery event, one JSON line of the events file.
type Event struct {
	// Time is set by Emit (UTC, RFC 3339 with nanoseconds).
	Time string `json:"time"`
	// Event is one of the Event* kinds.
	Event string `json:"event"`
	// Protocol is the layer: rlpx, eth, snap, devp2p, libp2p, reqresp, gossipsub, discv4, discv5.
	Protocol string `json:"protocol,omitempty"`
	// Peer identifies the peer (enode ID, libp2p peer ID, or node ID).
	Peer string `json:"peer,omitempty"`
	// Addr is the peer's network address.
	Addr string `json:"addr,omitempty"`
	// Kind is the message kind: a protocol-relative message code (eth, snap),
	// a request protocol ID, a gossip topic, or a discovery packet type.
	Kind string `json:"kind,omitempty"`
	// Size is the payload size in bytes.
	Size int `json:"size,omitempty"`
	// Status is the response status of a reply (a req/resp response code name).
	Status string `json:"status,omitempty"`
	// ReasonCode is the disconnect reason code the peer sent.
	ReasonCode *uint64 `json:"reason_code,omitempty"`
	// Reason is the disconnect reason text.
	Reason string `json:"reason,omitempty"`
	// Op is the failed operation of an error event.
	Op string `json:"op,omitempty"`
	// Error is the error text of an error event.
	Error string `json:"error,omitempty"`
}

var events struct {
	once sync.Once
	mu   sync.Mutex
	file *os.File
}

func openEvents() {
	path := os.Getenv(EventsEnv)
	if path == "" {
		return
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", EventsEnv, err)
		return
	}
	events.file = file
}

// EventsEnabled reports whether delivery events are being written. Callers use
// it to skip building an event.
func EventsEnabled() bool {
	events.once.Do(openEvents)
	return events.file != nil
}

// Emit appends one event to the events file. It is safe for concurrent use and
// does nothing when events are disabled. Every event is one write of one line,
// so a killed process loses nothing already emitted.
func Emit(event Event) {
	if !EventsEnabled() {
		return
	}
	event.Time = time.Now().UTC().Format(time.RFC3339Nano)
	line, err := json.Marshal(event)
	if err != nil {
		return
	}
	line = append(line, '\n')
	events.mu.Lock()
	_, _ = events.file.Write(line)
	events.mu.Unlock()
}

// EmitError emits an error event for a failed operation.
func EmitError(protocol, peer, op string, err error) {
	if err == nil || !EventsEnabled() {
		return
	}
	Emit(Event{Event: EventError, Protocol: protocol, Peer: peer, Op: op, Error: err.Error()})
}

// Reason returns a pointer to a disconnect reason code.
func Reason(code uint64) *uint64 { return &code }
