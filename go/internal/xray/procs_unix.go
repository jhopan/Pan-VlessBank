//go:build !windows

package xray

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// listPIDs mirrors webui.py _xray_procs on POSIX: scan /proc/<pid>/cmdline and
// match the lowercased basename of argv[0] against the binary name.
func listPIDs(binname string) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var pids []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil {
			continue
		}
		argv := strings.Split(string(raw), "\x00")
		if len(argv) == 0 || argv[0] == "" {
			continue
		}
		if strings.Contains(strings.ToLower(filepath.Base(argv[0])), binname) {
			if pid, err := strconv.Atoi(e.Name()); err == nil {
				pids = append(pids, pid)
			}
		}
	}
	return pids
}

// killPID sends SIGTERM, then SIGKILL when the process is still there.
func killPID(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !pidAlive(pid) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

// terminate is Process.terminate() on POSIX: SIGTERM.
func terminate(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}

func pidAlive(pid int) bool {
	_, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid)))
	return err == nil
}
