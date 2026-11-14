package ops

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestProxyTunnelValidationDetectsCollapsedPorts(t *testing.T) {
	got := ValidateProxyTunnel([]byte("ingress:\n- hostname: socks1.example.com\n  service: tcp://127.0.0.1:1081\n- hostname: socks2.example.com\n  service: tcp://127.0.0.1:1081\n"))
	if got.OK || len(got.Errors) == 0 {
		t.Fatalf("collapsed ingress accepted: %#v", got)
	}
}
func TestProxyTunnelValidationAcceptsDistinctPairs(t *testing.T) {
	got := ValidateProxyTunnel([]byte("ingress:\n- hostname: socks1.example.com\n  service: tcp://127.0.0.1:1081\n- hostname: http1.example.com\n  service: tcp://127.0.0.1:2081\n- hostname: socks2.example.com\n  service: tcp://127.0.0.1:1082\n- hostname: http2.example.com\n  service: tcp://127.0.0.1:2082\n"))
	if !got.OK {
		t.Fatalf("valid ingress rejected: %#v", got)
	}
}
func TestBackupRedactsSecrets(t *testing.T) {
	dir := t.TempDir()
	for n, v := range map[string]string{"webui.settings.json": "{\"auth\":\"user:secret\"}", "accounts.txt": "vless://secret@host:443", "webui.client-tokens.json": "secret", "webui.login": "secret"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(v), 0600); err != nil {
			t.Fatal(err)
		}
	}
	b, err := RedactedBackup(dir, 3)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("secret")) || bytes.Contains(b, []byte("vless://")) {
		t.Fatal("secret leaked")
	}
}
func TestAuditOmitsRawSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	if err := AppendAudit(path, "token.create", "created token secret"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("secret")) {
		t.Fatal("secret logged")
	}
}
