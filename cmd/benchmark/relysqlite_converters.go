package main

import (
	"encoding/hex"
	"fmt"

	"github.com/nbd-wtf/go-nostr"

	orlyEvent "next.orly.dev/pkg/nostr/encoders/event"
	orlyFilter "next.orly.dev/pkg/nostr/encoders/filter"
	orlyTag "next.orly.dev/pkg/nostr/encoders/tag"
)

// convertToNostrEvent converts an ORLY event to a go-nostr event
func convertToNostrEvent(ev *orlyEvent.E) (*nostr.Event, error) {
	if ev == nil {
		return nil, fmt.Errorf("nil event")
	}

	nostrEv := &nostr.Event{
		ID:        hex.EncodeToString(ev.ID),
		PubKey:    hex.EncodeToString(ev.Pubkey),
		CreatedAt: nostr.Timestamp(ev.CreatedAt),
		Kind:      int(ev.Kind),
		Content:   string(ev.Content),
		Sig:       hex.EncodeToString(ev.Sig),
	}

	// Convert tags
	if ev.Tags != nil && len(*ev.Tags) > 0 {
		nostrEv.Tags = make(nostr.Tags, 0, len(*ev.Tags))
		for _, orlyTag := range *ev.Tags {
			if orlyTag != nil && len(orlyTag.T) > 0 {
				tag := make(nostr.Tag, len(orlyTag.T))
				for i, val := range orlyTag.T {
					tag[i] = string(val)
				}
				nostrEv.Tags = append(nostrEv.Tags, tag)
			}
		}
	}

	return nostrEv, nil
}

// convertFromNostrEvent converts a go-nostr event to an ORLY event
func convertFromNostrEvent(ne *nostr.Event) (*orlyEvent.E, error) {
	if ne == nil {
		return nil, fmt.Errorf("nil event")
	}

	ev := orlyEvent.New()

	// Convert ID
	idBytes, err := hex.DecodeString(ne.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ID: %w", err)
	}
	ev.ID = idBytes

	// Convert Pubkey
	pubkeyBytes, err := hex.DecodeString(ne.PubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode pubkey: %w", err)
	}
	ev.Pubkey = pubkeyBytes

	// Convert Sig
	sigBytes, err := hex.DecodeString(ne.Sig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}
	ev.Sig = sigBytes

	// Simple fields
	ev.CreatedAt = int64(ne.CreatedAt)
	ev.Kind = uint16(ne.Kind)
	ev.Content = []byte(ne.Content)

	// Convert tags
	if len(ne.Tags) > 0 {
		ev.Tags = orlyTag.NewS()
		for _, nostrTag := range ne.Tags {
			if len(nostrTag) > 0 {
				tag := orlyTag.NewWithCap(len(nostrTag))
				for _, val := range nostrTag {
					tag.T = append(tag.T, []byte(val))
				}
				*ev.Tags = append(*ev.Tags, tag)
			}
		}
	} else {
		ev.Tags = orlyTag.NewS()
	}

	return ev, nil
}

// convertToNostrFilter converts an ORLY filter to a go-nostr filter
func convertToNostrFilter(f *orlyFilter.F) (nostr.Filter, error) {
	if f == nil {
		return nostr.Filter{}, fmt.Errorf("nil filter")
	}

	filter := nostr.Filter{}

	// Convert IDs
	if f.Ids != nil && len(f.Ids.T) > 0 {
		filter.IDs = make([]string, 0, len(f.Ids.T))
		for _, id := range f.Ids.T {
			filter.IDs = append(filter.IDs, hex.EncodeToString(id))
		}
	}

	// Convert Authors
	if f.Authors != nil && len(f.Authors.T) > 0 {
		filter.Authors = make([]string, 0, len(f.Authors.T))
		for _, author := range f.Authors.T {
			filter.Authors = append(filter.Authors, hex.EncodeToString(author))
		}
	}

	// Convert Kinds
	if f.Kinds != nil && len(f.Kinds.K) > 0 {
		filter.Kinds = make([]int, 0, len(f.Kinds.K))
		for _, kind := range f.Kinds.K {
			filter.Kinds = append(filter.Kinds, int(kind.K))
		}
	}

	// Convert Tags
	if f.Tags != nil && len(*f.Tags) > 0 {
		filter.Tags = make(nostr.TagMap)
		for _, tag := range *f.Tags {
			if tag != nil && len(tag.T) >= 2 {
				tagName := string(tag.T[0])
				tagValues := make([]string, 0, len(tag.T)-1)
				for i := 1; i < len(tag.T); i++ {
					tagValues = append(tagValues, string(tag.T[i]))
				}
				filter.Tags[tagName] = tagValues
			}
		}
	}

	// Convert timestamps
	if f.Since != nil {
		ts := nostr.Timestamp(f.Since.V)
		filter.Since = &ts
	}

	if f.Until != nil {
		ts := nostr.Timestamp(f.Until.V)
		filter.Until = &ts
	}

	// Convert limit
	if f.Limit != nil {
		limit := int(*f.Limit)
		filter.Limit = limit
	}

	return filter, nil
}
