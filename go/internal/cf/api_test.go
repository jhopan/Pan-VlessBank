package cf

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withAPI(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	old := apiBase
	apiBase = srv.URL
	t.Cleanup(func() {
		apiBase = old
		srv.Close()
	})
	return srv
}

func TestAPIZones(t *testing.T) {
	var gotPath, gotQuery, gotAuth string
	withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotAuth = r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"success":true,"result":[{"id":"z1","name":"example.com","status":"active"}]}`)
	})

	res := APIZones("tok")
	if res["ok"] != true {
		t.Fatalf("APIZones: %v", res)
	}
	zones, ok := res["zones"].([]map[string]any)
	if !ok || len(zones) != 1 || zones[0]["id"] != "z1" || zones[0]["name"] != "example.com" || zones[0]["status"] != "active" {
		t.Fatalf("zones: %#v", res["zones"])
	}
	if gotPath != "/zones" || gotQuery != "per_page=50" || gotAuth != "Bearer tok" {
		t.Fatalf("request: path=%q query=%q auth=%q", gotPath, gotQuery, gotAuth)
	}
}

func TestAPIZonesError(t *testing.T) {
	withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"success":false,"errors":[{"code":10000,"message":"Invalid API Token"}]}`)
	})

	res := APIZones("bad")
	if res["ok"] != false || res["error"] != "Invalid API Token" {
		t.Fatalf("APIZones error: %v", res)
	}
}

func TestAPIZonesNoErrorsArray(t *testing.T) {
	withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"success":false}`)
	})
	if res := APIZones("bad"); res["ok"] != false || res["error"] != "API error" {
		t.Fatalf("APIZones no errors: %v", res)
	}
}

func TestAPICreateDNSUsesProxy(t *testing.T) {
	var body map[string]any
	var method, path, auth string
	withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		method, path, auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body: %v", err)
		}
		io.WriteString(w, `{"success":true,"result":{"id":"rec1"}}`)
	})

	res := APICreateDNS("tok", "zone1", "example.com", "tid-1", "socks")
	if res["ok"] != true || res["id"] != "rec1" ||
		res["name"] != "*.socks.example.com" || res["target"] != "tid-1.cfargotunnel.com" {
		t.Fatalf("APICreateDNS: %v", res)
	}
	if method != "POST" || path != "/zones/zone1/dns_records" || auth != "Bearer tok" {
		t.Fatalf("request: %s %s auth=%q", method, path, auth)
	}
	if body["proxied"] != true {
		t.Fatalf("proxied must be true (DNS-only breaks cloudflared access): %v", body["proxied"])
	}
	if body["type"] != "CNAME" || body["ttl"] != float64(1) ||
		body["name"] != "*.socks.example.com" || body["content"] != "tid-1.cfargotunnel.com" {
		t.Fatalf("body: %v", body)
	}
}

func TestAPICreateDNSAlreadyExists(t *testing.T) {
	withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"success":false,"errors":[{"code":81057,"message":"An A, AAAA or CNAME record already exists with that host"}]}`)
	})
	res := APICreateDNS("tok", "zone1", "example.com", "tid-1", "socks")
	if res["ok"] != true || res["note"] != "sudah ada" || res["id"] != "" ||
		res["name"] != "*.socks.example.com" || res["target"] != "tid-1.cfargotunnel.com" {
		t.Fatalf("81057: %v", res)
	}
}

func TestAPICreateDNSError(t *testing.T) {
	withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"success":false,"errors":[{"code":6003,"message":"Invalid request headers"}]}`)
	})
	res := APICreateDNS("tok", "zone1", "example.com", "tid-1", "socks")
	if res["ok"] != false || res["error"] != "Invalid request headers" {
		t.Fatalf("error: %v", res)
	}
}

func TestAPIUnreachableHost(t *testing.T) {
	old := apiBase
	apiBase = "http://127.0.0.1:1/client/v4"
	t.Cleanup(func() { apiBase = old })

	res := APIZones("tok")
	if res["ok"] != false {
		t.Fatalf("unreachable must fail: %v", res)
	}
	if s, _ := res["error"].(string); s == "" {
		t.Fatalf("empty error: %v", res)
	}
}

func TestAPIUnparsableBody(t *testing.T) {
	withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "<html>nope</html>")
	})
	res := APIZones("tok")
	if res["ok"] != false {
		t.Fatalf("unparsable body must fail: %v", res)
	}
}
