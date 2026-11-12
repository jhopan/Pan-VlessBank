package cf

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	uuidRe = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	urlRe  = regexp.MustCompile(`https://\S+`)
)

// StartLogin runs a fresh "cloudflared tunnel login" and scrapes the URL
// (cf.py start_login: never auto-detects an old cert).
func StartLogin() map[string]any {
	b := resolvedBin()
	if b == "" {
		return map[string]any{"ok": false, "error": "cloudflared belum ada"}
	}
	mu.Lock()
	if procLogin.alive() {
		res := map[string]any{"ok": true, "url": loginURL, "done": loginDone}
		mu.Unlock()
		return res
	}
	loginURL = ""
	loginDone = false
	mu.Unlock()

	pr, pw, err := os.Pipe()
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	cmd := exec.Command(b, "tunnel", "login")
	cmd.Stdout = pw
	cmd.Stderr = pw
	c, err := startChild(cmd)
	pw.Close()
	if err != nil {
		pr.Close()
		return map[string]any{"ok": false, "error": err.Error()}
	}
	mu.Lock()
	procLogin = c
	mu.Unlock()
	go readLogin(pr)

	for range 16 {
		time.Sleep(500 * time.Millisecond)
		mu.Lock()
		u := loginURL
		mu.Unlock()
		if u != "" {
			break
		}
	}
	mu.Lock()
	defer mu.Unlock()
	return map[string]any{"ok": true, "url": loginURL, "done": loginDone}
}

func readLogin(pr *os.File) {
	defer pr.Close()
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		mu.Lock()
		if loginURL == "" {
			if m := urlRe.FindString(line); m != "" {
				loginURL = m
			}
		}
		if strings.Contains(line, "cert.pem") ||
			strings.Contains(line, "You have successfully") ||
			strings.Contains(line, "logged in") {
			loginDone = true
		}
		mu.Unlock()
	}
}

// LoginStatus mirrors cf.py login_status, plus "cert": true kalau sudah pernah
// `cloudflared tunnel login` (cert.pem ada) — panel tidak perlu login ulang.
func LoginStatus() map[string]any {
	mu.Lock()
	c := procLogin
	url, done := loginURL, loginDone
	mu.Unlock()
	return map[string]any{"running": c.alive(), "url": url, "done": done, "cert": HasCert()}
}

// CertPath mengembalikan lokasi cert.pem cloudflared.
func CertPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "cert.pem")
	}
	return filepath.Join(home, ".cloudflared", "cert.pem")
}

// HasCert melaporkan apakah kredensial akun Cloudflare sudah ada di cert.pem.
func HasCert() bool {
	st, err := os.Stat(CertPath())
	return err == nil && !st.IsDir()
}

// CancelLogin mirrors cf.py cancel_login.
func CancelLogin() map[string]any {
	mu.Lock()
	c := procLogin
	mu.Unlock()
	if c.alive() {
		terminate(c.cmd.Process)
		if !waitClosed(c.done, 3*time.Second) {
			c.cmd.Process.Kill()
		}
	}
	mu.Lock()
	procLogin = nil
	loginURL = ""
	loginDone = false
	mu.Unlock()
	return map[string]any{"ok": true}
}

// ListTunnels mirrors cf.py list_tunnels (empty slice, never nil).
func ListTunnels() []map[string]any {
	b := resolvedBin()
	if b == "" {
		return []map[string]any{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, b, "tunnel", "list", "--output", "json")
	out, _ := cmd.Output()
	var data []map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		return []map[string]any{}
	}
	res := []map[string]any{}
	for _, t := range data {
		res = append(res, map[string]any{
			"id":      jsonStr(t["id"]),
			"name":    jsonStr(t["name"]),
			"created": jsonStr(t["created_at"]),
		})
	}
	return res
}

// CreateTunnel mirrors cf.py create_tunnel.
func CreateTunnel(name string) map[string]any {
	b := resolvedBin()
	if b == "" {
		return map[string]any{"ok": false, "error": "cloudflared belum ada"}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return map[string]any{"ok": false, "error": "nama tunnel kosong"}
	}
	out := strings.TrimSpace(runCombined(b, 30*time.Second, "tunnel", "create", name))
	if m := uuidRe.FindString(out); m != "" {
		return map[string]any{"ok": true, "id": m, "name": name}
	}
	if out == "" {
		out = "UUID tidak ditemukan"
	}
	return map[string]any{"ok": false, "error": out}
}

// DeleteTunnel mirrors cf.py delete_tunnel.
func DeleteTunnel(id string) map[string]any {
	b := resolvedBin()
	if b == "" {
		return map[string]any{"ok": false, "error": "cloudflared belum ada"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, b, "tunnel", "delete", "-f", id)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err == nil {
		return map[string]any{"ok": true}
	}
	return map[string]any{"ok": false, "error": strings.TrimSpace(out.String() + errb.String())}
}

// Running reports whether the tunnel child started by Start is alive
// (cf.py tunnel_running).
func Running() bool {
	mu.Lock()
	c := procTunnel
	mu.Unlock()
	return c.alive()
}

// TunnelRunning is the panel-facing name of Running.
func TunnelRunning() bool {
	return Running()
}

// Start launches "cloudflared tunnel --config <configYML> run"
// (cf.py tunnel_start).
func Start(configYML string) string {
	b := resolvedBin()
	if b == "" {
		return "cloudflared belum ada"
	}
	if !fileExists(configYML) {
		return "config.tunnel.yml tidak ada"
	}
	if Running() {
		return "tunnel sudah jalan"
	}
	log, err := os.OpenFile(filepath.Join(Dir(), "cloudflared.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	cmd := exec.Command(b, "tunnel", "--config", configYML, "run")
	cmd.Stdout = log
	cmd.Stderr = log
	c, err := startChild(cmd)
	log.Close()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	mu.Lock()
	procTunnel = c
	mu.Unlock()
	return fmt.Sprintf("tunnel start (pid %d)", c.pid())
}

// Stop mirrors cf.py tunnel_stop.
func Stop() string {
	mu.Lock()
	c := procTunnel
	mu.Unlock()
	if !c.alive() {
		return "tunnel memang mati"
	}
	terminate(c.cmd.Process)
	if !waitClosed(c.done, 5*time.Second) {
		c.cmd.Process.Kill()
	}
	mu.Lock()
	procTunnel = nil
	mu.Unlock()
	return "tunnel stop"
}

// Status mirrors cf.py tunnel_status.
func Status() map[string]any {
	b := resolvedBin()
	yml := filepath.Join(Dir(), "config.tunnel.yml")
	return map[string]any{
		"running":  Running(),
		"bin_ok":   b != "",
		"version":  Version(),
		"tunnels":  ListTunnels(),
		"log_tail": TailLog(40),
		"yml_ok":   fileExists(yml),
		"yml_path": yml,
	}
}

// runCombined runs a command and returns stdout + stderr, like Python's
// subprocess.run(..., capture_output=True) with r.stdout + r.stderr.
func runCombined(bin string, timeout time.Duration, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	_ = cmd.Run()
	return out.String() + errb.String()
}
