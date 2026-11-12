//go:build windows

package xray

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// listPIDs mirrors webui.py _xray_procs on Windows:
// tasklist /FI "IMAGENAME eq <bin>" /FO CSV /NH, then compare the first CSV
// column (lowercased) with the binary name and take the second as the PID.
func listPIDs(binname string) []int {
	out, _ := exec.Command("tasklist", "/FI", "IMAGENAME eq "+binname,
		"/FO", "CSV", "/NH").Output()
	var pids []int
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, `","`)
		if len(parts) < 2 {
			continue
		}
		for i := range parts {
			parts[i] = strings.Trim(strings.TrimSpace(parts[i]), `"`)
		}
		if strings.ToLower(parts[0]) != binname {
			continue
		}
		if pid, err := strconv.Atoi(parts[1]); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

// killPID mirrors webui.py _kill_pid on Windows.
func killPID(pid int) {
	exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
}

// terminate is Process.terminate() on Windows: TerminateProcess.
func terminate(p *os.Process) error {
	return p.Kill()
}
