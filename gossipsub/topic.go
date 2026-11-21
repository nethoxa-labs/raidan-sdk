package gossipsub

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/nethoxa-labs/raidan-sdk/reqresp"
)

// Encoding is the consensus gossip encoding component.
const Encoding = "ssz_snappy"

// ForkDigestHex returns the current four-byte fork digest as lowercase hex.
func ForkDigestHex(ctx context.Context, beaconURL string) (string, error) {
	statusV1, _, err := reqresp.BeaconStatus(ctx, beaconURL)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(statusV1[:reqresp.ContextBytesLen]), nil
}

// Topic builds a canonical consensus gossip topic identifier.
func Topic(forkDigestHex, topic string) string {
	return fmt.Sprintf("/eth2/%s/%s/%s", forkDigestHex, strings.Trim(topic, "/"), Encoding)
}
