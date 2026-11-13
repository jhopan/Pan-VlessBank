package cf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateWebTunnelYML(t *testing.T) {
	dir := t.TempDir()
	credDir := filepath.Join(dir, "cred")
	SetDir(dir)
	t.Cleanup(func() { SetDir(".") })
	cred := filepath.Join(credDir, testTunnelID+".json")
	writeFile(t, cred, "{}")

	res := GenerateWebTunnelYML(testTunnelID, credDir, "example.com", "panel", 9000)
	if res["ok"] != true {
		t.Fatalf("GenerateWebTunnelYML: %v", res)
	}
	if res["hostname"] != "panel.example.com" || res["port"] != 9000 {
		t.Fatalf("result: %v", res)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "webui.tunnel.yml"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"tunnel: " + testTunnelID,
		"credentials-file: " + cred,
		"ingress:",
		"  - hostname: panel.example.com",
		"    service: http://127.0.0.1:9000",
		"  - service: http_status:404",
		"",
	}, "\n")
	if string(raw) != want {
		t.Fatalf("yml:\n%s\nwant:\n%s", raw, want)
	}
}
