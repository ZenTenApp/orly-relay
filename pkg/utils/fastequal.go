package utils

import "git.mleku.dev/mleku/nostr/utils/constraints"

func FastEqual[A constraints.Bytes, B constraints.Bytes](a A, b B) (same bool) {
	if len(a) != len(b) {
		return
	}
	ab := []byte(a)
	bb := []byte(b)
	for i, v := range ab {
		if v != bb[i] {
			return
		}
	}
	return true
}

func FastCompare[A constraints.Bytes, B constraints.Bytes](
	a A, b B,
) (diff int) {
	if len(a) != len(b) {
		return
	}
	ab := []byte(a)
	bb := []byte(b)
	for i, v := range ab {
		if v != bb[i] {
			if v > bb[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}
