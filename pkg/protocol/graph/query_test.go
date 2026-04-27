package graph

import (
	"testing"

	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
)

func TestQueryValidate(t *testing.T) {
	validPubkey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tests := []struct {
		name    string
		query   Query
		wantErr error
	}{
		{
			name:    "valid pp/out query",
			query:   Query{Pubkey: validPubkey, Depth: 2, Edge: "pp", Direction: "out"},
			wantErr: nil,
		},
		{
			name:    "valid pp/in query",
			query:   Query{Pubkey: validPubkey, Depth: 1, Edge: "pp", Direction: "in"},
			wantErr: nil,
		},
		{
			name:    "valid pe/both query",
			query:   Query{Pubkey: validPubkey, Depth: 3, Edge: "pe", Direction: "both"},
			wantErr: nil,
		},
		{
			name:    "valid ee/out query",
			query:   Query{Pubkey: validPubkey, Depth: 1, Edge: "ee", Direction: "out"},
			wantErr: nil,
		},
		{
			name:    "depth 0 defaults to 1",
			query:   Query{Pubkey: validPubkey, Depth: 0, Edge: "pp", Direction: "out"},
			wantErr: nil,
		},
		{
			name:    "missing pubkey",
			query:   Query{Edge: "pp", Direction: "out"},
			wantErr: ErrMissingPubkey,
		},
		{
			name:    "pubkey too short",
			query:   Query{Pubkey: "abc123", Edge: "pp", Direction: "out"},
			wantErr: ErrInvalidPubkey,
		},
		{
			name:    "pubkey with invalid hex",
			query:   Query{Pubkey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdeg", Edge: "pp", Direction: "out"},
			wantErr: ErrInvalidPubkey,
		},
		{
			name:    "depth too high",
			query:   Query{Pubkey: validPubkey, Depth: 17, Edge: "pp", Direction: "out"},
			wantErr: ErrInvalidDepth,
		},
		{
			name:    "missing edge",
			query:   Query{Pubkey: validPubkey, Direction: "out"},
			wantErr: ErrMissingEdge,
		},
		{
			name:    "invalid edge",
			query:   Query{Pubkey: validPubkey, Edge: "xx", Direction: "out"},
			wantErr: ErrInvalidEdge,
		},
		{
			name:    "missing direction",
			query:   Query{Pubkey: validPubkey, Edge: "pp"},
			wantErr: ErrMissingDirection,
		},
		{
			name:    "invalid direction",
			query:   Query{Pubkey: validPubkey, Edge: "pp", Direction: "left"},
			wantErr: ErrInvalidDirection,
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
	validPubkey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	q := Query{Pubkey: validPubkey, Depth: 0, Edge: "pp", Direction: "out"}
	if err := q.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Depth != 1 {
		t.Errorf("Depth = %d, want 1 (default)", q.Depth)
	}
}

func TestExtractFromFilter_Array(t *testing.T) {
	validPubkey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	tests := []struct {
		name       string
		filterJSON string
		wantQuery  bool
		wantErr    bool
		wantEdge   string
		wantDir    string
	}{
		{
			name:       "4-element array format",
			filterJSON: `{"kinds":[1],"_graph":["` + validPubkey + `",2,"pp","out"]}`,
			wantQuery:  true,
			wantEdge:   "pp",
			wantDir:    "out",
		},
		{
			name:       "legacy object format (backward compat)",
			filterJSON: `{"kinds":[1],"_graph":{"pubkey":"` + validPubkey + `","depth":2,"edge":"pp","direction":"out"}}`,
			wantQuery:  true,
			wantEdge:   "pp",
			wantDir:    "out",
		},
		{
			name:       "no graph query",
			filterJSON: `{"kinds":[1,7]}`,
			wantQuery:  false,
		},
		{
			name:       "invalid array length",
			filterJSON: `{"kinds":[1],"_graph":["` + validPubkey + `",2,"pp"]}`,
			wantErr:    true,
		},
		{
			name:       "invalid edge in array",
			filterJSON: `{"kinds":[1],"_graph":["` + validPubkey + `",2,"xx","out"]}`,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &filter.F{}
			if _, err := f.Unmarshal([]byte(tt.filterJSON)); err != nil {
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

			if tt.wantQuery {
				if q == nil {
					t.Fatal("expected query, got nil")
				}
				if q.Edge != tt.wantEdge {
					t.Errorf("Edge = %q, want %q", q.Edge, tt.wantEdge)
				}
				if q.Direction != tt.wantDir {
					t.Errorf("Direction = %q, want %q", q.Direction, tt.wantDir)
				}
			} else if q != nil {
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
			filterJSON: `{"kinds":[1],"_graph":["abc",1,"pp","out"]}`,
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
			if _, err := f.Unmarshal([]byte(tt.filterJSON)); err != nil {
				t.Fatalf("failed to unmarshal filter: %v", err)
			}
			if got := IsGraphQuery(f); got != tt.want {
				t.Errorf("IsGraphQuery() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQueryEdgeHelpers(t *testing.T) {
	tests := []struct {
		edge      string
		direction string
		pp, pe, ee bool
		out, in_, both bool
	}{
		{"pp", "out", true, false, false, true, false, false},
		{"pe", "in", false, true, false, false, true, false},
		{"ee", "both", false, false, true, true, true, true},
		{"pp", "both", true, false, false, true, true, true},
	}

	for _, tt := range tests {
		q := Query{Edge: tt.edge, Direction: tt.direction}
		if q.IsPubkeyPubkey() != tt.pp {
			t.Errorf("edge=%s: IsPubkeyPubkey() = %v, want %v", tt.edge, q.IsPubkeyPubkey(), tt.pp)
		}
		if q.IsPubkeyEvent() != tt.pe {
			t.Errorf("edge=%s: IsPubkeyEvent() = %v, want %v", tt.edge, q.IsPubkeyEvent(), tt.pe)
		}
		if q.IsEventEvent() != tt.ee {
			t.Errorf("edge=%s: IsEventEvent() = %v, want %v", tt.edge, q.IsEventEvent(), tt.ee)
		}
		if q.IsOutbound() != tt.out {
			t.Errorf("dir=%s: IsOutbound() = %v, want %v", tt.direction, q.IsOutbound(), tt.out)
		}
		if q.IsInbound() != tt.in_ {
			t.Errorf("dir=%s: IsInbound() = %v, want %v", tt.direction, q.IsInbound(), tt.in_)
		}
		if q.IsBidirectional() != tt.both {
			t.Errorf("dir=%s: IsBidirectional() = %v, want %v", tt.direction, q.IsBidirectional(), tt.both)
		}
	}
}
