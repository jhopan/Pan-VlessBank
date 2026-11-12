//go:build !windows

package cf

import (
	"os"
	"syscall"
)

// terminate mirrors Process.terminate() on POSIX (SIGTERM).
func terminate(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}
