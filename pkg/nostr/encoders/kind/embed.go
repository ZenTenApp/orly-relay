package kind

import (
	_ "embed"
	"encoding/json"
)

//go:embed kinds.json
var KindsJSON []byte

// KindData represents the full structure of the kinds.json file
type KindData struct {
	Version    string            `json:"version"`
	Source     string            `json:"source"`
	Ranges     Ranges            `json:"ranges"`
	Kinds      map[string]KindInfo `json:"kinds"`
	Privileged []int             `json:"privileged"`
	Directory  []int             `json:"directory"`
	Aliases    map[string]int    `json:"aliases"`
}

// Ranges defines the kind number ranges for each event classification
type Ranges struct {
	Regular      Range `json:"regular"`
	Replaceable  Range `json:"replaceable"`
	Ephemeral    Range `json:"ephemeral"`
	Parameterized Range `json:"parameterized"`
}

// Range defines a start and end for a kind range
type Range struct {
	Start       int    `json:"start"`
	End         int    `json:"end"`
	Description string `json:"description"`
}

// KindInfo contains metadata about a specific event kind
type KindInfo struct {
	Name           string  `json:"name"`
	NIP            *string `json:"nip,omitempty"`
	Description    string  `json:"description"`
	Classification string  `json:"classification,omitempty"`
	Deprecated     bool    `json:"deprecated,omitempty"`
	DeprecatedBy   string  `json:"deprecatedBy,omitempty"`
	Spec           string  `json:"spec,omitempty"`
	RangeEnd       int     `json:"rangeEnd,omitempty"`
}

var kindData *KindData

func init() {
	kindData = &KindData{}
	if err := json.Unmarshal(KindsJSON, kindData); err != nil {
		panic("failed to parse kinds.json: " + err.Error())
	}

	// Populate the Map from JSON data for backward compatibility
	for kStr, info := range kindData.Kinds {
		var k int
		if err := json.Unmarshal([]byte(kStr), &k); err == nil {
			Map[uint16(k)] = info.Name
		}
	}
}

// GetKindData returns the parsed kind data from the embedded JSON
func GetKindData() *KindData {
	return kindData
}

// GetKindInfo returns the KindInfo for a specific kind number
func GetKindInfo(k uint16) (KindInfo, bool) {
	kStr := json.Number(string(rune('0' + k))).String()
	// Manual conversion since json.Number doesn't work like that
	kStr = itoa(int(k))
	info, ok := kindData.Kinds[kStr]
	return info, ok
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var digits []byte
	negative := n < 0
	if negative {
		n = -n
	}

	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}

// GetDescription returns the description for a kind number
func GetDescription(k uint16) string {
	if info, ok := GetKindInfo(k); ok {
		return info.Description
	}
	return ""
}

// GetNIP returns the NIP reference for a kind number
func GetNIP(k uint16) string {
	if info, ok := GetKindInfo(k); ok && info.NIP != nil {
		return *info.NIP
	}
	return ""
}

// IsDeprecated returns whether a kind is deprecated
func IsDeprecated(k uint16) bool {
	if info, ok := GetKindInfo(k); ok {
		return info.Deprecated
	}
	return false
}

// GetClassification returns the classification (regular, replaceable, ephemeral, parameterized) for a kind
func GetClassification(k uint16) string {
	// First check explicit classification in JSON
	if info, ok := GetKindInfo(k); ok && info.Classification != "" {
		return info.Classification
	}

	// Fall back to range-based classification
	if k >= uint16(kindData.Ranges.Parameterized.Start) && k <= uint16(kindData.Ranges.Parameterized.End) {
		return "parameterized"
	}
	if k >= uint16(kindData.Ranges.Ephemeral.Start) && k < uint16(kindData.Ranges.Ephemeral.End) {
		return "ephemeral"
	}
	if k >= uint16(kindData.Ranges.Replaceable.Start) && k < uint16(kindData.Ranges.Replaceable.End) {
		return "replaceable"
	}
	if k >= uint16(kindData.Ranges.Regular.Start) && k <= uint16(kindData.Ranges.Regular.End) {
		return "regular"
	}

	// Core kinds 0 and 3 are replaceable
	if k == 0 || k == 3 {
		return "replaceable"
	}

	return "regular"
}
