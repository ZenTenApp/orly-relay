package graph

import (
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/filter"
)

func TestQueryValidate(t *testing.T) {
	validSeed := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tests := []struct {
		name    string
		query   Query
		wantErr error
	}{
		{
			name: "valid follows query",
			query: Query{
				Method: "follows",
				Seed:   validSeed,
				Depth:  2,
			},
			wantErr: nil,
		},
		{
			name: "valid followers query",
			query: Query{
				Method: "followers",
				Seed:   validSeed,
			},
			wantErr: nil,
		},
		{
			name: "valid mentions query",
			query: Query{
				Method: "mentions",
				Seed:   validSeed,
				Depth:  1,
			},
			wantErr: nil,
		},
		{
			name: "valid thread query",
			query: Query{
				Method: "thread",
				Seed:   validSeed,
				Depth:  10,
			},
			wantErr: nil,
		},
		{
			name: "valid query with inbound refs",
			query: Query{
				Method: "follows",
				Seed:   validSeed,
				Depth:  2,
				InboundRefs: []RefSpec{
					{Kinds: []int{7}, FromDepth: 1},
				},
			},
			wantErr: nil,
		},
		{
			name: "valid query with multiple ref specs",
			query: Query{
				Method: "follows",
				Seed:   validSeed,
				InboundRefs: []RefSpec{
					{Kinds: []int{7}, FromDepth: 1},
					{Kinds: []int{6}, FromDepth: 1},
				},
				OutboundRefs: []RefSpec{
					{Kinds: []int{1}, FromDepth: 0},
				},
			},
			wantErr: nil,
		},
		{
			name:    "missing method",
			query:   Query{Seed: validSeed},
			wantErr: ErrMissingMethod,
		},
		{
			name: "invalid method",
			query: Query{
				Method: "invalid",
				Seed:   validSeed,
			},
			wantErr: ErrInvalidMethod,
		},
		{
			name: "missing seed",
			query: Query{
				Method: "follows",
			},
			wantErr: ErrMissingSeed,
		},
		{
			name: "seed too short",
			query: Query{
				Method: "follows",
				Seed:   "abc123",
			},
			wantErr: ErrInvalidSeed,
		},
		{
			name: "seed with invalid characters",
			query: Query{
				Method: "follows",
				Seed:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdeg",
			},
			wantErr: ErrInvalidSeed,
		},
		{
			name: "depth too high",
			query: Query{
				Method: "follows",
				Seed:   validSeed,
				Depth:  17,
			},
			wantErr: ErrDepthTooHigh,
		},
		{
			name: "empty ref spec kinds",
			query: Query{
				Method: "follows",
				Seed:   validSeed,
				InboundRefs: []RefSpec{
					{Kinds: []int{}, FromDepth: 1},
				},
			},
			wantErr: ErrEmptyRefSpecKinds,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.query.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err != tt.wantErr {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestQueryDefaults(t *testing.T) {
	validSeed := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	q := Query{
		Method: "follows",
		Seed:   validSeed,
		Depth:  0, // Should default to 1
	}

	err := q.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.Depth != 1 {
		t.Errorf("Depth = %d, want 1 (default)", q.Depth)
	}
}

func TestKindsAtDepth(t *testing.T) {
	q := Query{
		Method: "follows",
		Seed:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Depth:  3,
		InboundRefs: []RefSpec{
			{Kinds: []int{7}, FromDepth: 0},       // From seed
			{Kinds: []int{6, 16}, FromDepth: 1},   // From depth 1
			{Kinds: []int{9735}, FromDepth: 2},    // From depth 2
		},
		OutboundRefs: []RefSpec{
			{Kinds: []int{1}, FromDepth: 1},
		},
	}

	// Test inbound kinds at depth 0
	kinds0 := q.InboundKindsAtDepth(0)
	if !kinds0[7] || kinds0[6] || kinds0[9735] {
		t.Errorf("InboundKindsAtDepth(0) = %v, want only kind 7", kinds0)
	}

	// Test inbound kinds at depth 1
	kinds1 := q.InboundKindsAtDepth(1)
	if !kinds1[7] || !kinds1[6] || !kinds1[16] || kinds1[9735] {
		t.Errorf("InboundKindsAtDepth(1) = %v, want kinds 7, 6, 16", kinds1)
	}

	// Test inbound kinds at depth 2
	kinds2 := q.InboundKindsAtDepth(2)
	if !kinds2[7] || !kinds2[6] || !kinds2[16] || !kinds2[9735] {
		t.Errorf("InboundKindsAtDepth(2) = %v, want all kinds", kinds2)
	}

	// Test outbound kinds at depth 0
	outKinds0 := q.OutboundKindsAtDepth(0)
	if len(outKinds0) != 0 {
		t.Errorf("OutboundKindsAtDepth(0) = %v, want empty", outKinds0)
	}

	// Test outbound kinds at depth 1
	outKinds1 := q.OutboundKindsAtDepth(1)
	if !outKinds1[1] {
		t.Errorf("OutboundKindsAtDepth(1) = %v, want kind 1", outKinds1)
	}
}

func TestExtractFromFilter(t *testing.T) {
	tests := []struct {
		name       string
		filterJSON string
		wantQuery  bool
		wantErr    bool
	}{
		{
			name:       "filter with valid graph query",
			filterJSON: `{"kinds":[1],"_graph":{"method":"follows","seed":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","depth":2}}`,
			wantQuery:  true,
			wantErr:    false,
		},
		{
			name:       "filter without graph query",
			filterJSON: `{"kinds":[1,7]}`,
			wantQuery:  false,
			wantErr:    false,
		},
		{
			name:       "filter with invalid graph query (missing method)",
			filterJSON: `{"kinds":[1],"_graph":{"seed":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}}`,
			wantQuery:  false,
			wantErr:    true,
		},
		{
			name:       "filter with complex graph query",
			filterJSON: `{"kinds":[0],"_graph":{"method":"follows","seed":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","depth":3,"inbound_refs":[{"kinds":[7],"from_depth":1}]}}`,
			wantQuery:  true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &filter.F{}
			_, err := f.Unmarshal([]byte(tt.filterJSON))
			if err != nil {
				t.Fatalf("failed to unmarshal filter: %v", err)
			}

			q, err := ExtractFromFilter(f)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantQuery && q == nil {
				t.Error("expected query, got nil")
			}
			if !tt.wantQuery && q != nil {
				t.Errorf("expected nil query, got %+v", q)
			}
		})
	}
}

func TestIsGraphQuery(t *testing.T) {
	tests := []struct {
		name       string
		filterJSON string
		want       bool
	}{
		{
			name:       "filter with graph query",
			filterJSON: `{"kinds":[1],"_graph":{"method":"follows","seed":"abc"}}`,
			want:       true,
		},
		{
			name:       "filter without graph query",
			filterJSON: `{"kinds":[1,7]}`,
			want:       false,
		},
		{
			name:       "filter with other extension",
			filterJSON: `{"kinds":[1],"_custom":"value"}`,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &filter.F{}
			_, err := f.Unmarshal([]byte(tt.filterJSON))
			if err != nil {
				t.Fatalf("failed to unmarshal filter: %v", err)
			}

			got := IsGraphQuery(f)
			if got != tt.want {
				t.Errorf("IsGraphQuery() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQueryHasRefs(t *testing.T) {
	tests := []struct {
		name           string
		query          Query
		hasInbound     bool
		hasOutbound    bool
		hasRefs        bool
	}{
		{
			name: "no refs",
			query: Query{
				Method: "follows",
				Seed:   "abc",
			},
			hasInbound:  false,
			hasOutbound: false,
			hasRefs:     false,
		},
		{
			name: "only inbound refs",
			query: Query{
				Method: "follows",
				Seed:   "abc",
				InboundRefs: []RefSpec{
					{Kinds: []int{7}},
				},
			},
			hasInbound:  true,
			hasOutbound: false,
			hasRefs:     true,
		},
		{
			name: "only outbound refs",
			query: Query{
				Method: "follows",
				Seed:   "abc",
				OutboundRefs: []RefSpec{
					{Kinds: []int{1}},
				},
			},
			hasInbound:  false,
			hasOutbound: true,
			hasRefs:     true,
		},
		{
			name: "both refs",
			query: Query{
				Method: "follows",
				Seed:   "abc",
				InboundRefs: []RefSpec{
					{Kinds: []int{7}},
				},
				OutboundRefs: []RefSpec{
					{Kinds: []int{1}},
				},
			},
			hasInbound:  true,
			hasOutbound: true,
			hasRefs:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.query.HasInboundRefs(); got != tt.hasInbound {
				t.Errorf("HasInboundRefs() = %v, want %v", got, tt.hasInbound)
			}
			if got := tt.query.HasOutboundRefs(); got != tt.hasOutbound {
				t.Errorf("HasOutboundRefs() = %v, want %v", got, tt.hasOutbound)
			}
			if got := tt.query.HasRefs(); got != tt.hasRefs {
				t.Errorf("HasRefs() = %v, want %v", got, tt.hasRefs)
			}
		})
	}
}
