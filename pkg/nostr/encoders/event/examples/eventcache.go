// Package examples is an embedded jsonl format of a collection of events
// intended to be used to test an event codec.
package examples

import (
	_ "embed"
)

//go:embed out.jsonl
var Cache []byte
