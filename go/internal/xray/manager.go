// Package xray manages the single xray process driven by the panel.
//
// Port of the xray section of webui.py: one instance only, strays from a
// previous panel run are detected through the OS and killed by Stop, and all
// state transitions are serialized with a mutex.
package xray

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// spawnGrace is how long Start waits before it decides the child survived
// the first 800ms of its life (webui.py: time.sleep(0.8)).
const spawnGrace = 800 * time.Millisecond

// Manager owns at most one xray child process plus the knowledge about stray
// xray processes left behind by an earlier panel run.
type Manager struct {
	mu       sync.Mutex
	cfgPath  string
	logPath  string
	xrayPath string
	resolve  func() string

	proc     *exec.Cmd
	procDone chan struct{}
	procGen  int
	logFile  *os.File
}

// exitInfo is written by the wait goroutine before it closes done, so readers
// that observe the closed channel see code without extra locking.
type exitInfo struct {
	code int
	done chan struct{}
}

// New builds a Manager. cfgPath is config.json, logPath xray.log, xrayPath the
// --xray flag default and resolve the "settings path -> folder binary -> PATH"
// scanner (may be nil).
func New(cfgPath, logPath, xrayPath string, resolve func() string) *Manager {
	return &Manager{cfgPath: cfgPath, logPath: logPath, xrayPath: xrayPath, resolve: resolve}
}

// Bin returns the xray binary that would be spawned now.
func (m *Manager) Bin() string {
	if m.resolve != nil {
		if p := m.resolve(); p != "" {
			return p
		}
	}
	if m.xrayPath != "" {
		return m.xrayPath
	}
	return "xray"
}

// PIDs returns the PIDs of every xray process on the machine, including strays
// from a previous panel run (webui.py _xray_procs).
func (m *Manager) PIDs() []int {
	binname := strings.ToLower(filepath.Base(m.Bin()))
	return listPIDs(binname)
}

// Running reports whether xray runs: our own child is alive or any OS process
// matches the binary name.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.aliveLocked() || len(m.PIDs()) > 0
}

// Start launches xray and returns the Indonesian status message of webui.py.
func (m *Manager) Start() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.aliveLocked() {
		return fmt.Sprintf("xray sudah jalan (pid %d)", m.proc.Process.Pid)
	}
	if lain := m.PIDs(); len(lain) > 0 {
		m.proc = nil
		return "xray sudah jalan (pid " + joinInts(lain) + ") - klik Stop dulu kalau mau start ulang"
	}
	if _, err := os.Stat(m.cfgPath); err != nil {
		return "config.json belum ada - klik Regenerate dulu"
	}

	binp := m.Bin()
	logf, err := os.OpenFile(m.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "xray gagal start (exit -1) - cek Log"
	}
	if m.logFile != nil {
		m.logFile.Close()
	}
	m.logFile = logf

	cmd := exec.Command(binp, "run", "-c", m.cfgPath)
	cmd.Stdout = logf
	cmd.Stderr = logf
	if err := cmd.Start(); err != nil {
		logf.Close()
		m.logFile = nil
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return fmt.Sprintf("xray binary tidak ketemu: %s (atur di Pengaturan)", binp)
		}
		return "xray gagal start (exit -1) - cek Log"
	}

	m.procGen++
	gen := m.procGen
	res := &exitInfo{done: make(chan struct{})}
	m.proc = cmd
	m.procDone = res.done
	pid := cmd.Process.Pid

	go func() {
		werr := cmd.Wait()
		var ee *exec.ExitError
		if errors.As(werr, &ee) {
			res.code = ee.ExitCode()
		}
		close(res.done)
		m.mu.Lock()
		if m.procGen == gen && m.logFile != nil {
			m.logFile.Close()
			m.logFile = nil
		}
		m.mu.Unlock()
	}()

	time.Sleep(spawnGrace)
	if drain(res.done) {
		m.proc = nil
		return fmt.Sprintf("xray gagal start (exit %d) - cek Log", res.code)
	}
	return fmt.Sprintf("xray start (pid %d)", pid)
}

// Stop kills our child and every stray xray process. It returns the Indonesian
// status message of webui.py.
func (m *Manager) Stop() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	killed := []int{}
	if m.proc != nil {
		if !drain(m.procDone) {
			p := m.proc.Process
			terminate(p)
			if !closed(m.procDone, 5*time.Second) {
				p.Kill()
			}
			killed = append(killed, p.Pid)
		}
		m.proc = nil
		m.procGen++
		m.procDone = nil
	}
	if m.logFile != nil {
		m.logFile.Close()
		m.logFile = nil
	}
	for _, pid := range m.PIDs() {
		if !containsInt(killed, pid) {
			killPID(pid)
			killed = append(killed, pid)
		}
	}
	if len(killed) == 0 {
		return "xray memang mati"
	}
	return fmt.Sprintf("xray stop (%d proses, pid %s)", len(killed), joinInts(killed))
}

// Restart stops then starts, returning the Start message.
func (m *Manager) Restart() string {
	m.Stop()
	return m.Start()
}

// TailLog returns the last n lines of xray.log, or "log kosong" when missing.
// CRLF dinormalkan seperti Python text-mode (universal newlines) supaya tampilan sama.
func (m *Manager) TailLog(n int) string {
	b, err := os.ReadFile(m.logPath)
	if err != nil {
		return "log kosong"
	}
	return tailLines(strings.ReplaceAll(string(b), "\r\n", "\n"), n)
}

func (m *Manager) aliveLocked() bool {
	return m.proc != nil && !drain(m.procDone)
}

// drain reports whether done is already closed, without blocking.
func drain(done <-chan struct{}) bool {
	if done == nil {
		return false
	}
	select {
	case <-done:
		return true
	default:
		return false
	}
}

// tailLines mirrors Python readlines()[-n:] + "".join(...): every element but
// the last keeps its newline.
func tailLines(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	lines := make([]string, 0, n)
	start := 0
	for i := range s {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "")
}

func joinInts(v []int) string {
	parts := make([]string, len(v))
	for i, n := range v {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ", ")
}

func containsInt(v []int, x int) bool {
	for _, n := range v {
		if n == x {
			return true
		}
	}
	return false
}

func closed(done <-chan struct{}, d time.Duration) bool {
	if done == nil {
		return false
	}
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}
