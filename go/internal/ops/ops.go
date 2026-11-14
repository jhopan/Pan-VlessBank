package ops

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type TunnelValidation struct {
	OK      bool     `json:"ok"`
	Entries int      `json:"entries"`
	Errors  []string `json:"errors"`
}
type ingress struct {
	host string
	port int
}

func ValidateProxyTunnel(raw []byte) TunnelValidation {
	lines := strings.Split(string(raw), "\n")
	var in []ingress
	host := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- hostname:") {
			host = strings.TrimSpace(strings.TrimPrefix(line, "- hostname:"))
			continue
		}
		if strings.HasPrefix(line, "service: tcp://") && host != "" {
			addr := strings.TrimPrefix(line, "service: tcp://")
			_, p, e := net.SplitHostPort(addr)
			if e != nil {
				continue
			}
			n, e := strconv.Atoi(p)
			if e == nil {
				in = append(in, ingress{host, n})
			}
			host = ""
		}
	}
	out := TunnelValidation{OK: true, Entries: len(in), Errors: []string{}}
	seenPort := map[int]string{}
	seenHost := map[string]bool{}
	for _, v := range in {
		if seenHost[v.host] {
			out.Errors = append(out.Errors, "hostname duplikat: "+v.host)
		}
		seenHost[v.host] = true
		if prior := seenPort[v.port]; prior != "" {
			out.Errors = append(out.Errors, fmt.Sprintf("port %d dipakai %s dan %s", v.port, prior, v.host))
		}
		seenPort[v.port] = v.host
	}
	if len(in) == 0 {
		out.Errors = append(out.Errors, "ingress proxy tidak ditemukan")
	}
	out.OK = len(out.Errors) == 0
	return out
}

type AuditEvent struct {
	Time   string `json:"time"`
	Actor  string `json:"actor"`
	Action string `json:"action"`
	Result string `json:"result"`
}

func AppendAudit(path, action, summary string) error {
	return AppendAuditResult(path, "admin", action, "ok")
}
func AppendAuditResult(path, actor, action, result string) error {
	rec := AuditEvent{Time: time.Now().UTC().Format(time.RFC3339), Actor: actor, Action: action, Result: result}
	b, e := json.Marshal(rec)
	if e != nil {
		return e
	}
	f, e := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = f.Write(append(b, '\n'))
	return e
}
func TailAudit(path string, n int) []AuditEvent {
	b, e := os.ReadFile(path)
	if e != nil {
		return []AuditEvent{}
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := make([]AuditEvent, 0, len(lines))
	for _, line := range lines {
		var r AuditEvent
		if json.Unmarshal([]byte(line), &r) == nil {
			out = append(out, r)
		}
	}
	return out
}
func RedactedBackup(dir string, accounts int) ([]byte, error) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	w, e := z.Create("backup.json")
	if e != nil {
		return nil, e
	}
	set := map[string]any{}
	if b, e := os.ReadFile(filepath.Join(dir, "webui.settings.json")); e == nil {
		var raw map[string]any
		if json.Unmarshal(b, &raw) == nil {
			for _, k := range []string{"base", "base_http", "listen", "tunnel", "tunnel_id", "web_tunnel_id", "web_tunnel_host", "insecure"} {
				if v, ok := raw[k]; ok {
					set[k] = v
				}
			}
		}
	}
	payload := map[string]any{"format": "pan-vlessbank-redacted-backup-v1", "created": time.Now().UTC().Format(time.RFC3339), "accounts": accounts, "settings": set}
	b, e := json.MarshalIndent(payload, "", "  ")
	if e != nil {
		return nil, e
	}
	if _, e = w.Write(b); e != nil {
		return nil, e
	}
	if e := z.Close(); e != nil {
		return nil, e
	}
	return buf.Bytes(), nil
}
func ReadTunnelValidation(path string) TunnelValidation {
	b, e := os.ReadFile(path)
	if e != nil {
		return TunnelValidation{OK: false, Errors: []string{"config tunnel belum ada"}}
	}
	return ValidateProxyTunnel(b)
}
func Health(xray, proxyTunnel, webTunnel, config bool, tunnel TunnelValidation) map[string]any {
	errs := append([]string{}, tunnel.Errors...)
	sort.Strings(errs)
	return map[string]any{"xray": xray, "proxy_tunnel": proxyTunnel, "web_tunnel": webTunnel, "config": config, "tunnel": TunnelValidation{OK: tunnel.OK, Entries: tunnel.Entries, Errors: errs}}
}
