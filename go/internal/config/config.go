// Package config is the Go port of gen.py: accounts.txt (a list of vless:// links)
// becomes an xray config document, one account = two inbounds (SOCKS5 + HTTP proxy).
//
// Port SOCKS = base + i, port HTTP = base_http + i (base_http 0 means base+1000).
//
// gen.py is the behavioral spec, so urllib.parse quirks are reproduced on purpose:
// parse_qs drops blank values, '+' becomes a space in query values, an invalid or
// out-of-range port is treated as the 443 default instead of crashing, and the ws
// path (plus the dedup key path) is unquoted twice because gen.py unquotes the
// already-unquoted parse_qs value again.
package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Account is one valid, deduped vless entry from accounts.txt.
type Account struct {
	Line     int    // 1-based line number in accounts.txt
	Idx      int    // 0-based rank among valid, deduped accounts == port offset
	Raw      string // link text as parsed (line trimmed, like gen.py's links list)
	Name     string // unquoted url fragment; empty -> "acc{Idx+1}" (like Python build_outbound)
	Host     string // lowercase hostname
	Port     int    // default 443
	Network  string // tcp | ws | grpc
	Security string // none | tls | reality
	UUID     string
}

// Bind is one account's local listen pair plus its upstream endpoint.
type Bind struct {
	Tag       string // "acc1"...
	Name      string // account display name
	SocksPort int
	HTTPPort  int
	Endpoint  string // "host:port" of the upstream
}

// link is the subset of urllib.parse.SplitResult gen.py relies on.
type link struct {
	scheme   string
	user     string
	host     string // lowercase, without brackets; "" when absent
	port     int    // 443 when absent/unusable
	fragment string // raw (not unquoted) fragment
	query    map[string]string
}

// ValidLink: link vless yang minimal bisa dipakai — uuid (userinfo) dan host tidak
// kosong. Link seperti "vless://abc" atau "vless://host:443" tanpa uuid ditolak,
// supaya tidak masuk accounts.txt dan bikin xray gagal start.
func ValidLink(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "vless" {
		return false
	}
	if u.User == nil || strings.TrimSpace(u.User.Username()) == "" {
		return false
	}
	if strings.TrimSpace(u.Hostname()) == "" {
		return false
	}
	if u.Port() != "" {
		if p, err := strconv.Atoi(u.Port()); err != nil || p < 1 || p > 65535 {
			return false
		}
	}
	return true
}

// ParseFile reads accounts.txt: skips blank lines and lines starting with '#',
// dedups on uuid+host+port+path (duplicates dropped, later ones lose), then keeps
// the vless links and assigns Idx in order. dup is the number of duplicates
// dropped; non-vless lines take part in dedup exactly like gen.py does, they are
// just dropped afterwards (gen.py prints "SKIP (bukan vless): ..." for them).
//
// Zero valid accounts is not an error: callers mirror gen.py's
// "ERROR: tidak ada akun valid" themselves. A missing file returns an error.
func ParseFile(path string) ([]Account, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, fmt.Errorf("%s tidak ada: %w", path, err)
		}
		return nil, 0, err
	}

	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	type entry struct {
		line int
		raw  string
		l    link
	}
	var cands []entry
	for i, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "#") {
			continue // comment (checked on the unwrapped line, like gen.py)
		}
		if strings.TrimSpace(line) == "" {
			continue // blank
		}
		raw := strings.TrimSpace(line)
		cands = append(cands, entry{line: i + 1, raw: raw, l: parseLink(raw)})
	}

	seen := make(map[string]bool, len(cands))
	accs := make([]Account, 0, len(cands))
	dup := 0
	for _, e := range cands {
		key := dedupKey(e.l)
		if seen[key] {
			dup++
			continue
		}
		seen[key] = true
		if e.l.scheme != "vless" {
			continue
		}
		idx := len(accs)
		name := e.l.name(idx)
		accs = append(accs, Account{
			Line:     e.line,
			Idx:      idx,
			Raw:      e.raw,
			Name:     name,
			Host:     e.l.host,
			Port:     e.l.port,
			Network:  e.l.network(),
			Security: e.l.security(),
			UUID:     e.l.user,
		})
	}
	return accs, dup, nil
}

// Text is the dedup key: uuid + host + port + path (path unquoted a second time).
// An absent path keys as "" like gen.py's q.get("path", [""]), not as "/".
func dedupKey(l link) string {
	return l.user + "\x00" + l.host + "\x00" + strconv.Itoa(l.port) + "\x00" + unquote(l.path(""))
}

// DedupFile mirrors webui.dedup_accounts(): it reads path, keeps comments and
// blank lines exactly in place, drops later lines whose dedup key
// (uuid+host+port+path, fragment excluded) was already seen, and rewrites the
// file only when something was dropped. It returns the dropped count; a missing
// file is not an error (0 dropped), like the Python version.
//
// Line endings are preserved as found. Python's text-mode rewrite translates
// every line ending to os.linesep ("\r\n" on Windows); keeping the original
// bytes leaves LF lines alone instead of rewriting the whole file.
func DedupFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	lines := splitKeepEnds(string(data))
	seen := make(map[string]bool, len(lines))
	out := make([]string, 0, len(lines))
	dropped := 0
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			out = append(out, line)
			continue
		}
		key := dedupKey(parseLink(s))
		if seen[key] {
			dropped++
			continue
		}
		seen[key] = true
		out = append(out, line)
	}
	if dropped == 0 {
		return 0, nil
	}
	if err := os.WriteFile(path, []byte(strings.Join(out, "")), 0o644); err != nil {
		return dropped, err
	}
	return dropped, nil
}

// splitKeepEnds splits text into lines that keep their own terminator, matching
// the universal-newline splitting of Python's readlines().
func splitKeepEnds(s string) []string {
	var out []string
	for len(s) > 0 {
		i := strings.IndexAny(s, "\r\n")
		if i < 0 {
			out = append(out, s)
			break
		}
		end := i + 1
		if s[i] == '\r' && end < len(s) && s[end] == '\n' {
			end++
		}
		out = append(out, s[:end])
		s = s[end:]
	}
	return out
}

// BuildConfig mirrors gen.py main(): baseHTTP==0 means base+1000, empty auth means
// noauth. insecure adds tlsSettings.allowInsecure=true only when xrayMajor < 26
// (0/unknown counts as "apply", because Python only skips for major >= 26).
//
// It returns the exact same document Python writes and no binds when there is no
// valid account (Python exits with "ERROR: tidak ada akun valid" instead).
func BuildConfig(accs []Account, listen string, base, baseHTTP int, auth string, insecure bool, xrayMajor int) (map[string]any, []Bind) {
	if baseHTTP == 0 {
		baseHTTP = base + 1000
	}
	skipInsecure := insecure && xrayMajor >= 26

	var (
		inbounds  []any
		outbounds []any
		rules     []any
		binds     []Bind
	)
	i := 0
	for _, acc := range accs {
		l := parseLink(acc.Raw)
		if l.scheme != "vless" {
			continue
		}
		tag := fmt.Sprintf("acc%d", i+1)
		portS := base + i
		portH := baseHTTP + i

		stream := map[string]any{"network": l.network(), "security": l.security()}
		switch l.security() {
		case "tls":
			tls := map[string]any{"serverName": l.sni()}
			if v, ok := l.query["alpn"]; ok {
				tls["alpn"] = strings.Split(v, ",")
			}
			if v, ok := l.query["fp"]; ok {
				tls["fingerprint"] = v
			}
			stream["tlsSettings"] = tls
		case "reality":
			stream["realitySettings"] = map[string]any{
				"serverName":  l.sni(),
				"publicKey":   l.query["pbk"],
				"shortId":     l.query["sid"],
				"fingerprint": l.fp(),
			}
		}
		switch l.network() {
		case "ws":
			ws := map[string]any{"path": unquote(l.path("/"))}
			if v, ok := l.query["host"]; ok {
				ws["headers"] = map[string]any{"Host": v}
			}
			stream["wsSettings"] = ws
		case "grpc":
			stream["grpcSettings"] = map[string]any{"serviceName": l.query["serviceName"]}
		}
		if insecure && !skipInsecure && l.security() == "tls" {
			tls, _ := stream["tlsSettings"].(map[string]any)
			if tls == nil {
				tls = map[string]any{}
				stream["tlsSettings"] = tls
			}
			tls["allowInsecure"] = true
		}

		ob := map[string]any{
			"tag":      tag,
			"protocol": "vless",
			"settings": map[string]any{"vnext": []any{map[string]any{
				"address": l.host,
				"port":    l.port,
				"users": []any{map[string]any{
					"id":         l.user,
					"encryption": "none",
					"flow":       l.query["flow"],
				}},
			}}},
			"streamSettings": stream,
		}

		inbounds = append(inbounds, socksInbound(fmt.Sprintf("in_s%d", i+1), listen, portS, auth))
		inbounds = append(inbounds, httpInbound(fmt.Sprintf("in_h%d", i+1), listen, portH, auth))
		outbounds = append(outbounds, ob)
		rules = append(rules, map[string]any{
			"type":        "field",
			"inboundTag":  []any{fmt.Sprintf("in_s%d", i+1), fmt.Sprintf("in_h%d", i+1)},
			"outboundTag": tag,
		})
		binds = append(binds, Bind{
			Tag:       tag,
			Name:      l.name(i),
			SocksPort: portS,
			HTTPPort:  portH,
			Endpoint:  net.JoinHostPort(l.host, strconv.Itoa(l.port)),
		})
		i++
	}
	if len(inbounds) == 0 {
		return nil, nil
	}

	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"dns": map[string]any{
			"servers":         []any{"1.1.1.1", "8.8.8.8"},
			"queryStrategy":   "UseIPv4",
			"disableCache":    false,
			"disableFallback": false,
		},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"routing":   map[string]any{"domainStrategy": "AsIs", "rules": rules},
	}
	return cfg, binds
}

// WriteJSON writes v as indented JSON (2 spaces), like json.dump(..., indent=2):
// no trailing newline, no HTML escaping.
func WriteJSON(path string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return os.WriteFile(path, bytes.TrimSuffix(buf.Bytes(), []byte("\n")), 0o644)
}

// TunnelYML mirrors gen.py's --tunnel output: hostname numbering comes from
// port_s - base + 1. credPath dipakai apa adanya; kosong berarti placeholder
// (versi lama). Pemanggil yang menentukan path kredensial asli supaya regen
// tidak menimpa yml buatan panel dengan placeholder.
func TunnelYML(tunnelID, domain string, binds []Bind, base int, credPath string) string {
	var b strings.Builder
	id := tunnelID
	if id == "" {
		id = "<TUNNEL-UUID>"
	}
	if credPath == "" {
		credPath = "<PATH>/<TUNNEL-UUID>.json"
	}
	fmt.Fprintf(&b, "tunnel: %s\n", id)
	fmt.Fprintf(&b, "credentials-file: %s\n", credPath)
	b.WriteString("ingress:\n")
	for _, bd := range binds {
		n := bd.SocksPort - base + 1
		fmt.Fprintf(&b, "  - hostname: socks%d.%s\n    service: tcp://localhost:%d\n", n, domain, bd.SocksPort)
		fmt.Fprintf(&b, "  - hostname: http%d.%s\n    service: tcp://localhost:%d\n", n, domain, bd.HTTPPort)
	}
	b.WriteString("  - service: http_status:404\n")
	return b.String()
}

var xrayVersionRe = regexp.MustCompile(`[Xx]ray\s+(\d+)\.`)

// XrayMajor runs "<bin> version" and returns the major version ("Xray 26.3.27" ->
// 26). 0 when the binary is missing, times out, or prints nothing recognizable.
// An empty bin means "xray" from PATH.
func XrayMajor(bin string) int {
	if bin == "" {
		bin = "xray"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, bin, "version").CombinedOutput()
	m := xrayVersionRe.FindStringSubmatch(string(out))
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

func socksInbound(tag, listen string, port int, auth string) map[string]any {
	s := map[string]any{"auth": "noauth", "udp": true}
	if auth != "" {
		user, pw, _ := strings.Cut(auth, ":")
		s = map[string]any{
			"auth":     "password",
			"accounts": []any{map[string]any{"user": user, "pass": pw}},
			"udp":      true,
		}
	}
	return map[string]any{
		"tag":      tag,
		"listen":   listen,
		"port":     port,
		"protocol": "socks",
		"settings": s,
		"sniffing": map[string]any{"enabled": true, "destOverride": []any{"http", "tls"}},
	}
}

func httpInbound(tag, listen string, port int, auth string) map[string]any {
	s := map[string]any{}
	if auth != "" {
		user, pw, _ := strings.Cut(auth, ":")
		s = map[string]any{"accounts": []any{map[string]any{"user": user, "pass": pw}}}
	}
	return map[string]any{
		"tag":      tag,
		"listen":   listen,
		"port":     port,
		"protocol": "http",
		"settings": s,
		"sniffing": map[string]any{"enabled": true, "destOverride": []any{"http", "tls"}},
	}
}

func (l link) network() string {
	if v, ok := l.query["type"]; ok {
		return v
	}
	return "tcp"
}

func (l link) security() string {
	if v, ok := l.query["security"]; ok {
		return v
	}
	return "none"
}

// sni defaults to the hostname, like gen.py's q.get("sni", addr).
func (l link) sni() string {
	if v, ok := l.query["sni"]; ok {
		return v
	}
	return l.host
}

func (l link) fp() string {
	if v, ok := l.query["fp"]; ok {
		return v
	}
	return "chrome"
}

func (l link) path(def string) string {
	if v, ok := l.query["path"]; ok {
		return v
	}
	return def
}

// name is unquote(fragment or "acc{i+1}"), i.e. the account display name.
func (l link) name(i int) string {
	if l.fragment == "" {
		return fmt.Sprintf("acc%d", i+1)
	}
	return unquote(l.fragment)
}

// parseLink mimics urllib.parse.urlparse + parse_qs (keep_blank_values=False) as
// used by gen.py.
func parseLink(raw string) link {
	// urlsplit() strips these before parsing.
	s := strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(raw)
	l := link{query: map[string]string{}, port: 443}

	body := s
	if i := strings.IndexByte(s, ':'); i > 0 && isASCIIAlpha(s[0]) {
		if isScheme(s[:i]) {
			l.scheme = strings.ToLower(s[:i])
			body = s[i+1:]
		}
	}

	netloc := ""
	if strings.HasPrefix(body, "//") {
		body = body[2:]
		end := len(body)
		for _, c := range []byte{'/', '?', '#'} {
			if j := strings.IndexByte(body, c); j >= 0 && j < end {
				end = j
			}
		}
		netloc, body = body[:end], body[end:]
	}
	if i := strings.IndexByte(body, '#'); i >= 0 {
		l.fragment, body = body[i+1:], body[:i]
	}
	qs := ""
	if i := strings.IndexByte(body, '?'); i >= 0 {
		qs = body[i+1:]
	}

	hostinfo := netloc
	if i := strings.LastIndexByte(netloc, '@'); i >= 0 {
		l.user, hostinfo = netloc[:i], netloc[i+1:]
	}
	host, portStr := "", ""
	if strings.ContainsRune(hostinfo, '[') {
		_, rest, _ := strings.Cut(hostinfo, "[")
		host, rest, _ = strings.Cut(rest, "]")
		_, portStr, _ = strings.Cut(rest, ":")
	} else {
		host, portStr, _ = strings.Cut(hostinfo, ":")
	}
	l.host = strings.ToLower(host)
	if portStr != "" {
		if n, err := strconv.Atoi(portStr); err == nil && n > 0 && n <= 65535 {
			l.port = n
		}
	}

	for _, pair := range strings.Split(qs, "&") {
		if pair == "" {
			continue
		}
		k, v, _ := strings.Cut(pair, "=")
		if v == "" {
			continue // keep_blank_values=False
		}
		k = unquote(strings.ReplaceAll(k, "+", " "))
		v = unquote(strings.ReplaceAll(v, "+", " "))
		if _, dup := l.query[k]; !dup {
			l.query[k] = v
		}
	}
	return l
}

func isASCIIAlpha(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// isScheme reports whether s is a valid URL scheme (RFC 3986, as urlsplit checks).
func isScheme(s string) bool {
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '+' || c == '-' || c == '.':
		default:
			return false
		}
	}
	return len(s) > 0
}

// unquote percent-decodes like urllib.parse.unquote: '+' is left alone and an
// invalid escape sequence stays literal.
func unquote(s string) string {
	if !strings.ContainsRune(s, '%') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hi, ok1 := hexVal(s[i+1])
			lo, ok2 := hexVal(s[i+2])
			if ok1 && ok2 {
				b.WriteByte(hi<<4 | lo)
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
