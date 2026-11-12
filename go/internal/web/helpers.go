package web

// helpers.go: util kecil + menu sidebar (MENUS di webui.py).

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"vlessbank/internal/config"
)

// esc sama dengan html.escape(s, quote=True) di Python.
func esc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#x27;")
	return s
}

const pyQuoteSafe = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_.-~/"

// pyQuote sama dengan urllib.parse.quote(s) (safe="/", byte UTF-8 -> %XX uppercase).
func pyQuote(s string) string {
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if strings.IndexByte(pyQuoteSafe, c) >= 0 {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

// fill mengganti placeholder @@NAMA@@ sekali jalan (tanpa rescan hasil ganti).
func fill(tmpl string, kv ...string) string {
	return strings.NewReplacer(kv...).Replace(tmpl)
}

func itoa(n int) string { return strconv.Itoa(n) }

// ---------- pembaca nilai map[string]any dari paket cf ----------
// Shape JSON-nya tetap (lihat kontrak), helper ini cuma bikin aman tipe.

func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func asBool(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func asInt(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	default:
		return 0
	}
}

func asList(v any) []map[string]any {
	switch t := v.(type) {
	case []map[string]any:
		return t
	case []any:
		out := make([]map[string]any, 0, len(t))
		for _, e := range t {
			if m, ok := e.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

// menus sama dengan MENUS di webui.py: (tab, url, ikon, label).
var menus = []struct{ tab, url, ico, label string }{
	{"dash", "/admin", "▤", "Dashboard"},
	{"akun", "/admin/akun", "≡", "Akun"},
	{"log", "/admin/log", "☰", "Log"},
	{"cf", "/admin/cf", "☁", "Cloudflare"},
	{"set", "/admin/settings", "⚙", "Pengaturan"},
}

// lanIP mengembalikan IPv4 LAN yang dipakai client. Urutan prioritas:
// 192.168/16 (LAN rumah/kantor) > 10/8 > 172.16/12 > alamat lain, dan alamat
// loopback/link-local (169.254.x dari adapter mati)/unspecified dilewati.
func lanIP() string {
	type cand struct {
		ip   net.IP
		rank int
	}
	var best *cand
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			v4 := ipn.IP.To4()
			if v4 == nil || v4.IsLoopback() || v4.IsLinkLocalUnicast() || v4.IsUnspecified() {
				continue
			}
			r := 3
			switch {
			case v4[0] == 192 && v4[1] == 168:
				r = 0
			case v4[0] == 10:
				r = 1
			case v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31:
				r = 2
			}
			if best == nil || r < best.rank {
				best = &cand{ip: v4, rank: r}
			}
			if best.rank == 0 {
				return best.ip.String()
			}
		}
	}
	if best != nil {
		return best.ip.String()
	}
	return "127.0.0.1"
}

func menuLabel(tab string) string {
	for _, m := range menus {
		if m.tab == tab {
			return m.label
		}
	}
	return ""
}

// ---------- baris akun (rows() di webui.py) ----------

type row struct {
	Line   int // nomor baris accounts.txt (1-based, sama dengan config.Account.Line)
	Idx    int
	Socks  int
	HTTP   int
	Name   string
	Server string
	Tipe   string
}

func (s *Server) rows() []row {
	set := s.set()
	accs := s.d.Accounts()
	baseHTTP := set.BaseHTTP
	if baseHTTP == 0 {
		baseHTTP = set.Base + 1000
	}
	out := make([]row, 0, len(accs))
	for _, a := range accs {
		out = append(out, row{
			// webui.py's rows() memakai index enumerate (0-based) atas seluruh file,
			// jadi nilainya = nomor baris fisik - 1. Dipakai juga oleh /api/delete.
			Line:   a.Line - 1,
			Idx:    a.Idx,
			Socks:  set.Base + a.Idx,
			HTTP:   baseHTTP + a.Idx,
			Name:   a.Name,
			Server: a.Host + ":" + strconv.Itoa(a.Port),
			Tipe:   a.Network + "/" + a.Security,
		})
	}
	return out
}

// rowsJSON sama dengan dict yang di-return rows() di webui.py.
func (s *Server) rowsJSON() []map[string]any {
	rs := s.rows()
	out := make([]map[string]any, 0, len(rs))
	for _, x := range rs {
		out = append(out, map[string]any{
			"line": x.Line, "idx": x.Idx,
			"port_socks": x.Socks, "port_http": x.HTTP,
			"name": x.Name, "server": x.Server, "tipe": x.Tipe,
		})
	}
	return out
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// ---------- API /api/proxies ----------

// proxyList menyusun daftar proxy siap-pakai (dipakai /api/proxies). Map dipakai
// supaya urutan key JSON sama dengan versi Python (sort_keys) -> byte-identik.
func (s *Server) proxyList(only string) ([]map[string]any, string) {
	set := s.set()
	host := lanIP()
	if set.Listen == "127.0.0.1" {
		host = "127.0.0.1"
	}
	cred := ""
	if set.Auth != "" {
		cred = set.Auth + "@"
	}
	out := make([]map[string]any, 0)
	for _, x := range s.rows() {
		pi := map[string]any{
			"idx":      x.Idx + 1,
			"name":     x.Name,
			"upstream": x.Server,
			"type":     x.Tipe,
		}
		if only != "http" {
			pi["socks"] = map[string]any{"host": host, "port": x.Socks,
				"url": "socks5://" + cred + net.JoinHostPort(host, itoa(x.Socks))}
		}
		if only != "socks" {
			pi["http"] = map[string]any{"host": host, "port": x.HTTP,
				"url": "http://" + cred + net.JoinHostPort(host, itoa(x.HTTP))}
		}
		if set.Tunnel != "" {
			n := itoa(x.Idx + 1)
			pi["public"] = map[string]any{
				"socks":        "socks" + n + "." + set.Tunnel,
				"http":         "http" + n + "." + set.Tunnel,
				"access_socks": "cloudflared access tcp --hostname socks" + n + "." + set.Tunnel + " --url 127.0.0.1:" + itoa(7000+x.Idx+1),
				"access_http":  "cloudflared access tcp --hostname http" + n + "." + set.Tunnel + " --url 127.0.0.1:" + itoa(8000+x.Idx+1),
			}
		}
		out = append(out, pi)
	}
	return out, host
}

// proxyText = format teks: satu URI per baris (siap tempel ke aplikasi).
func (s *Server) proxyText(only string) string {
	list, _ := s.proxyList(only)
	var b strings.Builder
	for _, p := range list {
		name, _ := p["name"].(string)
		if e, ok := p["socks"].(map[string]any); ok {
			url, _ := e["url"].(string)
			b.WriteString(url + "#" + name + "\n")
		}
		if e, ok := p["http"].(map[string]any); ok {
			url, _ := e["url"].(string)
			b.WriteString(url + "#" + name + "\n")
		}
	}
	return b.String()
}

// linkLines sama dengan parsing form "links" di webui.py: buang kosong dan komentar.
func linkLines(raw string) []string {
	out := []string{}
	for _, l := range strings.Split(raw, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, l)
	}
	return out
}

// vlessLinks sama dengan filter urlparse(l).scheme == "vless", ditambah validasi
// minimal (uuid + host) supaya link rusak tidak masuk accounts.txt dan bikin xray gagal start.
func vlessLinks(links []string) []string {
	out := []string{}
	for _, l := range links {
		if config.ValidLink(l) {
			out = append(out, l)
		}
	}
	return out
}
