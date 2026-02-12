package tor

import "syscall"

// sysProcAttr returns platform-specific process attributes for the Tor subprocess.
// On Linux, Pdeathsig ensures the kernel sends SIGKILL to the Tor process
// when its parent dies, preventing orphaned Tor processes.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
}
