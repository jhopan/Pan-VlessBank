// Package diag: `vlessbank check` — cek tiap akun: IP keluar harus sama dengan
// IP di path WS (`/<ip>-<port>`). Pengganti Go untuk check-accounts.py.
package diag

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"vlessbank/internal/config"
	"vlessbank/internal/netcheck"
)

var ipInPath = regexp.MustCompile(`^/(\d{1,3}(?:\.\d{1,3}){3})-\d+`)

type row struct {
	Idx   int
	Port  int
	Name  string
	Want  string
	Have  string
	State string
}

// Run builds a probe config, starts xray, probes every account and prints the
// table. Return value = exit code (1 kalau ada MISMATCH/FAIL).
func Run(accountsPath, xrayBin string, base int, timeout time.Duration, workers int, out io.Writer) int {
	accs, _, err := config.ParseFile(accountsPath)
	if err != nil {
		fmt.Fprintf(out, "ERROR: %s tidak ada\n", accountsPath)
		return 2
	}
	if len(accs) == 0 {
		fmt.Fprintln(out, "ERROR: tidak ada akun valid")
		return 2
	}
	cfg, binds := config.BuildConfig(accs, "127.0.0.1", base, base+1000, "", false, 0)

	rows := make([]*row, 0, len(binds))
	for i, b := range binds {
		acc := accs[i]
		want := "-"
		if m := ipInPath.FindStringSubmatch(pathOf(acc.Raw)); m != nil {
			want = m[1]
		}
		rows = append(rows, &row{Idx: i + 1, Port: b.SocksPort, Name: acc.Name, Want: want})
	}

	tmp, err := os.CreateTemp("", "vlessbank-check-*.json")
	if err != nil {
		fmt.Fprintln(out, "ERROR:", err)
		return 2
	}
	name := tmp.Name()
	tmp.Close()
	defer os.Remove(name)
	if err := config.WriteJSON(name, cfg); err != nil {
		fmt.Fprintln(out, "ERROR:", err)
		return 2
	}

	logName := name + ".log"
	cmd := exec.Command(xrayBin, "run", "-c", name)
	cmd.Stdout = io.Discard
	if lf, err := os.Create(logName); err == nil {
		cmd.Stderr = lf
		defer func() { lf.Close(); os.Remove(logName) }()
	} else {
		cmd.Stderr = io.Discard
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(out, "ERROR: xray tidak bisa dijalankan (%s): %v\n", xrayBin, err)
		return 2
	}
	defer killTree(cmd)

	if !waitPort(base, 10*time.Second) {
		fmt.Fprintln(out, "ERROR: xray gagal start / port tidak listen")
		if b, err := os.ReadFile(logName); err == nil && len(b) > 0 {
			fmt.Fprintln(out, "--- xray stderr ---")
			fmt.Fprintln(out, tail(string(b), 6))
		}
		fmt.Fprintln(out, "hint: pakai --base lain kalau port sudah dipakai proses xray yang lain")
		return 2
	}

	if workers < 1 {
		workers = 6
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for _, r := range rows {
		wg.Add(1)
		sem <- struct{}{}
		go func(r *row) {
			defer wg.Done()
			defer func() { <-sem }()
			r.Have = probe(r.Port, timeout)
		}(r)
	}
	wg.Wait()

	fmt.Fprintf(out, "%3s %5s  %-16s %-16s %-9s %s\n", "#", "port", "path-ip", "egress", "status", "nama")
	bad, ok, nopath := 0, 0, 0
	for _, r := range rows {
		switch {
		case r.Have == "":
			r.State = "FAIL"
			bad++
		case r.Want == "-":
			r.State = "NOPATH"
			nopath++
		case r.Have == r.Want:
			r.State = "OK"
			ok++
		default:
			r.State = "MISMATCH"
			bad++
		}
		have := r.Have
		if have == "" {
			have = "-"
		}
		fmt.Fprintf(out, "%3d %5d  %-16s %-16s %-9s %s\n", r.Idx, r.Port, r.Want, have, r.State, r.Name)
	}
	fmt.Fprintf(out, "\n%d akun | OK %d | MISMATCH/FAIL %d | tanpa ip-di-path %d\n", len(rows), ok, bad, nopath)
	if bad > 0 {
		return 1
	}
	return 0
}

// pathOf mengambil parameter path dari link vless (sama seperti yang ditulis ke wsSettings).
func pathOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return u.Query().Get("path")
}

func probe(port int, timeout time.Duration) string {
	conn, err := netcheck.DialSOCKS5(net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		"checkip.amazonaws.com:80", timeout)
	if err != nil {
		return ""
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))
	fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: checkip.amazonaws.com\r\nConnection: close\r\n\r\n")
	body, _ := io.ReadAll(io.LimitReader(conn, 4096))
	text := string(body)
	if i := strings.Index(text, "\r\n\r\n"); i >= 0 {
		text = text[i+4:]
	}
	ip := strings.TrimSpace(text)
	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}

func waitPort(port int, t time.Duration) bool {
	end := time.Now().Add(t)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	for time.Now().Before(end) {
		c, err := net.DialTimeout("tcp", addr, 400*time.Millisecond)
		if err == nil {
			c.Close()
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

// tail mengambil n baris terakhir (untuk menampilkan error xray).
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\r\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// killTree mematikan xray beserta anak prosesnya.
func killTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
	} else {
		_ = cmd.Process.Kill()
	}
}
