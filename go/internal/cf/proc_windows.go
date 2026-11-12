//go:build windows

package cf

import "os"

// terminate mirrors Process.terminate() on Windows (TerminateProcess).
func terminate(p *os.Process) error {
	return p.Kill()
}
