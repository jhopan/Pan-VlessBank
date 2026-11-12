package cf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The test binary doubles as the cloudflared stub: spawned with VB_CF_STUB set
// it answers the cloudflared subcommand it was given instead of running the
// test suite. The variable's value selects the variant under test.
func TestMain(m *testing.M) {
	if mode := os.Getenv("VB_CF_STUB"); mode != "" {
		stubMain(mode)
	}
	os.Exit(m.Run())
}

func stubMain(mode string) {
	args := strings.Join(os.Args[1:], " ")
	switch {
	case strings.Contains(args, "--version"):
		fmt.Println("cloudflared version 2026.1.0 (built 2026-01-01)")
	case strings.Contains(args, "tunnel list"):
		if mode == "listbroken" {
			fmt.Println("not json at all")
			break
		}
		fmt.Println(`[{"id":"tid-1","name":"tun1","created_at":"2026-01-01T00:00:00Z"},` +
			`{"id":"tid-2","name":"tun2","created_at":"2026-01-02T00:00:00Z"}]`)
	case strings.Contains(args, "tunnel create"):
		if mode == "createuuid" {
			fmt.Println("Created tunnel mytun with id 11111111-2222-3333-4444-555555555555")
			break
		}
		fmt.Println("Tunnel created, credentials written to /tmp/tun.json")
	case strings.Contains(args, "tunnel delete"):
		if mode == "deletefail" {
			fmt.Fprintln(os.Stderr, "tunnel not found")
			os.Exit(1)
		}
	case strings.Contains(args, "tunnel login"):
		fmt.Println("https://dash.cloudflare.com/argotunnel?aud=&callback=https%3A%2F%2Flogin.cloudflareaccess.org%2Ftok")
		if mode == "login-done" {
			fmt.Println("You have successfully logged in.")
			fmt.Println("You can now use cloudflared with the credentials in /tmp/cert.pem")
		} else {
			fmt.Println("Please open the following URL and log in with your Cloudflare account:")
		}
		time.Sleep(60 * time.Second)
	case strings.Contains(args, "run"):
		time.Sleep(60 * time.Second)
	}
	os.Exit(0)
}

// setup puts a runnable stub in a temp dir as the bundled cloudflared and makes
// PATH point there too, so binary resolution is deterministic.
func setup(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	src, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cloudflared"+exeSuffix()), raw, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("VB_CF_STUB", mode)
	SetDir(dir)
	t.Cleanup(func() { SetDir(".") })
	return dir
}

// setupNoBin uses a dir without cloudflared and a PATH that cannot find one.
func setupNoBin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	SetDir(dir)
	t.Cleanup(func() { SetDir(".") })
	return dir
}

func stubBin() string {
	return filepath.Join(Dir(), "cloudflared"+exeSuffix())
}

func TestFindBin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	if got := FindBin(dir); got != "" {
		t.Fatalf("FindBin on empty dir: %q", got)
	}
	bundle := filepath.Join(dir, "cloudflared"+exeSuffix())
	if err := os.WriteFile(bundle, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := FindBin(dir); got != bundle {
		t.Fatalf("FindBin: %q (want %q)", got, bundle)
	}
}

func TestPlatformInfoNoBin(t *testing.T) {
	dir := setupNoBin(t)
	info := PlatformInfo()
	if info["bin_found"] != false {
		t.Fatalf("bin_found: %v", info["bin_found"])
	}
	if info["bin"] != "(belum ditemukan)" {
		t.Fatalf("bin: %v", info["bin"])
	}
	if info["bundle"] != filepath.Join(dir, "cloudflared"+exeSuffix()) {
		t.Fatalf("bundle: %v", info["bundle"])
	}
	for _, k := range []string{"os", "arch"} {
		if s, _ := info[k].(string); s == "" {
			t.Fatalf("%s empty: %v", k, info[k])
		}
	}
	if name, err := BundleName(); err != nil || name == "" {
		t.Fatalf("BundleName: %q %v", name, err)
	}
	if DownloadURL() == "" || !strings.HasPrefix(DownloadURL(), CFCDN) {
		t.Fatalf("DownloadURL: %q", DownloadURL())
	}
}

func TestDownloadStatusStates(t *testing.T) {
	setupNoBin(t)
	mu.Lock()
	dlProgress, dlError = -1, ""
	mu.Unlock()

	want := map[string]any{"state": "missing", "progress": 0, "error": "", "version": ""}
	if got := DownloadStatus(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("missing: %v (want %v)", got, want)
	}

	mu.Lock()
	dlProgress, dlError = 42, ""
	mu.Unlock()
	if got := DownloadStatus(); got["state"] != "downloading" || got["progress"] != 42 {
		t.Fatalf("downloading: %v", got)
	}

	mu.Lock()
	dlProgress, dlError = -2, "boo"
	mu.Unlock()
	if got := DownloadStatus(); got["state"] != "error" || got["error"] != "boo" {
		t.Fatalf("error: %v", got)
	}

	mu.Lock()
	dlProgress, dlError = -1, ""
	mu.Unlock()
}

func TestVersionAndBinOK(t *testing.T) {
	setup(t, "plain")
	if !BinOK() {
		t.Fatal("BinOK false with bundle present")
	}
	if got := Version(); got != "cloudflared version 2026.1.0 (built 2026-01-01)" {
		t.Fatalf("Version: %q", got)
	}
}

func TestLoginScrapeAndCancel(t *testing.T) {
	setup(t, "login-done")

	res := StartLogin()
	if res["ok"] != true {
		t.Fatalf("StartLogin: %v", res)
	}
	const want = "https://dash.cloudflare.com/argotunnel?aud=&callback=https%3A%2F%2Flogin.cloudflareaccess.org%2Ftok"
	if url, _ := res["url"].(string); url != want {
		t.Fatalf("login url: %v", res["url"])
	}
	if st := LoginStatus(); st["running"] != true {
		t.Fatalf("login not running: %v", st)
	}
	done := false
	for range 20 {
		if LoginStatus()["done"] == true {
			done = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !done {
		t.Fatalf("login done not scraped: %v", LoginStatus())
	}
	if got := CancelLogin(); got["ok"] != true {
		t.Fatalf("CancelLogin: %v", got)
	}
	st := LoginStatus()
	if st["running"] != false || st["url"] != "" || st["done"] != false {
		t.Fatalf("state after cancel: %v", st)
	}
}

func TestListTunnels(t *testing.T) {
	setup(t, "plain")
	got := ListTunnels()
	if len(got) != 2 || got[0]["id"] != "tid-1" || got[1]["name"] != "tun2" {
		t.Fatalf("ListTunnels: %v", got)
	}
	if got[0]["created"] != "2026-01-01T00:00:00Z" {
		t.Fatalf("created: %v", got[0])
	}

	t.Setenv("VB_CF_STUB", "listbroken")
	if got := ListTunnels(); got == nil || len(got) != 0 {
		t.Fatalf("broken json must yield empty non-nil slice: %#v", got)
	}
}

func TestCreateDeleteTunnel(t *testing.T) {
	setup(t, "createuuid")
	if got := CreateTunnel(" mytun "); got["ok"] != true ||
		got["id"] != "11111111-2222-3333-4444-555555555555" || got["name"] != "mytun" {
		t.Fatalf("CreateTunnel ok: %v", got)
	}

	t.Setenv("VB_CF_STUB", "plain")
	got := CreateTunnel("tun")
	if got["ok"] != false || got["error"] != "Tunnel created, credentials written to /tmp/tun.json" {
		t.Fatalf("CreateTunnel no id: %v", got)
	}
	if got := CreateTunnel("   "); got["ok"] != false || got["error"] != "nama tunnel kosong" {
		t.Fatalf("CreateTunnel empty name: %v", got)
	}
	if got := DeleteTunnel("tid-1"); got["ok"] != true {
		t.Fatalf("DeleteTunnel ok: %v", got)
	}

	t.Setenv("VB_CF_STUB", "deletefail")
	if got := DeleteTunnel("tid-1"); got["ok"] != false || got["error"] != "tunnel not found" {
		t.Fatalf("DeleteTunnel fail: %v", got)
	}
}

func TestCreateDeleteNoBin(t *testing.T) {
	setupNoBin(t)
	if got := CreateTunnel("tun"); got["ok"] != false || got["error"] != "cloudflared belum ada" {
		t.Fatalf("CreateTunnel no bin: %v", got)
	}
	if got := DeleteTunnel("tid"); got["ok"] != false || got["error"] != "cloudflared belum ada" {
		t.Fatalf("DeleteTunnel no bin: %v", got)
	}
	if got := StartLogin(); got["ok"] != false || got["error"] != "cloudflared belum ada" {
		t.Fatalf("StartLogin no bin: %v", got)
	}
	if got := Start("/nope/config.tunnel.yml"); got != "cloudflared belum ada" {
		t.Fatalf("Start no bin: %q", got)
	}
	if got := ListTunnels(); got == nil || len(got) != 0 {
		t.Fatalf("ListTunnels no bin: %#v", got)
	}
	if got := Version(); got != "" {
		t.Fatalf("Version no bin: %q", got)
	}
}

func TestTunnelLifecycle(t *testing.T) {
	dir := setup(t, "plain")

	yml := filepath.Join(dir, "config.tunnel.yml")
	if err := os.WriteFile(yml, []byte("tunnel: tid\ningress:\n  - service: http_status:404\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Start(filepath.Join(dir, "missing.yml")); got != "config.tunnel.yml tidak ada" {
		t.Fatalf("Start missing yml: %q", got)
	}
	msg := Start(yml)
	if !strings.HasPrefix(msg, "tunnel start (pid ") || !strings.HasSuffix(msg, ")") {
		t.Fatalf("Start: %q", msg)
	}
	defer Stop()
	if !Running() {
		t.Fatal("Running() false after Start")
	}
	if got := Start(yml); got != "tunnel sudah jalan" {
		t.Fatalf("second Start: %q", got)
	}
	if got := Stop(); got != "tunnel stop" {
		t.Fatalf("Stop: %q", got)
	}
	if Running() {
		t.Fatal("Running() true after Stop")
	}
	if got := Stop(); got != "tunnel memang mati" {
		t.Fatalf("second Stop: %q", got)
	}
	if got := Start(yml); !strings.HasPrefix(got, "tunnel start (pid ") {
		t.Fatalf("restart after Stop: %q", got)
	}

	st := Status()
	if st["running"] != true || st["bin_ok"] != true || st["yml_ok"] != true || st["yml_path"] != yml {
		t.Fatalf("Status: %v", st)
	}
	if tun, ok := st["tunnels"].([]map[string]any); !ok || len(tun) != 2 {
		t.Fatalf("Status tunnels: %#v", st["tunnels"])
	}
	if st["version"] != "cloudflared version 2026.1.0 (built 2026-01-01)" {
		t.Fatalf("version: %v", st["version"])
	}
	if _, err := os.Stat(filepath.Join(dir, "cloudflared.log")); err != nil {
		t.Fatalf("log file not created by Start: %v", err)
	}
	if st["log_tail"] != "" {
		t.Fatalf("log_tail for silent stub: %q", st["log_tail"])
	}
}

func TestTailLog(t *testing.T) {
	dir := setupNoBin(t)
	if got := TailLog(10); got != "" {
		t.Fatalf("missing log: %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "cloudflared.log"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := TailLog(2); got != "two\nthree\n" {
		t.Fatalf("tail: %q", got)
	}
}

func TestRunCombinedCapturesStderr(t *testing.T) {
	setup(t, "deletefail")
	out := runCombined(stubBin(), 10*time.Second, "tunnel", "delete", "-f", "tid")
	if out != "tunnel not found\n" {
		t.Fatalf("runCombined: %q", out)
	}
}
