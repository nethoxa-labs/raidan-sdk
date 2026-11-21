package gossipsub

import (
	"fmt"

	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"google.golang.org/protobuf/proto"
)

// EncodeSubscriptions encodes one gossipsub RPC subscription list.
func EncodeSubscriptions(topics []string, subscribe bool) ([]byte, error) {
	subscriptions := make([]*pubsubpb.RPC_SubOpts, len(topics))
	for i, topic := range topics {
		if topic == "" {
			return nil, fmt.Errorf("subscription %d has an empty topic", i)
		}
		topicCopy := topic
		subscribeCopy := subscribe
		subscriptions[i] = &pubsubpb.RPC_SubOpts{Subscribe: &subscribeCopy, Topicid: &topicCopy}
	}
	return proto.Marshal(&pubsubpb.RPC{Subscriptions: subscriptions})
}

// EncodePublish encodes one unsigned gossipsub RPC publish entry.
func EncodePublish(topic string, data []byte) ([]byte, error) {
	if topic == "" {
		return nil, fmt.Errorf("publish topic is empty")
	}
	topicCopy := topic
	return proto.Marshal(&pubsubpb.RPC{Publish: []*pubsubpb.Message{{Topic: &topicCopy, Data: append([]byte(nil), data...)}}})
}
