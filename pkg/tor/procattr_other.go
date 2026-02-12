//go:build !linux

package tor

import "syscall"

// sysProcAttr returns nil on non-Linux platforms where Pdeathsig is unavailable.
func sysProcAttr() *syscall.SysProcAttr {
	return nil
}
