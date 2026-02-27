package marmot

import (
	"fmt"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/interfaces/signer"
)

// MessageToEvent creates a kind 445 event carrying an MLS-encrypted
// application message for a group.
func MessageToEvent(groupID, ciphertext []byte, sign signer.I) (*event.E, error) {
	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = KindGroupMessage
	ev.Content = ciphertext
	ev.Tags = tag.NewS(
		tag.NewFromAny("g", hex.Enc(groupID)),
	)
	if err := ev.Sign(sign); err != nil {
		return nil, fmt.Errorf("sign group message: %w", err)
	}
	return ev, nil
}

// EventToMessage extracts the group ID and MLS ciphertext from a kind 445 event.
func EventToMessage(ev *event.E) (groupID, ciphertext []byte, err error) {
	if ev.Kind != KindGroupMessage {
		return nil, nil, fmt.Errorf("expected kind %d, got %d", KindGroupMessage, ev.Kind)
	}

	gTag := ev.Tags.GetFirst([]byte("g"))
	if gTag == nil {
		return nil, nil, fmt.Errorf("missing 'g' tag (group ID)")
	}
	groupID, err = hex.Dec(string(gTag.Value()))
	if err != nil {
		return nil, nil, fmt.Errorf("decode group ID: %w", err)
	}
	return groupID, ev.Content, nil
}
