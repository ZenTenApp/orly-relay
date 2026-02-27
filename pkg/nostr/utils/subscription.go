package utils

import (
	"fmt"
)

func NewSubscription(n int) []byte {
	return []byte(fmt.Sprintf("sub:%d", n))
}
