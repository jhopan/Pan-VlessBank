package cf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testTunnelID = "11111111-2222-3333-4444-555555555555"

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateTunnelYML(t *testing.T) {
	dir := t.TempDir()
	credDir := filepath.Join(dir, "cred")
	SetDir(dir)
	t.Cleanup(func() { SetDir(".") })

	cred := filepath.Join(credDir, testTunnelID+".json")
	writeFile(t, cred, "{}")
	accounts := filepath.Join(dir, "accounts.txt")
	writeFile(t, accounts, "# catatan\nvless://aaa@host:443?type=tcp#one\n\nvless://bbb@host:443?type=tcp#two\n")
	settings := filepath.Join(dir, "webui.settings.json")
	writeFile(t, settings, `{"xray": "", "base": 1081, "base_http": 0, "listen": "127.0.0.1"}`)

	res := GenerateTunnelYML(testTunnelID, credDir, accounts, settings, "example.com", "socks")
	if res["ok"] != true {
		t.Fatalf("GenerateTunnelYML: %v", res)
	}
	if res["accounts"] != 2 {
		t.Fatalf("accounts: %v", res["accounts"])
	}
	if res["cred"] != cred || res["domain"] != "example.com" || res["subdomain"] != "socks" {
		t.Fatalf("payload: %v", res)
	}
	yml := filepath.Join(dir, "config.tunnel.yml")
	if res["path"] != yml {
		t.Fatalf("path: %v", res["path"])
	}

	raw, err := os.ReadFile(yml)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"tunnel: " + testTunnelID,
		"credentials-file: " + cred,
		"ingress:",
		"  - hostname: socks1.socks.example.com",
		"    service: tcp://localhost:1081",
		"  - hostname: http1.socks.example.com",
		"    service: tcp://localhost:2081",
		"  - hostname: socks2.socks.example.com",
		"    service: tcp://localhost:1082",
		"  - hostname: http2.socks.example.com",
		"    service: tcp://localhost:2082",
		"  - service: http_status:404",
		"",
	}, "\n")
	if string(raw) != want {
		t.Fatalf("yml:\n%s\nwant:\n%s", raw, want)
	}
}

func TestGenerateTunnelYMLExplicitBaseHTTP(t *testing.T) {
	dir := t.TempDir()
	credDir := filepath.Join(dir, "cred")
	SetDir(dir)
	t.Cleanup(func() { SetDir(".") })
	writeFile(t, filepath.Join(credDir, testTunnelID+".json"), "{}")
	accounts := filepath.Join(dir, "accounts.txt")
	writeFile(t, accounts, "vless://aaa@host:443?type=tcp#one\n")
	settings := filepath.Join(dir, "webui.settings.json")
	writeFile(t, settings, `{"base": 1200, "base_http": "3000"}`)

	if res := GenerateTunnelYML(testTunnelID, credDir, accounts, settings, "example.com", "vpn"); res["ok"] != true {
		t.Fatalf("GenerateTunnelYML: %v", res)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "config.tunnel.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{
		"  - hostname: socks1.vpn.example.com\n    service: tcp://localhost:1200\n",
		"  - hostname: http1.vpn.example.com\n    service: tcp://localhost:3000\n",
		"  - service: http_status:404\n",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("yml missing %q:\n%s", want, text)
		}
	}
	if strings.Count(text, "  - hostname: ") != 2 {
		t.Fatalf("ingress count:\n%s", text)
	}
}

func TestGenerateTunnelYMLMissingCred(t *testing.T) {
	dir := t.TempDir()
	SetDir(dir)
	t.Cleanup(func() { SetDir(".") })
	res := GenerateTunnelYML(testTunnelID, filepath.Join(dir, "cred"), filepath.Join(dir, "accounts.txt"),
		filepath.Join(dir, "webui.settings.json"), "example.com", "socks")
	want := "credentials tidak ada: " + filepath.Join(dir, "cred", testTunnelID+".json")
	if res["ok"] != false || res["error"] != want {
		t.Fatalf("missing cred: %v (want %q)", res, want)
	}
}

func TestGenerateTunnelYMLNoAccountsNoSettings(t *testing.T) {
	dir := t.TempDir()
	credDir := filepath.Join(dir, "cred")
	SetDir(dir)
	t.Cleanup(func() { SetDir(".") })
	writeFile(t, filepath.Join(credDir, testTunnelID+".json"), "{}")

	if res := GenerateTunnelYML(testTunnelID, credDir, filepath.Join(dir, "nope.txt"),
		filepath.Join(dir, "nope.json"), "example.com", "socks"); res["ok"] != true {
		t.Fatalf("GenerateTunnelYML: %v", res)
	}
	if res := GenerateTunnelYML(testTunnelID, credDir, filepath.Join(dir, "nope.txt"),
		filepath.Join(dir, "nope.json"), "example.com", "socks"); res["accounts"] != 0 {
		t.Fatalf("accounts: %v", res["accounts"])
	}
	raw, err := os.ReadFile(filepath.Join(dir, "config.tunnel.yml"))
	if err != nil {
		t.Fatal(err)
	}
	want := "tunnel: " + testTunnelID + "\ncredentials-file: " +
		filepath.Join(credDir, testTunnelID+".json") + "\ningress:\n  - service: http_status:404\n"
	if string(raw) != want {
		t.Fatalf("yml: %q want %q", raw, want)
	}
}

func TestCountAccounts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.txt")
	writeFile(t, path, "# one\n\n   \n  # indented comment still a comment\nvless://a@h:443#1\nvless://b@h:443#2\n\n")
	if got := countAccounts(path); got != 2 {
		t.Fatalf("countAccounts: %d", got)
	}
	if got := countAccounts(filepath.Join(dir, "missing.txt")); got != 0 {
		t.Fatalf("countAccounts missing: %d", got)
	}
}

func TestSettingsPorts(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		base int
		http int
	}{
		{"defaults", `{"base": 1081, "base_http": 0}`, 1081, 2081},
		{"base only", `{"base": 2000}`, 2000, 3000},
		{"explicit http", `{"base": 2000, "base_http": 2500}`, 2000, 2500},
		{"string values", `{"base": "1090", "base_http": "2090"}`, 1090, 2090},
		{"null http", `{"base": 1100, "base_http": null}`, 1100, 2100},
		{"null base falls back", `{"base": null, "base_http": 2090}`, 1081, 2081},
		{"broken base falls back", `{"base": "abc", "base_http": 2090}`, 1081, 2081},
		{"broken json", `not json`, 1081, 2081},
		{"empty object", `{}`, 1081, 2081},
	}
	for _, c := range cases {
		base, http := settingsPorts([]byte(c.raw))
		if base != c.base || http != c.http {
			t.Fatalf("%s: got (%d,%d) want (%d,%d)", c.name, base, http, c.base, c.http)
		}
	}
}

func TestYMLPreview(t *testing.T) {
	dir := t.TempDir()
	SetDir(dir)
	t.Cleanup(func() { SetDir(".") })

	res := YMLPreview()
	if res["ok"] != false || res["error"] != "config.tunnel.yml belum ada" {
		t.Fatalf("preview missing: %v", res)
	}
	writeFile(t, filepath.Join(dir, "config.tunnel.yml"), "tunnel: tid\n")
	res = YMLPreview()
	if res["ok"] != true || res["content"] != "tunnel: tid\n" {
		t.Fatalf("preview: %v", res)
	}
}
