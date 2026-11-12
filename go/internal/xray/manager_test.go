package xray

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"
)

// The test binary doubles as the xray stub: spawned with VB_XRAY_STUB set it
// sleeps (or exits) instead of running the test suite.
func TestMain(m *testing.M) {
	switch os.Getenv("VB_XRAY_STUB") {
	case "sleep":
		time.Sleep(60 * time.Second)
		os.Exit(0)
	case "exit3":
		os.Exit(3)
	}
	os.Exit(m.Run())
}

// stub copies the test binary under a unique name so that PIDs() only ever
// matches our own stub and never the running test process.
func stub(t *testing.T) string {
	t.Helper()
	src, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "xraystub.exe")
	if err := os.WriteFile(dst, raw, 0o755); err != nil {
		t.Fatal(err)
	}
	return dst
}

func newManager(t *testing.T, bin string) *Manager {
	t.Helper()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfg, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(cfg, filepath.Join(dir, "xray.log"), bin, func() string { return bin })
	t.Cleanup(func() { m.Stop() })
	return m
}

var startRe = regexp.MustCompile(`^xray start \(pid (\d+)\)$`)

func pidOf(t *testing.T, msg string) int {
	t.Helper()
	m := startRe.FindStringSubmatch(msg)
	if m == nil {
		t.Fatalf("unexpected message: %q", msg)
	}
	pid, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return pid
}

func spawnStub(t *testing.T, bin string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "VB_XRAY_STUB=sleep")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
	return cmd
}

func TestStartStopCycle(t *testing.T) {
	t.Setenv("VB_XRAY_STUB", "sleep")
	bin := stub(t)
	m := newManager(t, bin)

	msg := m.Start()
	pid := pidOf(t, msg)
	defer m.Stop()

	if !m.Running() {
		t.Fatalf("Running() false after %q", msg)
	}
	if got := m.Start(); got != fmt.Sprintf("xray sudah jalan (pid %d)", pid) {
		t.Fatalf("second Start: %q", got)
	}
	if got := m.Stop(); got != fmt.Sprintf("xray stop (1 proses, pid %d)", pid) {
		t.Fatalf("Stop: %q", got)
	}
	if m.Running() {
		t.Fatal("Running() true after Stop")
	}
	if got := m.Stop(); got != "xray memang mati" {
		t.Fatalf("second Stop: %q", got)
	}
}

func TestStartMissingConfig(t *testing.T) {
	bin := stub(t)
	m := New(filepath.Join(t.TempDir(), "config.json"), filepath.Join(t.TempDir(), "xray.log"), bin, nil)
	if got := m.Start(); got != "config.json belum ada - klik Regenerate dulu" {
		t.Fatalf("Start: %q", got)
	}
}

func TestStartMissingBinary(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfg, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "nope", "xray-nostub-xyz.exe")
	m := New(cfg, filepath.Join(dir, "xray.log"), bin, nil)
	want := "xray binary tidak ketemu: " + bin + " (atur di Pengaturan)"
	if got := m.Start(); got != want {
		t.Fatalf("Start: %q (want %q)", got, want)
	}
	if m.Running() {
		t.Fatal("Running() true without binary")
	}
}

func TestStartExitsImmediately(t *testing.T) {
	t.Setenv("VB_XRAY_STUB", "exit3")
	m := newManager(t, stub(t))
	if got := m.Start(); got != "xray gagal start (exit 3) - cek Log" {
		t.Fatalf("Start: %q", got)
	}
	if m.Running() {
		t.Fatal("Running() true after failed start")
	}
}

func TestStartDetectsStray(t *testing.T) {
	t.Setenv("VB_XRAY_STUB", "sleep")
	bin := stub(t)
	m := newManager(t, bin)
	stray := spawnStub(t, bin)

	pid := stray.Process.Pid
	waitFor(t, func() bool { return containsInt(m.PIDs(), pid) })

	msg := m.Start()
	if msg != fmt.Sprintf("xray sudah jalan (pid %d) - klik Stop dulu kalau mau start ulang", pid) {
		t.Fatalf("Start with stray: %q", msg)
	}
	if !m.Running() {
		t.Fatal("Running() false with stray")
	}
	if got := m.Stop(); got != fmt.Sprintf("xray stop (1 proses, pid %d)", pid) {
		t.Fatalf("Stop stray: %q", got)
	}
	waitFor(t, func() bool { return !m.Running() })
}

func TestPIDsPortable(t *testing.T) {
	t.Setenv("VB_XRAY_STUB", "sleep")
	bin := stub(t)
	m := newManager(t, bin)

	if pids := m.PIDs(); len(pids) != 0 {
		t.Fatalf("expected no stray, got %v", pids)
	}
	stray := spawnStub(t, bin)
	pid := stray.Process.Pid

	// The scan may not see the process instantly (and on some platforms the
	// scan source is unavailable), so tolerate absence but never a wrong pid.
	found := false
	for range 20 {
		pids := m.PIDs()
		for _, p := range pids {
			if p <= 0 {
				t.Fatalf("bad pid in %v", pids)
			}
		}
		if containsInt(pids, pid) {
			found = true
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !found {
		t.Logf("pid %d not visible through the portable scan (tolerated)", pid)
		return
	}
	if err := stray.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	stray.Wait()
	waitFor(t, func() bool { return !containsInt(m.PIDs(), pid) })
}

func TestTailLog(t *testing.T) {
	dir := t.TempDir()
	logp := filepath.Join(dir, "xray.log")
	m := New(filepath.Join(dir, "config.json"), logp, "xray", nil)

	if got := m.TailLog(10); got != "log kosong" {
		t.Fatalf("missing log: %q", got)
	}
	if err := os.WriteFile(logp, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := m.TailLog(2); got != "b\nc\n" {
		t.Fatalf("tail 2: %q", got)
	}
	if got := m.TailLog(50); got != "a\nb\nc\n" {
		t.Fatalf("tail 50: %q", got)
	}
	if err := os.WriteFile(logp, []byte("x\ny"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := m.TailLog(1); got != "y" {
		t.Fatalf("tail last line: %q", got)
	}
	if got := m.TailLog(0); got != "" {
		t.Fatalf("tail 0: %q", got)
	}
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}
