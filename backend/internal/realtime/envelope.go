// backend/internal/realtime/envelope.go
package realtime

import (
	"encoding/json"
	"errors"
)

// MessageType is a closed set of envelope kinds — never a bare string
// literal at call sites, so a typo doesn't silently produce a message
// no consumer or control-loop switch statement recognizes.
type MessageType string

const (
	// MessageTypeUpdate is server -> client: a push of new data on a
	// topic. Consumers own their own Payload shape per topic.
	MessageTypeUpdate MessageType = "update"
	// MessageTypeSubscribe / MessageTypeUnsubscribe are client -> server
	// control messages naming the Topic to (un)subscribe from — the only
	// client-originated traffic this push-only shell accepts.
	MessageTypeSubscribe   MessageType = "subscribe"
	MessageTypeUnsubscribe MessageType = "unsubscribe"
)

// Envelope is the only shape the Hub understands, in both directions.
// Payload is left as raw JSON deliberately — the Hub routes on Topic/Type
// and never inspects Payload, so a consumer's payload shape can change
// without the shell changing.
type Envelope struct {
	Topic   string          `json:"topic"`
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

var (
	errMissingTopic = errors.New("envelope: topic is required")
	errMissingType  = errors.New("envelope: type is required")
	errUnknownType  = errors.New("envelope: unknown type")
)

// Validate rejects a malformed envelope before it reaches the Hub —
// called on every inbound control message (connection.go's readPump)
// and cheap enough to also call on outbound envelopes in tests/dev.
func (e Envelope) Validate() error {
	if e.Topic == "" {
		return errMissingTopic
	}
	switch e.Type {
	case MessageTypeUpdate, MessageTypeSubscribe, MessageTypeUnsubscribe:
		return nil
	case "":
		return errMissingType
	default:
		return errUnknownType
	}
}
