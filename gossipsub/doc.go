// Package gossipsub provides Ethereum consensus-layer gossip publishing.
//
// ForkDigestHex reads the active fork digest from a beacon API, and Topic builds
// a canonical topic path. Publish establishes a libp2p gossipsub session, waits
// for a subscribed peer when requested, and publishes one caller-encoded
// message. Callers use github.com/golang/snappy directly for consensus gossip
// payload encoding.
//
//	digest, err := gossipsub.ForkDigestHex(ctx, target.ConsensusURL)
//	if err != nil {
//		return err
//	}
//	topic := gossipsub.Topic(digest, "beacon_block")
//	err = gossipsub.Publish(
//		ctx,
//		target.ConsensusURL,
//		target.ConsensusP2P,
//		topic,
//		snappy.Encode(nil, ssz),
//		gossipsub.Options{WaitForPeer: true},
//	)
//
// ErrNoPeers lets callers distinguish an unavailable topic subscription from
// transport failures. Options controls peer waiting and publish timeouts.
package gossipsub
