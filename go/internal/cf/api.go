package cf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// apiBase is the Cloudflare REST root (overridable in tests).
var apiBase = "https://api.cloudflare.com/client/v4"

var apiClient = &http.Client{Timeout: 10 * time.Second}

// cfAPI mirrors cf.py _cf_api: returns the decoded JSON body, or the
// {"success":false,"errors":[...]} shape on any failure.
func cfAPI(token, path, method string, body any) map[string]any {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return apiError(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, apiBase+path, rd)
	if err != nil {
		return apiError(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := apiClient.Do(req)
	if err != nil {
		return apiError(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiError(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return apiError(err)
	}
	return out
}

func apiError(err error) map[string]any {
	return map[string]any{
		"success": false,
		"errors":  []any{map[string]any{"message": err.Error()}},
	}
}

// firstError extracts errors[0].message, falling back to "API error".
func firstError(r map[string]any) string {
	if errs, ok := r["errors"].([]any); ok && len(errs) > 0 {
		if m, ok := errs[0].(map[string]any); ok {
			if s, ok := m["message"].(string); ok {
				return s
			}
		}
	}
	return "API error"
}

// errorCode returns errors[0].code, -1 when absent.
func errorCode(r map[string]any) int {
	if errs, ok := r["errors"].([]any); ok && len(errs) > 0 {
		if m, ok := errs[0].(map[string]any); ok {
			if c, ok := m["code"].(float64); ok {
				return int(c)
			}
		}
	}
	return -1
}

// APIZones mirrors cf.py cf_api_zones.
func APIZones(token string) map[string]any {
	r := cfAPI(token, "/zones?per_page=50", "GET", nil)
	if ok, _ := r["success"].(bool); !ok {
		return map[string]any{"ok": false, "error": firstError(r)}
	}
	zones := []map[string]any{}
	if arr, ok := r["result"].([]any); ok {
		for _, z := range arr {
			zm, ok := z.(map[string]any)
			if !ok {
				continue
			}
			zones = append(zones, map[string]any{
				"id":     jsonStr(zm["id"]),
				"name":   jsonStr(zm["name"]),
				"status": jsonStr(zm["status"]),
			})
		}
	}
	return map[string]any{"ok": true, "zones": zones}
}

// APICreateDNS creates the wildcard CNAME *.<sub>.<zoneName> ->
// <tunnelID>.cfargotunnel.com with proxied TRUE (orange cloud). DNS-only
// records make the hostname unresolvable for cloudflared, which is why the
// Python version's DNS-only variant is a bug. Code 81057 (already exists)
// counts as success.
func APICreateDNS(token, zoneID, zoneName, tunnelID, sub string) map[string]any {
	// sub kosong = wildcard 1 level (*.zone). Wajib untuk cert Universal SSL free plan:
	// hostname 2 level (socks1.socks.zone) gagal TLS handshake di cloudflared access.
	name := "*." + zoneName
	if sub != "" {
		name = "*." + sub + "." + zoneName
	}
	target := tunnelID + ".cfargotunnel.com"
	body := map[string]any{
		"type":    "CNAME",
		"name":    name,
		"content": target,
		"ttl":     1,
		"proxied": true,
	}
	r := cfAPI(token, fmt.Sprintf("/zones/%s/dns_records", zoneID), "POST", body)
	if ok, _ := r["success"].(bool); ok {
		rec, _ := r["result"].(map[string]any)
		return map[string]any{"ok": true, "id": jsonStr(rec["id"]), "name": name, "target": target}
	}
	if errorCode(r) == 81057 {
		return map[string]any{"ok": true, "id": "", "name": name, "target": target, "note": "sudah ada"}
	}
	return map[string]any{"ok": false, "error": firstError(r)}
}

// APICreateWebDNS creates the proxied CNAME for an isolated WebUI tunnel.
func APICreateWebDNS(token, zoneID, hostname, tunnelID string) map[string]any {
	res := cfAPI(token, "/zones/"+zoneID+"/dns_records", http.MethodPost, map[string]any{
		"type": "CNAME", "name": hostname, "content": tunnelID + ".cfargotunnel.com", "proxied": true,
	})
	if ok, _ := res["success"].(bool); ok || errorCode(res) == 81057 {
		return map[string]any{"ok": true, "hostname": hostname, "target": tunnelID + ".cfargotunnel.com"}
	}
	return map[string]any{"ok": false, "error": firstError(res)}
}
