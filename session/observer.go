package session

import "context"

// Write describes bytes written by one of the SDK's protocol sessions.
// Protocol identifies the wire protocol, while Code, ProtocolID, and Topic are
// populated only when that protocol uses the corresponding discriminator.
type Write struct {
	Protocol   string
	Code       uint64
	ProtocolID string
	Topic      string
	Payload    []byte
	Raw        bool
}

// Observer receives a copy of each payload written by an SDK session.
// It is useful for tracing, metrics, and recording traffic in embedding tools.
type Observer func(Write)

// ObserveWrite reports a protocol write to the observer attached to ctx.
func ObserveWrite(ctx context.Context, write Write) {
	observer := scopeFrom(ctx).Observer
	if observer == nil {
		return
	}
	write.Payload = append([]byte(nil), write.Payload...)
	observer(write)
}
