// Package orlydbv1 provides type converters between proto messages and Go types.
package orlydbv1

import (
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"next.orly.dev/pkg/database"
	indextypes "next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/interfaces/store"
)

// EventToProto converts a nostr event.E to a proto Event.
// Binary fields (ID, Pubkey, Sig) are copied directly.
// Tags are converted preserving binary values for e/p tags.
func EventToProto(ev *event.E) *Event {
	if ev == nil {
		return nil
	}

	pb := &Event{
		Id:        ev.ID,
		Pubkey:    ev.Pubkey,
		CreatedAt: ev.CreatedAt,
		Kind:      uint32(ev.Kind),
		Content:   ev.Content,
		Sig:       ev.Sig,
	}

	if ev.Tags != nil {
		pb.Tags = make([]*Tag, 0, len(*ev.Tags))
		for _, t := range *ev.Tags {
			pbTag := &Tag{
				Values: make([][]byte, 0, len(t.T)),
			}
			for _, v := range t.T {
				// Copy bytes directly to preserve binary data
				val := make([]byte, len(v))
				copy(val, v)
				pbTag.Values = append(pbTag.Values, val)
			}
			pb.Tags = append(pb.Tags, pbTag)
		}
	}

	return pb
}

// ProtoToEvent converts a proto Event to a nostr event.E.
func ProtoToEvent(pb *Event) *event.E {
	if pb == nil {
		return nil
	}

	ev := event.New()
	ev.ID = pb.Id
	ev.Pubkey = pb.Pubkey
	ev.CreatedAt = pb.CreatedAt
	ev.Kind = uint16(pb.Kind)
	ev.Content = pb.Content
	ev.Sig = pb.Sig

	if len(pb.Tags) > 0 {
		tags := tag.NewSWithCap(len(pb.Tags))
		for _, pbTag := range pb.Tags {
			t := tag.NewWithCap(len(pbTag.Values))
			for _, v := range pbTag.Values {
				t.T = append(t.T, v)
			}
			*tags = append(*tags, t)
		}
		ev.Tags = tags
	}

	return ev
}

// FilterToProto converts a nostr filter.F to a proto Filter.
func FilterToProto(f *filter.F) *Filter {
	if f == nil {
		return nil
	}

	pb := &Filter{}

	// IDs
	if f.Ids != nil && len(f.Ids.T) > 0 {
		pb.Ids = make([][]byte, 0, len(f.Ids.T))
		for _, id := range f.Ids.T {
			pb.Ids = append(pb.Ids, id)
		}
	}

	// Kinds
	if f.Kinds != nil && f.Kinds.Len() > 0 {
		pb.Kinds = make([]uint32, 0, f.Kinds.Len())
		for _, k := range f.Kinds.K {
			pb.Kinds = append(pb.Kinds, uint32(k.K))
		}
	}

	// Authors
	if f.Authors != nil && len(f.Authors.T) > 0 {
		pb.Authors = make([][]byte, 0, len(f.Authors.T))
		for _, a := range f.Authors.T {
			pb.Authors = append(pb.Authors, a)
		}
	}

	// Tags (e.g., #e, #p, #t)
	if f.Tags != nil && len(*f.Tags) > 0 {
		pb.Tags = make(map[string]*TagSet)
		for _, t := range *f.Tags {
			if len(t.T) >= 2 {
				key := string(t.T[0])
				ts, exists := pb.Tags[key]
				if !exists {
					ts = &TagSet{}
					pb.Tags[key] = ts
				}
				// Add all values after the key
				for _, v := range t.T[1:] {
					ts.Values = append(ts.Values, v)
				}
			}
		}
	}

	// Since
	if f.Since != nil {
		since := f.Since.I64()
		pb.Since = &since
	}

	// Until
	if f.Until != nil {
		until := f.Until.I64()
		pb.Until = &until
	}

	// Search
	if len(f.Search) > 0 {
		pb.Search = f.Search
	}

	// Limit
	if f.Limit != nil {
		limit := uint32(*f.Limit)
		pb.Limit = &limit
	}

	return pb
}

// ProtoToFilter converts a proto Filter to a nostr filter.F.
func ProtoToFilter(pb *Filter) *filter.F {
	if pb == nil {
		return nil
	}

	f := filter.New()

	// IDs
	if len(pb.Ids) > 0 {
		f.Ids = tag.NewWithCap(len(pb.Ids))
		for _, id := range pb.Ids {
			f.Ids.T = append(f.Ids.T, id)
		}
	}

	// Kinds
	if len(pb.Kinds) > 0 {
		kinds := kind.NewWithCap(len(pb.Kinds))
		for _, k := range pb.Kinds {
			kinds.K = append(kinds.K, kind.New(uint16(k)))
		}
		f.Kinds = kinds
	}

	// Authors
	if len(pb.Authors) > 0 {
		f.Authors = tag.NewWithCap(len(pb.Authors))
		for _, a := range pb.Authors {
			f.Authors.T = append(f.Authors.T, a)
		}
	}

	// Tags
	if len(pb.Tags) > 0 {
		tags := tag.NewSWithCap(len(pb.Tags))
		for key, ts := range pb.Tags {
			for _, v := range ts.Values {
				t := tag.NewWithCap(2)
				t.T = append(t.T, []byte(key), v)
				*tags = append(*tags, t)
			}
		}
		f.Tags = tags
	}

	// Since
	if pb.Since != nil {
		f.Since = timestamp.FromUnix(*pb.Since)
	}

	// Until
	if pb.Until != nil {
		f.Until = timestamp.FromUnix(*pb.Until)
	}

	// Search
	if len(pb.Search) > 0 {
		f.Search = pb.Search
	}

	// Limit
	if pb.Limit != nil {
		limit := uint(*pb.Limit)
		f.Limit = &limit
	}

	return f
}

// Uint40ToProto converts a indextypes.Uint40 to a proto Uint40.
func Uint40ToProto(u *indextypes.Uint40) *Uint40 {
	if u == nil {
		return nil
	}
	return &Uint40{Value: u.Get()}
}

// ProtoToUint40 converts a proto Uint40 to a indextypes.Uint40.
func ProtoToUint40(pb *Uint40) *indextypes.Uint40 {
	if pb == nil {
		return nil
	}
	return newUint40(pb.Value)
}

// Uint40sToProto converts a slice of Uint40s to a SerialList.
func Uint40sToProto(serials indextypes.Uint40s) *SerialList {
	result := &SerialList{
		Serials: make([]uint64, 0, len(serials)),
	}
	for _, s := range serials {
		result.Serials = append(result.Serials, s.Get())
	}
	return result
}

// ProtoToUint40s converts a SerialList to a slice of Uint40 pointers.
func ProtoToUint40s(pb *SerialList) []*indextypes.Uint40 {
	if pb == nil {
		return nil
	}
	result := make([]*indextypes.Uint40, 0, len(pb.Serials))
	for _, s := range pb.Serials {
		result = append(result, newUint40(s))
	}
	return result
}

// IdPkTsToProto converts a store.IdPkTs to a proto IdPkTs.
func IdPkTsToProto(i *store.IdPkTs) *IdPkTs {
	if i == nil {
		return nil
	}
	return &IdPkTs{
		Id:        i.Id,
		Pubkey:    i.Pub,
		Timestamp: i.Ts,
		Serial:    i.Ser,
	}
}

// ProtoToIdPkTs converts a proto IdPkTs to a store.IdPkTs.
func ProtoToIdPkTs(pb *IdPkTs) *store.IdPkTs {
	if pb == nil {
		return nil
	}
	return &store.IdPkTs{
		Id:  pb.Id,
		Pub: pb.Pubkey,
		Ts:  pb.Timestamp,
		Ser: pb.Serial,
	}
}

// IdPkTsListToProto converts a slice of IdPkTs to a proto IdPkTsList.
func IdPkTsListToProto(items []*store.IdPkTs) *IdPkTsList {
	result := &IdPkTsList{
		Items: make([]*IdPkTs, 0, len(items)),
	}
	for _, item := range items {
		result.Items = append(result.Items, IdPkTsToProto(item))
	}
	return result
}

// ProtoToIdPkTsList converts a proto IdPkTsList to a slice of IdPkTs.
func ProtoToIdPkTsList(pb *IdPkTsList) []*store.IdPkTs {
	if pb == nil {
		return nil
	}
	result := make([]*store.IdPkTs, 0, len(pb.Items))
	for _, item := range pb.Items {
		result = append(result, ProtoToIdPkTs(item))
	}
	return result
}

// SubscriptionToProto converts a database.Subscription to a proto Subscription.
func SubscriptionToProto(s *database.Subscription, pubkey []byte) *Subscription {
	if s == nil {
		return nil
	}
	return &Subscription{
		Pubkey:           pubkey,
		TrialEnd:         s.TrialEnd.Unix(),
		PaidUntil:        s.PaidUntil.Unix(),
		BlossomLevel:     s.BlossomLevel,
		BlossomStorageMb: s.BlossomStorage,
	}
}

// ProtoToSubscription converts a proto Subscription to a database.Subscription.
func ProtoToSubscription(pb *Subscription) *database.Subscription {
	if pb == nil {
		return nil
	}
	return &database.Subscription{
		TrialEnd:       timeFromUnix(pb.TrialEnd),
		PaidUntil:      timeFromUnix(pb.PaidUntil),
		BlossomLevel:   pb.BlossomLevel,
		BlossomStorage: pb.BlossomStorageMb,
	}
}

// PaymentToProto converts a database.Payment to a proto Payment.
func PaymentToProto(p *database.Payment) *Payment {
	return &Payment{
		Amount:    p.Amount,
		Timestamp: p.Timestamp.Unix(),
		Invoice:   p.Invoice,
		Preimage:  p.Preimage,
	}
}

// ProtoToPayment converts a proto Payment to a database.Payment.
func ProtoToPayment(pb *Payment) *database.Payment {
	return &database.Payment{
		Amount:    pb.Amount,
		Timestamp: timeFromUnix(pb.Timestamp),
		Invoice:   pb.Invoice,
		Preimage:  pb.Preimage,
	}
}

// PaymentListToProto converts a slice of payments to a proto PaymentList.
func PaymentListToProto(payments []database.Payment) *PaymentList {
	result := &PaymentList{
		Payments: make([]*Payment, 0, len(payments)),
	}
	for _, p := range payments {
		result.Payments = append(result.Payments, PaymentToProto(&p))
	}
	return result
}

// ProtoToPaymentList converts a proto PaymentList to a slice of payments.
func ProtoToPaymentList(pb *PaymentList) []database.Payment {
	if pb == nil {
		return nil
	}
	result := make([]database.Payment, 0, len(pb.Payments))
	for _, p := range pb.Payments {
		result = append(result, *ProtoToPayment(p))
	}
	return result
}

// NIP43MembershipToProto converts a database.NIP43Membership to a proto NIP43Membership.
func NIP43MembershipToProto(m *database.NIP43Membership) *NIP43Membership {
	if m == nil {
		return nil
	}
	return &NIP43Membership{
		Pubkey:     m.Pubkey,
		AddedAt:    m.AddedAt.Unix(),
		InviteCode: m.InviteCode,
	}
}

// ProtoToNIP43Membership converts a proto NIP43Membership to a database.NIP43Membership.
func ProtoToNIP43Membership(pb *NIP43Membership) *database.NIP43Membership {
	if pb == nil {
		return nil
	}
	return &database.NIP43Membership{
		Pubkey:     pb.Pubkey,
		AddedAt:    timeFromUnix(pb.AddedAt),
		InviteCode: pb.InviteCode,
	}
}

// RangeToProto converts a database.Range to a proto Range.
func RangeToProto(r database.Range) *Range {
	return &Range{
		Prefix: r.Start,
		Start:  0,
		End:    0,
	}
}

// ProtoToRange converts a proto Range to a database.Range.
func ProtoToRange(pb *Range) database.Range {
	if pb == nil {
		return database.Range{}
	}
	return database.Range{
		Start: pb.Prefix,
		End:   nil,
	}
}

// EventMapToProto converts a map of serial->event to a proto EventMap.
func EventMapToProto(events map[uint64]*event.E) *EventMap {
	result := &EventMap{
		Events: make(map[uint64]*Event),
	}
	for serial, ev := range events {
		result.Events[serial] = EventToProto(ev)
	}
	return result
}

// ProtoToEventMap converts a proto EventMap to a map of serial->event.
func ProtoToEventMap(pb *EventMap) map[uint64]*event.E {
	if pb == nil {
		return nil
	}
	result := make(map[uint64]*event.E)
	for serial, ev := range pb.Events {
		result[serial] = ProtoToEvent(ev)
	}
	return result
}

// EventsToProto converts a slice of events to a slice of proto Events.
func EventsToProto(events event.S) []*Event {
	result := make([]*Event, 0, len(events))
	for _, ev := range events {
		result = append(result, EventToProto(ev))
	}
	return result
}

// ProtoToEvents converts a slice of proto Events to a slice of events.
func ProtoToEvents(pbs []*Event) event.S {
	result := make(event.S, 0, len(pbs))
	for _, pb := range pbs {
		result = append(result, ProtoToEvent(pb))
	}
	return result
}

// timeFromUnix converts a unix timestamp to time.Time (unexported).
func timeFromUnix(unix int64) time.Time {
	if unix == 0 {
		return time.Time{}
	}
	return time.Unix(unix, 0)
}

// TimeFromUnix converts a unix timestamp to time.Time (exported).
func TimeFromUnix(unix int64) time.Time {
	return timeFromUnix(unix)
}

// newUint40 creates a new Uint40 with the given value.
func newUint40(value uint64) *indextypes.Uint40 {
	u := &indextypes.Uint40{}
	_ = u.Set(value)
	return u
}

// BytesToTag converts a slice of byte slices to a tag.T.
func BytesToTag(ids [][]byte) *tag.T {
	t := tag.NewWithCap(len(ids))
	for _, id := range ids {
		t.T = append(t.T, id)
	}
	return t
}

// === Blob Storage Converters ===

// BlobMetadataToProto converts a database.BlobMetadata to a proto BlobMetadata.
func BlobMetadataToProto(m *database.BlobMetadata) *BlobMetadata {
	if m == nil {
		return nil
	}
	return &BlobMetadata{
		Pubkey:    m.Pubkey,
		MimeType:  m.MimeType,
		Size:      m.Size,
		Uploaded:  m.Uploaded,
		Extension: m.Extension,
	}
}

// ProtoToBlobMetadata converts a proto BlobMetadata to a database.BlobMetadata.
func ProtoToBlobMetadata(pb *BlobMetadata) *database.BlobMetadata {
	if pb == nil {
		return nil
	}
	return &database.BlobMetadata{
		Pubkey:    pb.Pubkey,
		MimeType:  pb.MimeType,
		Size:      pb.Size,
		Uploaded:  pb.Uploaded,
		Extension: pb.Extension,
	}
}

// BlobDescriptorToProto converts a database.BlobDescriptor to a proto BlobDescriptor.
// Note: NIP94 field is not transported via gRPC as it's generated at response time.
func BlobDescriptorToProto(d *database.BlobDescriptor) *BlobDescriptor {
	if d == nil {
		return nil
	}
	return &BlobDescriptor{
		Url:      d.URL,
		Sha256:   d.SHA256,
		Size:     d.Size,
		Type:     d.Type,
		Uploaded: d.Uploaded,
	}
}

// ProtoToBlobDescriptor converts a proto BlobDescriptor to a database.BlobDescriptor.
// Note: NIP94 field is not populated from proto as it's generated at response time.
func ProtoToBlobDescriptor(pb *BlobDescriptor) *database.BlobDescriptor {
	if pb == nil {
		return nil
	}
	return &database.BlobDescriptor{
		URL:      pb.Url,
		SHA256:   pb.Sha256,
		Size:     pb.Size,
		Type:     pb.Type,
		Uploaded: pb.Uploaded,
	}
}

// BlobDescriptorListToProto converts a slice of BlobDescriptors to proto.
func BlobDescriptorListToProto(descriptors []*database.BlobDescriptor) []*BlobDescriptor {
	result := make([]*BlobDescriptor, 0, len(descriptors))
	for _, d := range descriptors {
		result = append(result, BlobDescriptorToProto(d))
	}
	return result
}

// ProtoToBlobDescriptorList converts proto BlobDescriptors to a slice.
func ProtoToBlobDescriptorList(pbs []*BlobDescriptor) []*database.BlobDescriptor {
	result := make([]*database.BlobDescriptor, 0, len(pbs))
	for _, pb := range pbs {
		result = append(result, ProtoToBlobDescriptor(pb))
	}
	return result
}

// UserBlobStatsToProto converts a database.UserBlobStats to a proto UserBlobStats.
func UserBlobStatsToProto(s *database.UserBlobStats) *UserBlobStats {
	if s == nil {
		return nil
	}
	return &UserBlobStats{
		PubkeyHex:      s.PubkeyHex,
		BlobCount:      s.BlobCount,
		TotalSizeBytes: s.TotalSizeBytes,
	}
}

// ProtoToUserBlobStats converts a proto UserBlobStats to a database.UserBlobStats.
func ProtoToUserBlobStats(pb *UserBlobStats) *database.UserBlobStats {
	if pb == nil {
		return nil
	}
	return &database.UserBlobStats{
		PubkeyHex:      pb.PubkeyHex,
		BlobCount:      pb.BlobCount,
		TotalSizeBytes: pb.TotalSizeBytes,
	}
}

// UserBlobStatsListToProto converts a slice of UserBlobStats to proto.
func UserBlobStatsListToProto(stats []*database.UserBlobStats) []*UserBlobStats {
	result := make([]*UserBlobStats, 0, len(stats))
	for _, s := range stats {
		result = append(result, UserBlobStatsToProto(s))
	}
	return result
}

// ProtoToUserBlobStatsList converts proto UserBlobStats to a slice.
func ProtoToUserBlobStatsList(pbs []*UserBlobStats) []*database.UserBlobStats {
	result := make([]*database.UserBlobStats, 0, len(pbs))
	for _, pb := range pbs {
		result = append(result, ProtoToUserBlobStats(pb))
	}
	return result
}
