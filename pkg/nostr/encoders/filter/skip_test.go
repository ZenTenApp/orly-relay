package filter

import (
	"testing"
)

func TestSkipJSONValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantVal  string
		wantRem  string
		wantErr  bool
	}{
		// Objects
		{
			name:    "empty object",
			input:   `{}`,
			wantVal: `{}`,
			wantRem: "",
		},
		{
			name:    "simple object",
			input:   `{"foo":"bar"}`,
			wantVal: `{"foo":"bar"}`,
			wantRem: "",
		},
		{
			name:    "nested object",
			input:   `{"a":{"b":"c"}},"next"`,
			wantVal: `{"a":{"b":"c"}}`,
			wantRem: `,"next"`,
		},
		{
			name:    "object with array",
			input:   `{"items":[1,2,3]}rest`,
			wantVal: `{"items":[1,2,3]}`,
			wantRem: `rest`,
		},

		// Arrays
		{
			name:    "empty array",
			input:   `[]`,
			wantVal: `[]`,
			wantRem: "",
		},
		{
			name:    "number array",
			input:   `[1,2,3]`,
			wantVal: `[1,2,3]`,
			wantRem: "",
		},
		{
			name:    "string array",
			input:   `["a","b","c"],rest`,
			wantVal: `["a","b","c"]`,
			wantRem: `,rest`,
		},
		{
			name:    "nested array",
			input:   `[[1,2],[3,4]]more`,
			wantVal: `[[1,2],[3,4]]`,
			wantRem: `more`,
		},

		// Strings
		{
			name:    "simple string",
			input:   `"hello"`,
			wantVal: `"hello"`,
			wantRem: "",
		},
		{
			name:    "string with escapes",
			input:   `"hello \"world\""rest`,
			wantVal: `"hello \"world\""`,
			wantRem: `rest`,
		},
		{
			name:    "string with backslash",
			input:   `"path\\to\\file",next`,
			wantVal: `"path\\to\\file"`,
			wantRem: `,next`,
		},

		// Numbers
		{
			name:    "integer",
			input:   `123`,
			wantVal: `123`,
			wantRem: "",
		},
		{
			name:    "negative integer",
			input:   `-456,next`,
			wantVal: `-456`,
			wantRem: `,next`,
		},
		{
			name:    "decimal",
			input:   `3.14159}`,
			wantVal: `3.14159`,
			wantRem: `}`,
		},
		{
			name:    "scientific notation",
			input:   `1.23e-4,next`,
			wantVal: `1.23e-4`,
			wantRem: `,next`,
		},

		// Booleans
		{
			name:    "true",
			input:   `true`,
			wantVal: `true`,
			wantRem: "",
		},
		{
			name:    "false",
			input:   `false,next`,
			wantVal: `false`,
			wantRem: `,next`,
		},

		// Null
		{
			name:    "null",
			input:   `null`,
			wantVal: `null`,
			wantRem: "",
		},
		{
			name:    "null with remainder",
			input:   `null}`,
			wantVal: `null`,
			wantRem: `}`,
		},

		// Complex nested structures
		{
			name:    "graph query object",
			input:   `{"method":"follows","seed":"abc123","depth":2,"inbound_refs":[{"kinds":[7],"from_depth":1}]},rest`,
			wantVal: `{"method":"follows","seed":"abc123","depth":2,"inbound_refs":[{"kinds":[7],"from_depth":1}]}`,
			wantRem: `,rest`,
		},

		// Error cases
		{
			name:    "empty input",
			input:   ``,
			wantErr: true,
		},
		{
			name:    "unclosed object",
			input:   `{"foo":"bar"`,
			wantErr: true,
		},
		{
			name:    "unclosed array",
			input:   `[1,2,3`,
			wantErr: true,
		},
		{
			name:    "unclosed string",
			input:   `"hello`,
			wantErr: true,
		},
		{
			name:    "invalid true",
			input:   `tru`,
			wantErr: true,
		},
		{
			name:    "invalid false",
			input:   `fals`,
			wantErr: true,
		},
		{
			name:    "invalid null",
			input:   `nul`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, rem, err := skipJSONValue([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if string(val) != tt.wantVal {
				t.Errorf("val = %q, want %q", string(val), tt.wantVal)
			}

			if string(rem) != tt.wantRem {
				t.Errorf("rem = %q, want %q", string(rem), tt.wantRem)
			}
		})
	}
}

func TestUnmarshalWithUnknownFields(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantKinds  []int
		wantExtra  map[string]string
		wantErr    bool
	}{
		{
			name:      "simple filter with _graph extension",
			input:     `{"kinds":[1,7],"_graph":{"method":"follows","seed":"abc123","depth":2}}`,
			wantKinds: []int{1, 7},
			wantExtra: map[string]string{
				"_graph": `{"method":"follows","seed":"abc123","depth":2}`,
			},
		},
		{
			name:      "filter with unknown string field",
			input:     `{"kinds":[1],"_custom":"value"}`,
			wantKinds: []int{1},
			wantExtra: map[string]string{
				"_custom": `"value"`,
			},
		},
		{
			name:      "filter with multiple unknown fields",
			input:     `{"kinds":[1],"_foo":123,"_bar":["a","b"]}`,
			wantKinds: []int{1},
			wantExtra: map[string]string{
				"_foo": `123`,
				"_bar": `["a","b"]`,
			},
		},
		{
			name:      "filter with complex _graph extension",
			input:     `{"kinds":[0],"_graph":{"method":"follows","seed":"abc","depth":2,"inbound_refs":[{"kinds":[7],"from_depth":1}]}}`,
			wantKinds: []int{0},
			wantExtra: map[string]string{
				"_graph": `{"method":"follows","seed":"abc","depth":2,"inbound_refs":[{"kinds":[7],"from_depth":1}]}`,
			},
		},
		{
			name:      "unknown field before known fields",
			input:     `{"_unknown":true,"kinds":[3]}`,
			wantKinds: []int{3},
			wantExtra: map[string]string{
				"_unknown": `true`,
			},
		},
		{
			name:      "unknown field with null value",
			input:     `{"kinds":[1],"_nullable":null}`,
			wantKinds: []int{1},
			wantExtra: map[string]string{
				"_nullable": `null`,
			},
		},
		{
			name:      "standard filter without unknown fields",
			input:     `{"kinds":[1,7]}`,
			wantKinds: []int{1, 7},
			wantExtra: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &F{}
			_, err := f.Unmarshal([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check kinds
			if f.Kinds != nil {
				if f.Kinds.Len() != len(tt.wantKinds) {
					t.Errorf("kinds len = %d, want %d", f.Kinds.Len(), len(tt.wantKinds))
				} else {
					for i, k := range f.Kinds.K {
						if int(k.K) != tt.wantKinds[i] {
							t.Errorf("kinds[%d] = %d, want %d", i, k.K, tt.wantKinds[i])
						}
					}
				}
			} else if len(tt.wantKinds) > 0 {
				t.Errorf("kinds is nil, want %v", tt.wantKinds)
			}

			// Check extra fields
			if tt.wantExtra == nil {
				if f.Extra != nil && len(f.Extra) > 0 {
					t.Errorf("extra = %v, want nil", f.Extra)
				}
			} else {
				if f.Extra == nil {
					t.Errorf("extra is nil, want %v", tt.wantExtra)
					return
				}
				for key, wantVal := range tt.wantExtra {
					gotVal, ok := f.Extra[key]
					if !ok {
						t.Errorf("extra[%q] not found", key)
						continue
					}
					if string(gotVal) != wantVal {
						t.Errorf("extra[%q] = %q, want %q", key, string(gotVal), wantVal)
					}
				}
				for key := range f.Extra {
					if _, ok := tt.wantExtra[key]; !ok {
						t.Errorf("unexpected extra key %q", key)
					}
				}
			}
		})
	}
}
