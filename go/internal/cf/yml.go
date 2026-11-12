package cf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateTunnelYML writes config.tunnel.yml for tunnelID (cf.py
// generate_tunnel_yml): accounts are counted from the non-comment lines of
// accounts.txt and the ports come from webui.settings.json.
func GenerateTunnelYML(tunnelID, credDir, accountsPath, settingsPath, domain, sub string) map[string]any {
	cred := filepath.Join(credDir, tunnelID+".json")
	if !fileExists(cred) {
		return map[string]any{"ok": false, "error": "credentials tidak ada: " + cred}
	}

	nAcc := countAccounts(accountsPath)

	base, baseHTTP := 1081, 2081
	if raw, err := os.ReadFile(settingsPath); err == nil {
		base, baseHTTP = settingsPorts(raw)
	}

	lines := []string{
		"tunnel: " + tunnelID,
		"credentials-file: " + cred,
		"ingress:",
	}
	prefix := ""
	if sub != "" {
		prefix = sub + "."
	}
	for i := range nAcc {
		n := i + 1
		lines = append(lines,
			fmt.Sprintf("  - hostname: socks%d.%s%s", n, prefix, domain),
			fmt.Sprintf("    service: tcp://localhost:%d", base+i),
			fmt.Sprintf("  - hostname: http%d.%s%s", n, prefix, domain),
			fmt.Sprintf("    service: tcp://localhost:%d", baseHTTP+i),
		)
	}
	lines = append(lines, "  - service: http_status:404")

	yml := filepath.Join(Dir(), "config.tunnel.yml")
	if err := os.WriteFile(yml, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	return map[string]any{
		"ok":        true,
		"path":      yml,
		"accounts":  nAcc,
		"cred":      cred,
		"domain":    domain,
		"subdomain": sub,
	}
}

// YMLPreview returns the content of config.tunnel.yml (cf.py yml_preview).
func YMLPreview() map[string]any {
	yml := filepath.Join(Dir(), "config.tunnel.yml")
	if !fileExists(yml) {
		return map[string]any{"ok": false, "error": "config.tunnel.yml belum ada"}
	}
	b, err := os.ReadFile(yml)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	return map[string]any{"ok": true, "content": strings.ReplaceAll(string(b), "\r\n", "\n")}
}

func countAccounts(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			n++
		}
	}
	return n
}

// settingsPorts mirrors the settings block of cf.py generate_tunnel_yml:
// base_http 0/absent means base+1000, and any unusable value falls back to the
// defaults for both ports.
func settingsPorts(raw []byte) (int, int) {
	var s map[string]any
	if err := json.Unmarshal(raw, &s); err != nil {
		return 1081, 2081
	}
	base := 1081
	if v, ok := s["base"]; ok {
		n, err := toInt(v)
		if err != nil {
			return 1081, 2081
		}
		base = n
	}
	bh := 0
	if v, ok := s["base_http"]; ok && v != nil {
		n, err := toInt(v)
		if err != nil {
			return 1081, 2081
		}
		bh = n
	}
	if bh != 0 {
		return base, bh
	}
	return base, base + 1000
}
