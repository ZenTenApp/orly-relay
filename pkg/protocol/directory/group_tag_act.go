package directory

import (
	"strconv"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

// GroupTagAct represents a complete Group Tag Act event
// (Kind 39102) with typed access to its components.
type GroupTagAct struct {
	Event       *event.E
	GroupID     string
	TagName     string
	TagValue    string
	Actor       string
	Confidence  int
	Owners      *OwnershipSpec
	Created     *time.Time
	Description string
	IdentityTag *IdentityTag
}

// OwnershipSpec defines the ownership control for a group tag.
type OwnershipSpec struct {
	Scheme SignatureScheme
	Owners []string // Public keys of owners
}

// SignatureScheme defines the type of signature requirement.
type SignatureScheme string

const (
	SchemeSingle SignatureScheme = "single"
	Scheme2of3   SignatureScheme = "2-of-3"
	Scheme3of5   SignatureScheme = "3-of-5"
)

// ValidateSignatureScheme checks if a signature scheme is valid.
func ValidateSignatureScheme(scheme SignatureScheme) error {
	switch scheme {
	case SchemeSingle, Scheme2of3, Scheme3of5:
		return nil
	default:
		return errorf.E("invalid signature scheme: %s", scheme)
	}
}

// RequiredSignatures returns the number of signatures required for the scheme.
func (s SignatureScheme) RequiredSignatures() int {
	switch s {
	case SchemeSingle:
		return 1
	case Scheme2of3:
		return 2
	case Scheme3of5:
		return 3
	default:
		return 0
	}
}

// NewGroupTagAct creates a new Group Tag Act event.
func NewGroupTagAct(
	pubkey []byte,
	groupID, tagName, tagValue, actor string,
	confidence int,
	owners *OwnershipSpec,
	description string,
	identityTag *IdentityTag,
) (gta *GroupTagAct, err error) {

	// Validate required fields
	if len(pubkey) != 32 {
		return nil, errorf.E("pubkey must be 32 bytes")
	}
	if groupID == "" {
		return nil, errorf.E("group ID is required")
	}

	// Validate group ID is URL-safe
	if err = ValidateGroupTagName(groupID); chk.E(err) {
		return nil, errorf.E("invalid group ID: %w", err)
	}

	if tagName == "" {
		return nil, errorf.E("tag name is required")
	}
	if tagValue == "" {
		return nil, errorf.E("tag value is required")
	}
	if actor == "" {
		return nil, errorf.E("actor is required")
	}
	if len(actor) != 64 {
		return nil, errorf.E("actor must be 64 hex characters")
	}
	if confidence < 0 || confidence > 100 {
		return nil, errorf.E("confidence must be between 0 and 100")
	}

	// Validate ownership spec if provided
	if owners != nil {
		if err = ValidateSignatureScheme(owners.Scheme); chk.E(err) {
			return
		}
		if len(owners.Owners) == 0 {
			return nil, errorf.E("at least one owner is required")
		}
		// Validate owner count matches scheme
		switch owners.Scheme {
		case SchemeSingle:
			if len(owners.Owners) != 1 {
				return nil, errorf.E("single scheme requires exactly 1 owner")
			}
		case Scheme2of3:
			if len(owners.Owners) != 3 {
				return nil, errorf.E("2-of-3 scheme requires exactly 3 owners")
			}
		case Scheme3of5:
			if len(owners.Owners) != 5 {
				return nil, errorf.E("3-of-5 scheme requires exactly 5 owners")
			}
		}
	}

	// Create base event
	ev := CreateBaseEvent(pubkey, GroupTagActKind)
	ev.Content = []byte(description)

	// Add required tags
	ev.Tags.Append(tag.NewFromAny(string(DTag), groupID))
	ev.Tags.Append(tag.NewFromAny(string(GroupTagTag), tagName, tagValue))
	ev.Tags.Append(tag.NewFromAny(string(ActorTag), actor))
	ev.Tags.Append(tag.NewFromAny(string(ConfidenceTag), strconv.Itoa(confidence)))

	// Add ownership tag if provided
	if owners != nil {
		ownersTagParts := make([]any, 0, len(owners.Owners)+2)
		ownersTagParts = append(ownersTagParts, string(OwnersTag), string(owners.Scheme))
		for _, owner := range owners.Owners {
			ownersTagParts = append(ownersTagParts, owner)
		}
		ev.Tags.Append(tag.NewFromAny(ownersTagParts...))
	}

	// Add created timestamp
	created := time.Now()
	ev.Tags.Append(tag.NewFromAny(string(CreatedTag), strconv.FormatInt(created.Unix(), 10)))

	// Add identity tag if provided
	if identityTag != nil {
		iTag := tag.NewFromAny(string(ITag), identityTag.NPubIdentity, identityTag.Nonce, identityTag.Signature)
		ev.Tags.Append(iTag)
	}

	gta = &GroupTagAct{
		Event:       ev,
		GroupID:     groupID,
		TagName:     tagName,
		TagValue:    tagValue,
		Actor:       actor,
		Confidence:  confidence,
		Owners:      owners,
		Created:     &created,
		Description: description,
		IdentityTag: identityTag,
	}

	return
}

// ParseGroupTagAct parses an event into a GroupTagAct
// structure with validation.
func ParseGroupTagAct(ev *event.E) (gta *GroupTagAct, err error) {
	if ev == nil {
		return nil, errorf.E("event cannot be nil")
	}

	// Validate event kind
	if ev.Kind != GroupTagActKind.K {
		return nil, errorf.E("invalid event kind: expected %d, got %d",
			GroupTagActKind.K, ev.Kind)
	}

	// Extract required tags
	dTag := ev.Tags.GetFirst(DTag)
	if dTag == nil {
		return nil, errorf.E("missing d tag")
	}

	groupTagTag := ev.Tags.GetFirst(GroupTagTag)
	if groupTagTag == nil {
		return nil, errorf.E("missing group_tag tag")
	}

	// Validate group_tag has at least 2 elements (name and value)
	if groupTagTag.Len() < 3 { // "group_tag", name, value
		return nil, errorf.E("group_tag must have name and value")
	}

	actorTag := ev.Tags.GetFirst(ActorTag)
	if actorTag == nil {
		return nil, errorf.E("missing actor tag")
	}

	confidenceTag := ev.Tags.GetFirst(ConfidenceTag)
	if confidenceTag == nil {
		return nil, errorf.E("missing confidence tag")
	}

	// Parse confidence
	var confidence int
	if confidence, err = strconv.Atoi(string(confidenceTag.Value())); chk.E(err) {
		return nil, errorf.E("invalid confidence value: %w", err)
	}

	if confidence < 0 || confidence > 100 {
		return nil, errorf.E("confidence must be between 0 and 100")
	}

	// Parse optional ownership tag
	var owners *OwnershipSpec
	ownersTag := ev.Tags.GetFirst(OwnersTag)
	if ownersTag != nil && ownersTag.Len() >= 3 {
		scheme := SignatureScheme(ownersTag.T[1])
		if err = ValidateSignatureScheme(scheme); chk.E(err) {
			return nil, errorf.E("invalid signature scheme: %w", err)
		}
		ownerPubkeys := make([]string, 0, ownersTag.Len()-2)
		for i := 2; i < ownersTag.Len(); i++ {
			ownerPubkeys = append(ownerPubkeys, string(ownersTag.T[i]))
		}
		owners = &OwnershipSpec{
			Scheme: scheme,
			Owners: ownerPubkeys,
		}
	}

	// Parse optional created timestamp
	var created *time.Time
	createdTag := ev.Tags.GetFirst(CreatedTag)
	if createdTag != nil {
		var timestamp int64
		if timestamp, err = strconv.ParseInt(string(createdTag.Value()), 10, 64); err == nil {
			t := time.Unix(timestamp, 0)
			created = &t
		}
	}

	// Parse optional identity tag
	var identityTag *IdentityTag
	iTag := ev.Tags.GetFirst(ITag)
	if iTag != nil {
		if identityTag, err = ParseIdentityTag(iTag); chk.E(err) {
			return
		}
	}

	gta = &GroupTagAct{
		Event:       ev,
		GroupID:     string(dTag.Value()),
		TagName:     string(groupTagTag.T[1]),
		TagValue:    string(groupTagTag.T[2]),
		Actor:       string(actorTag.Value()),
		Confidence:  confidence,
		Owners:      owners,
		Created:     created,
		Description: string(ev.Content),
		IdentityTag: identityTag,
	}

	return
}

// Validate performs comprehensive validation of a GroupTagAct.
func (gta *GroupTagAct) Validate() (err error) {
	if gta == nil {
		return errorf.E("GroupTagAct cannot be nil")
	}

	if gta.Event == nil {
		return errorf.E("event cannot be nil")
	}

	// Validate event signature
	if _, err = gta.Event.Verify(); chk.E(err) {
		return errorf.E("invalid event signature: %w", err)
	}

	// Validate required fields
	if gta.GroupID == "" {
		return errorf.E("group ID is required")
	}

	if gta.TagName == "" {
		return errorf.E("tag name is required")
	}

	if gta.TagValue == "" {
		return errorf.E("tag value is required")
	}

	if gta.Actor == "" {
		return errorf.E("actor is required")
	}

	if len(gta.Actor) != 64 {
		return errorf.E("actor must be 64 hex characters")
	}

	if gta.Confidence < 0 || gta.Confidence > 100 {
		return errorf.E("confidence must be between 0 and 100")
	}

	return nil
}

// GetGroupID returns the group identifier.
func (gta *GroupTagAct) GetGroupID() string {
	return gta.GroupID
}

// GetTagName returns the tag name being attested.
func (gta *GroupTagAct) GetTagName() string {
	return gta.TagName
}

// GetTagValue returns the tag value being attested.
func (gta *GroupTagAct) GetTagValue() string {
	return gta.TagValue
}

// GetActor returns the public key of the relay making the act.
func (gta *GroupTagAct) GetActor() string {
	return gta.Actor
}

// GetConfidence returns the confidence level (0-100) in this act.
func (gta *GroupTagAct) GetConfidence() int {
	return gta.Confidence
}

// GetDescription returns the optional description of the act.
func (gta *GroupTagAct) GetDescription() string {
	return gta.Description
}

// IsHighConfidence returns true if the confidence level is 80 or higher.
func (gta *GroupTagAct) IsHighConfidence() bool {
	return gta.Confidence >= 80
}

// IsMediumConfidence returns true if the confidence level is between 50 and 79.
func (gta *GroupTagAct) IsMediumConfidence() bool {
	return gta.Confidence >= 50 && gta.Confidence < 80
}

// IsLowConfidence returns true if the confidence level is below 50.
func (gta *GroupTagAct) IsLowConfidence() bool {
	return gta.Confidence < 50
}

// MatchesTag returns true if this act matches the given tag name and value.
func (gta *GroupTagAct) MatchesTag(name, value string) bool {
	return gta.TagName == name && gta.TagValue == value
}

// MatchesGroup returns true if this act belongs to the given group.
func (gta *GroupTagAct) MatchesGroup(groupID string) bool {
	return gta.GroupID == groupID
}

// IsAttestedBy returns true if this act was made by the given actor.
func (gta *GroupTagAct) IsAttestedBy(actor string) bool {
	return gta.Actor == actor
}
