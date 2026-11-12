package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Synthetic fixtures only: never real credentials.
const (
	linkOne = "vless://11111111-1111-1111-1111-111111111111@edge-one.example.com:443" +
		"?type=ws&security=tls&sni=edge-one.example.com&path=%252Fws%252Fbank" +
		"&host=edge-one.example.com&alpn=h2%2Chttp%2F1.1&fp=chrome#Akun%20Satu"
	linkTwo = "vless://22222222-2222-2222-2222-222222222222@edge-two.example.com:8443" +
		"?type=grpc&security=reality&sni=edge-two.example.com&pbk=PUBKEY123&sid=ab12cd" +
		"&fp=firefox&serviceName=grpcsvc&flow=xtls-rprx-vision#Akun%20Dua"
)

// accountsBody is a 2-account fixture plus a comment, a blank line, a duplicate
// of the first link and a non-vless line.
func accountsBody() string {
	return strings.Join([]string{
		"# komentar",
		"",
		linkOne,
		linkOne, // duplicate -> dropped by dedup
		"trojan://x@nowhere.example.com:443?path=/x", // non-vless -> skipped
		linkTwo,
		"",
	}, "\n")
}

func writeAccounts(t *testing.T, dir string) string {
	t.Helper()
	body := accountsBody()
	path := filepath.Join(dir, "accounts.txt")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func obj(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("want object, got %T (%v)", v, v)
	}
	return m
}

func arr(t *testing.T, v any) []any {
	t.Helper()
	a, ok := v.([]any)
	if !ok {
		t.Fatalf("want array, got %T (%v)", v, v)
	}
	return a
}

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := writeAccounts(t, dir)

	accs, dup, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if dup != 1 {
		t.Errorf("dup = %d, want 1", dup)
	}
	if len(accs) != 2 {
		t.Fatalf("got %d accounts, want 2", len(accs))
	}

	want := []Account{
		{
			Line: 3, Idx: 0, Raw: linkOne, Name: "Akun Satu",
			Host: "edge-one.example.com", Port: 443, Network: "ws", Security: "tls",
			UUID: "11111111-1111-1111-1111-111111111111",
		},
		{
			Line: 6, Idx: 1, Raw: linkTwo, Name: "Akun Dua",
			Host: "edge-two.example.com", Port: 8443, Network: "grpc", Security: "reality",
			UUID: "22222222-2222-2222-2222-222222222222",
		},
	}
	if !reflect.DeepEqual(accs, want) {
		t.Errorf("accounts mismatch\n got %+v\nwant %+v", accs, want)
	}

	if _, _, err := ParseFile(filepath.Join(dir, "tidak-ada.txt")); err == nil {
		t.Error("missing file: want error")
	}
}

func TestParseFileDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.txt")
	// no port, no type/security, empty fragment, IPv6 literal
	body := "vless://44444444-4444-4444-4444-444444444444@edge-four.example.com#\n" +
		"vless://55555555-5555-5555-5555-555555555555@[2001:db8::1]:8443?type=tcp#V6\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	accs, dup, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if dup != 0 || len(accs) != 2 {
		t.Fatalf("dup=%d accs=%d, want 0/2", dup, len(accs))
	}
	if got := accs[0]; got.Name != "acc1" || got.Port != 443 || got.Network != "tcp" || got.Security != "none" {
		t.Errorf("defaults mismatch: %+v", got)
	}
	if got := accs[1]; got.Host != "2001:db8::1" || got.Port != 8443 {
		t.Errorf("ipv6 host mismatch: %+v", got)
	}

	cfg, binds := BuildConfig(accs, "127.0.0.1", 1081, 0, "", false, 0)
	if binds[1].Endpoint != "[2001:db8::1]:8443" {
		t.Errorf("endpoint = %q, want [2001:db8::1]:8443", binds[1].Endpoint)
	}
	o0 := obj(t, arr(t, cfg["outbounds"])[0])
	st := obj(t, o0["streamSettings"])
	if st["network"] != "tcp" || st["security"] != "none" {
		t.Errorf("streamSettings = %v", st)
	}
	for _, k := range []string{"tlsSettings", "wsSettings", "grpcSettings", "realitySettings"} {
		if _, ok := st[k]; ok {
			t.Errorf("unexpected %s for plain tcp account", k)
		}
	}
}

func TestBuildConfigDocument(t *testing.T) {
	dir := t.TempDir()
	accs, _, err := ParseFile(writeAccounts(t, dir))
	if err != nil {
		t.Fatal(err)
	}

	cfg, binds := BuildConfig(accs, "127.0.0.1", 1081, 0, "", false, 0)
	if cfg == nil {
		t.Fatal("BuildConfig returned no document")
	}

	wantBinds := []Bind{
		{Tag: "acc1", Name: "Akun Satu", SocksPort: 1081, HTTPPort: 2081, Endpoint: "edge-one.example.com:443"},
		{Tag: "acc2", Name: "Akun Dua", SocksPort: 1082, HTTPPort: 2082, Endpoint: "edge-two.example.com:8443"},
	}
	if !reflect.DeepEqual(binds, wantBinds) {
		t.Errorf("binds mismatch\n got %+v\nwant %+v", binds, wantBinds)
	}

	// log + dns + routing
	if got := obj(t, cfg["log"])["loglevel"]; got != "warning" {
		t.Errorf("loglevel = %v", got)
	}
	dns := obj(t, cfg["dns"])
	if !reflect.DeepEqual(dns["servers"], []any{"1.1.1.1", "8.8.8.8"}) {
		t.Errorf("dns servers = %v", dns["servers"])
	}
	if dns["queryStrategy"] != "UseIPv4" || dns["disableCache"] != false || dns["disableFallback"] != false {
		t.Errorf("dns = %v", dns)
	}
	routing := obj(t, cfg["routing"])
	if routing["domainStrategy"] != "AsIs" {
		t.Errorf("domainStrategy = %v", routing["domainStrategy"])
	}
	rules := arr(t, routing["rules"])
	if len(rules) != 2 {
		t.Fatalf("rules = %d, want 2", len(rules))
	}
	if !reflect.DeepEqual(obj(t, rules[0]), map[string]any{
		"type":        "field",
		"inboundTag":  []any{"in_s1", "in_h1"},
		"outboundTag": "acc1",
	}) {
		t.Errorf("rule 0 = %v", rules[0])
	}
	if obj(t, rules[1])["outboundTag"] != "acc2" {
		t.Errorf("rule 1 = %v", rules[1])
	}

	// inbounds: in_sN then in_hN per account
	inbounds := arr(t, cfg["inbounds"])
	if len(inbounds) != 4 {
		t.Fatalf("inbounds = %d, want 4", len(inbounds))
	}
	wantTags := []string{"in_s1", "in_h1", "in_s2", "in_h2"}
	wantPorts := []int{1081, 2081, 1082, 2082}
	for i, in := range inbounds {
		m := obj(t, in)
		if m["tag"] != wantTags[i] || m["port"] != wantPorts[i] {
			t.Errorf("inbound %d = tag %v port %v, want %s/%d", i, m["tag"], m["port"], wantTags[i], wantPorts[i])
		}
		if m["listen"] != "127.0.0.1" {
			t.Errorf("inbound %d listen = %v", i, m["listen"])
		}
		sniff := obj(t, m["sniffing"])
		if sniff["enabled"] != true || !reflect.DeepEqual(sniff["destOverride"], []any{"http", "tls"}) {
			t.Errorf("inbound %d sniffing = %v", i, sniff)
		}
	}
	if got := obj(t, obj(t, inbounds[0])["settings"]); got["auth"] != "noauth" || got["udp"] != true {
		t.Errorf("socks settings = %v", got)
	}
	if obj(t, inbounds[0])["protocol"] != "socks" || obj(t, inbounds[1])["protocol"] != "http" {
		t.Error("inbound protocols wrong")
	}
	if len(obj(t, obj(t, inbounds[1])["settings"])) != 0 {
		t.Errorf("http settings = %v, want {}", obj(t, inbounds[1])["settings"])
	}

	// outbounds
	outbounds := arr(t, cfg["outbounds"])
	if len(outbounds) != 2 {
		t.Fatalf("outbounds = %d, want 2", len(outbounds))
	}
	o0 := obj(t, outbounds[0])
	if o0["tag"] != "acc1" || o0["protocol"] != "vless" {
		t.Errorf("outbound 0 = %v", o0)
	}
	v0 := obj(t, arr(t, obj(t, o0["settings"])["vnext"])[0])
	if v0["address"] != "edge-one.example.com" || v0["port"] != 443 {
		t.Errorf("vnext 0 = %v", v0)
	}
	u0 := obj(t, arr(t, v0["users"])[0])
	if u0["id"] != "11111111-1111-1111-1111-111111111111" || u0["encryption"] != "none" || u0["flow"] != "" {
		t.Errorf("user 0 = %v", u0)
	}
	st0 := obj(t, o0["streamSettings"])
	if st0["network"] != "ws" || st0["security"] != "tls" {
		t.Errorf("streamSettings 0 = %v", st0)
	}
	tls0 := obj(t, st0["tlsSettings"])
	if tls0["serverName"] != "edge-one.example.com" || !reflect.DeepEqual(tls0["alpn"], []string{"h2", "http/1.1"}) {
		t.Errorf("tlsSettings = %v", tls0)
	}
	if tls0["fingerprint"] != "chrome" {
		t.Errorf("tls fingerprint = %v", tls0["fingerprint"])
	}
	if _, ok := tls0["allowInsecure"]; ok {
		t.Error("allowInsecure present without --insecure")
	}
	ws0 := obj(t, st0["wsSettings"])
	if ws0["path"] != "/ws/bank" {
		t.Errorf("ws path = %v (double-unquote mismatch)", ws0["path"])
	}
	if !reflect.DeepEqual(ws0["headers"], map[string]any{"Host": "edge-one.example.com"}) {
		t.Errorf("ws headers = %v", ws0["headers"])
	}

	o1 := obj(t, outbounds[1])
	if o1["tag"] != "acc2" {
		t.Errorf("outbound 1 tag = %v", o1["tag"])
	}
	v1 := obj(t, arr(t, obj(t, o1["settings"])["vnext"])[0])
	if v1["address"] != "edge-two.example.com" || v1["port"] != 8443 {
		t.Errorf("vnext 1 = %v", v1)
	}
	if u1 := obj(t, arr(t, v1["users"])[0]); u1["flow"] != "xtls-rprx-vision" {
		t.Errorf("user 1 = %v", u1)
	}
	st1 := obj(t, o1["streamSettings"])
	if st1["network"] != "grpc" || st1["security"] != "reality" {
		t.Errorf("streamSettings 1 = %v", st1)
	}
	reality := obj(t, st1["realitySettings"])
	want := map[string]any{
		"serverName":  "edge-two.example.com",
		"publicKey":   "PUBKEY123",
		"shortId":     "ab12cd",
		"fingerprint": "firefox",
	}
	if !reflect.DeepEqual(reality, want) {
		t.Errorf("realitySettings = %v, want %v", reality, want)
	}
	if obj(t, st1["grpcSettings"])["serviceName"] != "grpcsvc" {
		t.Errorf("grpcSettings = %v", st1["grpcSettings"])
	}
}

func TestBuildConfigAuthPortsAndInsecure(t *testing.T) {
	dir := t.TempDir()
	accs, _, err := ParseFile(writeAccounts(t, dir))
	if err != nil {
		t.Fatal(err)
	}

	cfg, binds := BuildConfig(accs, "0.0.0.0", 1081, 2081, "user:pass", true, 25)
	inbounds := arr(t, cfg["inbounds"])
	socks := obj(t, obj(t, inbounds[0])["settings"])
	if socks["auth"] != "password" || socks["udp"] != true {
		t.Errorf("socks settings = %v", socks)
	}
	if !reflect.DeepEqual(socks["accounts"], []any{map[string]any{"user": "user", "pass": "pass"}}) {
		t.Errorf("socks accounts = %v", socks["accounts"])
	}
	http := obj(t, obj(t, inbounds[1])["settings"])
	if !reflect.DeepEqual(http["accounts"], []any{map[string]any{"user": "user", "pass": "pass"}}) {
		t.Errorf("http accounts = %v", http["accounts"])
	}
	if binds[1].SocksPort != 1082 || binds[1].HTTPPort != 2082 {
		t.Errorf("explicit base_http binds = %+v", binds)
	}

	// insecure: applied below xray 26, including when the major is unknown (0)
	for _, tc := range []struct {
		major int
		want  bool
	}{{0, true}, {25, true}, {26, false}, {27, false}} {
		c, _ := BuildConfig(accs, "127.0.0.1", 1081, 0, "", true, tc.major)
		outs := arr(t, c["outbounds"])
		st := obj(t, obj(t, outs[0])["streamSettings"])
		if _, got := obj(t, st["tlsSettings"])["allowInsecure"]; got != tc.want {
			t.Errorf("xray %d: allowInsecure = %v, want %v", tc.major, got, tc.want)
		}
		rst := obj(t, obj(t, outs[1])["streamSettings"])
		if _, ok := obj(t, rst["realitySettings"])["allowInsecure"]; ok {
			t.Errorf("xray %d: reality outbound got allowInsecure", tc.major)
		}
	}

	// base_http 0 -> base+1000
	_, binds = BuildConfig(accs, "0.0.0.0", 3000, 0, "user:pass", false, 0)
	if binds[0].SocksPort != 3000 || binds[0].HTTPPort != 4000 || binds[1].HTTPPort != 4001 {
		t.Errorf("derived http ports = %+v", binds)
	}

	// no valid account -> no document (gen.py: "ERROR: tidak ada akun valid")
	if cfg, binds := BuildConfig(nil, "127.0.0.1", 1081, 0, "", false, 0); cfg != nil || binds != nil {
		t.Errorf("empty accounts -> %v, %v; want nil, nil", cfg, binds)
	}
}

func TestWriteJSON(t *testing.T) {
	dir := t.TempDir()
	accs, _, err := ParseFile(writeAccounts(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := BuildConfig(accs, "127.0.0.1", 1081, 0, "", false, 0)

	path := filepath.Join(dir, "config.json")
	if err := WriteJSON(path, cfg); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.HasSuffix(text, "\n") {
		t.Error("trailing newline written; WriteJSON must not add one")
	}
	if !strings.Contains(text, "\n  \"log\": {") || !strings.Contains(text, "\"port\": 1081") {
		t.Errorf("unexpected indentation:\n%s", text)
	}
	if got := readJSON(t, path); !reflect.DeepEqual(got, readJSON(t, writeJSONOf(t, dir, cfg))) {
		t.Error("round-trip mismatch")
	}
}

// readJSON membaca file JSON untuk perbandingan round-trip.
func readJSON(t *testing.T, path string) any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// writeJSONOf re-marshals cfg through WriteJSON to check the round-trip is stable.
func writeJSONOf(t *testing.T, dir string, v any) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "round.json")
	if err := WriteJSON(p, v); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTunnelYML(t *testing.T) {
	dir := t.TempDir()
	accs, _, err := ParseFile(writeAccounts(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	_, binds := BuildConfig(accs, "127.0.0.1", 1081, 0, "", false, 0)

	want := strings.Join([]string{
		"tunnel: <TUNNEL-UUID>",
		"credentials-file: <PATH>/<TUNNEL-UUID>.json",
		"ingress:",
		"  - hostname: socks1.example.com",
		"    service: tcp://localhost:1081",
		"  - hostname: http1.example.com",
		"    service: tcp://localhost:2081",
		"  - hostname: socks2.example.com",
		"    service: tcp://localhost:1082",
		"  - hostname: http2.example.com",
		"    service: tcp://localhost:2082",
		"  - service: http_status:404",
		"",
	}, "\n")
	if got := TunnelYML("", "example.com", binds, 1081, ""); got != want {
		t.Errorf("TunnelYML mismatch\n got:\n%s\nwant:\n%s", got, want)
	}

	if got := TunnelYML("abc-123", "example.com", binds, 1081, ""); !strings.HasPrefix(got, "tunnel: abc-123\n") {
		t.Errorf("tunnel id not used: %q", got)
	}
	if got := TunnelYML("abc-123", "example.com", binds, 1081, ""); !strings.Contains(got, "credentials-file: <PATH>/<TUNNEL-UUID>.json") {
		t.Errorf("credentials path no longer a placeholder: %q", got)
	}
	if got := TunnelYML("abc-123", "example.com", binds, 1081, `C:\Users\x\.cloudflared\abc-123.json`); !strings.Contains(got, `credentials-file: C:\Users\x\.cloudflared\abc-123.json`) {
		t.Errorf("real credentials path not kept: %q", got)
	}
}

// parityCase is one fixture compared against the reference implementation.
type parityCase struct {
	name     string
	body     string
	listen   string
	base     int
	baseHTTP int
	auth     string
}

func TestDedupFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.txt")

	// same key as linkOne (uuid/host/port/path) but different query order and a
	// different fragment name -> dropped
	dupByKey := "vless://11111111-1111-1111-1111-111111111111@edge-one.example.com:443" +
		"?security=tls&type=ws&path=%252Fws%252Fbank&host=edge-one.example.com" +
		"&alpn=h2%2Chttp%2F1.1&fp=chrome&sni=edge-one.example.com#Nama%20Lain"

	body := "# komentar\n" +
		"\n" +
		linkOne + "\n" +
		dupByKey + "\n" +
		linkTwo + "\r\n" + // CRLF line must survive untouched
		"  # komentar indentasi\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	dropped, err := DedupFile(path)
	if err != nil {
		t.Fatalf("DedupFile: %v", err)
	}
	if dropped != 1 {
		t.Errorf("dropped = %d, want 1", dropped)
	}

	want := "# komentar\n" +
		"\n" +
		linkOne + "\n" +
		linkTwo + "\r\n" +
		"  # komentar indentasi\n"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); got != want {
		t.Errorf("file after dedup mismatch\n got: %q\nwant: %q", got, want)
	}

	// idempotent: nothing left to drop, file untouched
	if dropped, err := DedupFile(path); err != nil || dropped != 0 {
		t.Errorf("second run = (%d, %v), want (0, nil)", dropped, err)
	}
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != want {
		t.Errorf("file changed on second run: %q", string(again))
	}

	// missing file -> 0, no error (like webui.dedup_accounts)
	if dropped, err := DedupFile(filepath.Join(dir, "tidak-ada.txt")); err != nil || dropped != 0 {
		t.Errorf("missing file = (%d, %v), want (0, nil)", dropped, err)
	}

	// absent path and explicit path=/ are different keys (gen.py keys them "" and "/")
	slash := filepath.Join(dir, "slash.txt")
	sameHost := "edge-seven.example.com:443"
	if err := os.WriteFile(slash, []byte(
		"vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@"+sameHost+"?type=ws&path=/\n"+
			"vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@"+sameHost+"?type=ws\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if dropped, err := DedupFile(slash); err != nil || dropped != 0 {
		t.Errorf("absent vs explicit path=/ = (%d, %v), want (0, nil)", dropped, err)
	}
}

// dedupPy is webui.dedup_accounts() with the same logic, reading from sys.argv[1]
// and writing the retained lines to sys.argv[2] with newline="" so the platform's
// line separator is not applied (content, not os.linesep, is what we compare).
