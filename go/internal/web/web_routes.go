package web

// web_routes.go: do_GET/do_POST dari webui.py (urutan route sama persis).

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"vlessbank/internal/cf"
	"vlessbank/internal/ops"
	"vlessbank/internal/settings"
)

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Jangan biarkan panic jadi "empty reply" (client cuma lihat error tanpa pesan).
	defer func() {
		if rec := recover(); rec != nil {
			fmt.Fprintf(os.Stderr, "PANIC %s %s: %v\n%s\n", r.Method, r.URL.Path, rec, debug.Stack())
			writeJSON(w, map[string]any{"error": fmt.Sprintf("internal error: %v", rec)},
				http.StatusInternalServerError)
		}
	}()
	switch r.Method {
	case http.MethodGet:
		s.doGet(w, r)
	case http.MethodPost:
		s.doPost(w, r)
	default:
		// BaseHTTPRequestHandler default: method lain -> 501.
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func (s *Server) doGet(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if path == "/logout" {
		s.d.Sessions.Drop(s.sessionToken(r))
		w.Header().Set("Set-Cookie", "vb_session=; Path=/; Max-Age=0; HttpOnly; SameSite=Lax")
		redirect(w, "/login")
		return
	}

	if path == "/api/client/proxies" {
		if !s.clientTokenOK(r) {
			writeJSON(w, map[string]any{"error": "unauthorized"}, http.StatusUnauthorized)
			return
		}
		writeJSON(w, map[string]any{"proxies": s.clientProxyList()}, http.StatusOK)
		return
	}
	authed := s.authOK(r)
	if (path == "/" || path == "/login") && authed {
		redirect(w, "/admin")
		return
	}
	if path == "/" && !authed {
		writeHTML(w, s.statusPage(), http.StatusOK)
		return
	}
	if path == "/login" || !authed {
		if strings.HasPrefix(path, "/api/") {
			writeJSON(w, map[string]any{"error": "unauthorized"}, http.StatusUnauthorized)
			return
		}
		writeHTML(w, s.loginPage(""), http.StatusOK)
		return
	}

	q := r.URL.Query()
	switch path {
	case "/api/health":
		validation := ops.ReadTunnelValidation(s.tunPath())
		set := s.set()
		proxyTunnel := cf.TunnelRunning() || (set.TunnelID != "" && cf.TunnelConnections(set.TunnelID) > 0)
		webTunnel := cf.WebTunnelRunning() || (set.WebTunnelID != "" && cf.TunnelConnections(set.WebTunnelID) > 0)
		writeJSON(w, ops.Health(s.xrayRunning(), proxyTunnel, webTunnel, fileExists(s.cfgPath()), validation), http.StatusOK)
		return
	case "/api/client-tokens":
		if s.d.ClientTokens == nil {
			writeJSON(w, map[string]any{"tokens": []any{}}, http.StatusOK)
			return
		}
		writeJSON(w, map[string]any{"tokens": s.d.ClientTokens.List()}, http.StatusOK)
		return
	case "/api/audit":
		writeJSON(w, map[string]any{"events": ops.TailAudit(s.auditPath(), 100)}, http.StatusOK)
		return
	case "/api/backup":
		b, err := ops.RedactedBackup(s.d.Dir, len(s.rows()))
		if err != nil {
			writeJSON(w, map[string]any{"error": err.Error()}, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment; filename=pan-vlessbank-backup.zip")
		w.Header().Set("Content-Length", strconv.Itoa(len(b)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
		return
	case "/api/status":
		set := s.set()
		writeJSON(w, map[string]any{
			"xray":     s.xrayRunning(),
			"accounts": len(s.rows()),
			"base":     set.Base,
			"listen":   set.Listen,
			"auth":     set.Auth != "",
			"tunnel":   set.Tunnel,
			"config":   fileExists(s.cfgPath()),
		}, http.StatusOK)
		return
	case "/api/accounts":
		writeJSON(w, map[string]any{"accounts": s.rowsJSON()}, http.StatusOK)
		return
	case "/api/proxies":
		only := q.Get("only") // "", "socks", "http"
		if only != "socks" && only != "http" {
			only = ""
		}
		if q.Get("format") == "text" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, s.proxyText(only))
			return
		}
		list, host := s.proxyList(only)
		set := s.set()
		writeJSON(w, map[string]any{
			"host":    host,
			"auth":    set.Auth != "",
			"listen":  set.Listen,
			"tunnel":  set.Tunnel,
			"count":   len(list),
			"proxies": list,
		}, http.StatusOK)
		return
	case "/api/ip":
		p := q.Get("port")
		writeJSON(w, map[string]any{"port": p, "result": s.ipTest(p)}, http.StatusOK)
		return
	case "/api/ipall":
		writeJSON(w, s.ipAll(), http.StatusOK)
		return
	case "/ip":
		writeHTML(w, s.ipTest(q.Get("port")), http.StatusOK)
		return
	case "/ipall":
		writeHTML(w, string(jsonBytes(s.ipAll())), http.StatusOK)
		return
	case "/api/cf/dlstatus":
		writeJSON(w, cf.DownloadStatus(), http.StatusOK)
		return
	case "/api/cf/loginstatus":
		writeJSON(w, cf.LoginStatus(), http.StatusOK)
		return
	case "/api/cf/logstatus":
		writeJSON(w, map[string]any{"log": cf.TailLog(40), "running": cf.Running()}, http.StatusOK)
		return
	case "/api/cf/status":
		writeJSON(w, cf.Status(), http.StatusOK)
		return
	}

	if strings.HasPrefix(path, "/admin") {
		tab := strings.Trim(strings.TrimPrefix(path, "/admin"), "/")
		if tab == "" {
			tab = "dash"
		}
		msg := q.Get("msg")
		switch tab {
		case "akun":
			writeHTML(w, s.pageWith("akun", msg, s.bodyAkun()), http.StatusOK)
		case "log":
			body := `<div class="card"><h2>Log Xray</h2><pre>` + esc(s.d.Xray.TailLog(80)) + `</pre></div>`
			writeHTML(w, s.pageWith("log", msg, body), http.StatusOK)
		case "settings":
			writeHTML(w, s.pageWith("set", msg, s.bodySettings()), http.StatusOK)
		case "cf":
			writeHTML(w, s.pageWith("cf", msg, s.bodyCF()), http.StatusOK)
		default:
			writeHTML(w, s.page("dash", msg), http.StatusOK)
		}
		return
	}

	// path lama tetap dialihkan biar bookmark lama gak mati
	if path == "/log" || path == "/settings" {
		dst := "/admin/log"
		if path == "/settings" {
			dst = "/admin/settings"
		}
		redirect(w, dst)
		return
	}
	writeHTML(w, s.statusPage(), http.StatusOK)
}

func (s *Server) doPost(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if path == "/api/login" {
		if u, p, ok := s.basicUser(r); ok && s.checkUser(u, p) {
			tok := s.d.Sessions.New(u)
			w.Header().Set("Set-Cookie",
				"vb_session="+tok+"; Path=/; Max-Age=2592000; HttpOnly; SameSite=Lax")
			writeJSON(w, map[string]any{"ok": true, "user": u, "expires_days": 30}, http.StatusOK)
			return
		}
		writeJSON(w, map[string]any{"error": "salah"}, http.StatusUnauthorized)
		return
	}

	if !s.authOK(r) {
		writeJSON(w, map[string]any{"error": "unauthorized"}, http.StatusUnauthorized)
		return
	}

	f := form(r)
	switch path {
	case "/api/client-tokens":
		if s.d.ClientTokens == nil {
			writeJSON(w, map[string]any{"error": "token client belum tersedia"}, http.StatusServiceUnavailable)
			return
		}
		label := strings.TrimSpace(fval(f, "label", ""))
		hours, err := strconv.Atoi(fval(f, "hours", "720"))
		if err != nil || hours < 1 || hours > 24*365 {
			writeJSON(w, map[string]any{"error": "masa token harus 1 sampai 8760 jam"}, http.StatusBadRequest)
			return
		}
		raw, record, err := s.d.ClientTokens.Create(label, time.Duration(hours)*time.Hour)
		if err != nil {
			writeJSON(w, map[string]any{"error": err.Error()}, http.StatusBadRequest)
			return
		}
		s.audit("client_token.create")
		writeJSON(w, map[string]any{"token": raw, "record": record}, http.StatusOK)
		return
	case "/api/client-tokens/revoke":
		if s.d.ClientTokens == nil {
			writeJSON(w, map[string]any{"error": "token client belum tersedia"}, http.StatusServiceUnavailable)
			return
		}
		if err := s.d.ClientTokens.Revoke(strings.TrimSpace(fval(f, "id", ""))); err != nil {
			writeJSON(w, map[string]any{"error": err.Error()}, http.StatusBadRequest)
			return
		}
		s.audit("client_token.revoke")
		writeJSON(w, map[string]any{"ok": true}, http.StatusOK)
		return
	case "/api/add":
		links := linkLines(fval(f, "links", ""))
		ok := vlessLinks(links)
		if len(ok) > 0 {
			writeJSON(w, s.d.AddAccounts(ok), http.StatusOK)
			return
		}
		writeJSON(w, map[string]any{"added": 0, "skipped": len(links), "duplicates": 0}, http.StatusOK)
		return

	case "/api/delete":
		line := formInt(fval(f, "line", "-1"))
		if s.d.DeleteAccount(line) {
			writeJSON(w, map[string]any{"deleted": line}, http.StatusOK)
			return
		}
		writeJSON(w, map[string]any{"error": "line tidak valid"}, http.StatusBadRequest)
		return

	case "/api/xray":
		switch act := fval(f, "act", ""); act {
		case "start":
			s.audit("xray.start")
			writeJSON(w, map[string]any{"action": "start", "result": s.d.Xray.Start()}, http.StatusOK)
		case "stop":
			s.audit("xray.stop")
			writeJSON(w, map[string]any{"action": "stop", "result": s.d.Xray.Stop()}, http.StatusOK)
		case "restart":
			s.audit("xray.restart")
			writeJSON(w, map[string]any{"action": "restart", "result": s.d.Xray.Restart()}, http.StatusOK)
		default:
			writeJSON(w, map[string]any{"error": "act harus start/stop/restart"}, http.StatusBadRequest)
		}
		return

	case "/api/regen":
		writeJSON(w, map[string]any{"result": s.d.Regen()}, http.StatusOK)
		return

	case "/api/cf/download":
		cf.DownloadBinary()
		writeJSON(w, map[string]any{"ok": true}, http.StatusOK)
		return

	case "/api/cf/login":
		writeJSON(w, cf.StartLogin(), http.StatusOK)
		return

	case "/api/cf/create":
		name := strings.TrimSpace(fval(f, "name", ""))
		res := cf.CreateTunnel(name)
		if asBool(res["ok"]) {
			set := s.set()
			set.TunnelID = asString(res["id"])
			if !s.saveSettings(w, set) {
				return
			}
		}
		writeJSON(w, res, http.StatusOK)
		return

	case "/api/cf/usetunnel":
		tid := strings.TrimSpace(fval(f, "id", ""))
		name := strings.TrimSpace(fval(f, "name", ""))
		if tid == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "id kosong"}, http.StatusBadRequest)
			return
		}
		set := s.set()
		set.TunnelID = tid
		if !s.saveSettings(w, set) {
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": tid, "name": name}, http.StatusOK)
		return

	case "/api/cf/deltunnel":
		writeJSON(w, cf.DeleteTunnel(strings.TrimSpace(fval(f, "id", ""))), http.StatusOK)
		return

	case "/api/cf/cancelLogin":
		writeJSON(w, cf.CancelLogin(), http.StatusOK)
		return

	case "/api/cf/refreshtunnels":
		tunnels := cf.ListTunnels()
		writeJSON(w, map[string]any{"ok": true, "count": len(tunnels), "tunnels": tunnels}, http.StatusOK)
		return

	case "/api/cf/zones":
		tok := strings.TrimSpace(fval(f, "token", ""))
		if tok == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "token kosong"}, http.StatusBadRequest)
			return
		}
		writeJSON(w, cf.APIZones(tok), http.StatusOK)
		return

	case "/api/cf/genYml":
		set := s.set()
		if set.TunnelID == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "Pilih tunnel dulu (step 3)"}, http.StatusOK)
			return
		}
		zoneName := strings.TrimSpace(fval(f, "zone_name", ""))
		sub := strings.TrimSpace(fvalExact(f, "subdomain", "socks"))
		if zoneName == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "domain kosong"}, http.StatusOK)
			return
		}
		set.Tunnel = zoneName
		if !s.saveSettings(w, set) {
			return
		}
		writeJSON(w, cf.GenerateTunnelYML(set.TunnelID, s.cfCredDir(), s.accPath(),
			s.settingsPath(), zoneName, sub), http.StatusOK)
		return

	case "/api/cf/createDNS":
		tok := strings.TrimSpace(fval(f, "token", ""))
		zoneID := strings.TrimSpace(fval(f, "zone_id", ""))
		zoneName := strings.TrimSpace(fval(f, "zone_name", ""))
		sub := strings.TrimSpace(fvalExact(f, "subdomain", "socks"))
		tid := s.set().TunnelID
		if tok == "" || zoneID == "" || zoneName == "" || tid == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "token/zone/tunnel_id tidak lengkap"}, http.StatusOK)
			return
		}
		writeJSON(w, cf.APICreateDNS(tok, zoneID, zoneName, tid, sub), http.StatusOK)
		return

	case "/api/cf/webtunnel":
		set := s.set()
		switch act := fval(f, "act", ""); act {
		case "create":
			r := cf.CreateTunnel("Pan-VlessBank")
			if asBool(r["ok"]) {
				set.WebTunnelID = asString(r["id"])
				if !s.saveSettings(w, set) {
					return
				}
			}
			writeJSON(w, r, http.StatusOK)
		case "generate":
			if set.WebTunnelID == "" {
				writeJSON(w, map[string]any{"error": "tunnel WebUI belum ada"}, http.StatusBadRequest)
				return
			}
			host := strings.TrimSpace(fval(f, "host", set.WebTunnelHost))
			if host == "" {
				writeJSON(w, map[string]any{"error": "isi hostname penuh, contoh panel.example.com"}, http.StatusBadRequest)
				return
			}
			set.WebTunnelHost = host
			if !s.saveSettings(w, set) {
				return
			}
			writeJSON(w, cf.GenerateWebTunnelYML(set.WebTunnelID, s.cfCredDir(), set.WebTunnelHost, s.d.Port), http.StatusOK)
		case "start":
			writeJSON(w, map[string]any{"result": cf.StartWebTunnel(s.webTunPath())}, http.StatusOK)
		case "stop":
			writeJSON(w, map[string]any{"result": cf.StopWebTunnel()}, http.StatusOK)
		default:
			writeJSON(w, map[string]any{"error": "act harus create/generate/start/stop"}, http.StatusBadRequest)
		}
		return

	case "/api/cf/webdns":
		set := s.set()
		if set.WebTunnelID == "" || set.WebTunnelHost == "" {
			writeJSON(w, map[string]any{"error": "tunnel atau hostname WebUI belum diisi"}, http.StatusBadRequest)
			return
		}
		writeJSON(w, cf.RouteDNS(set.WebTunnelID, set.WebTunnelHost), http.StatusOK)
		return

	case "/api/cf/tunnel":
		switch act := fval(f, "act", ""); act {
		case "start":
			writeJSON(w, map[string]any{"result": cf.Start(s.tunPath())}, http.StatusOK)
		case "stop":
			writeJSON(w, map[string]any{"result": cf.Stop()}, http.StatusOK)
		default:
			writeJSON(w, map[string]any{"error": "act harus start/stop"}, http.StatusBadRequest)
		}
		return
	}

	// form HTML lama (dari dashboard)
	switch path {
	case "/add":
		links := linkLines(fval(f, "links", ""))
		ok := vlessLinks(links)
		bad := len(links) - len(ok)
		if len(ok) > 0 {
			s.d.AddAccounts(ok)
		}
		msg := strconv.Itoa(len(ok)) + " akun ditambah"
		if bad > 0 {
			msg += ", " + strconv.Itoa(bad) + " dilewati (bukan vless)"
		}
		redirPath(w, "/admin/akun", msg)
		return

	case "/del":
		line := formInt(fval(f, "line", "-1"))
		if s.d.DeleteAccount(line) {
			redirPath(w, "/admin/akun", "akun dihapus")
		} else {
			redirPath(w, "/admin/akun", "line tidak valid")
		}
		return

	case "/regen":
		redir(w, s.d.Regen())
		return

	case "/start":
		redir(w, s.d.Xray.Start())
		return

	case "/stop":
		redir(w, s.d.Xray.Stop())
		return

	case "/restart":
		redir(w, s.d.Xray.Restart())
		return

	case "/settings":
		set := s.set()
		set.XrayPath = strings.TrimSpace(fval(f, "xray_path", ""))
		if n, err := strconv.Atoi(fval(f, "base", "1081")); err == nil {
			set.Base = clamp(n, 1024, 65000)
		}
		if bh, err := strconv.Atoi(fval(f, "base_http", "0")); err == nil {
			if bh == 0 {
				set.BaseHTTP = 0
			} else {
				set.BaseHTTP = clamp(bh, 1024, 65000)
			}
		}
		set.Listen = fval(f, "listen", "127.0.0.1")
		set.Auth = strings.TrimSpace(fval(f, "auth", ""))
		set.Insecure = fval(f, "insecure", "") == "1"
		set.Tunnel = strings.TrimSpace(fval(f, "tunnel", ""))
		set.TunnelID = strings.TrimSpace(fval(f, "tunnel_id", ""))
		if set.Listen == "0.0.0.0" && set.Auth == "" {
			redir(w, "ditolak: listen publik wajib auth")
			return
		}
		if !s.saveSettings(w, set) {
			return
		}
		redir(w, "pengaturan disimpan - Regenerate config untuk efektif")
		return

	case "/settings-tunnel":
		set := s.set()
		set.Tunnel = strings.TrimSpace(fval(f, "tunnel", ""))
		set.TunnelID = strings.TrimSpace(fval(f, "tunnel_id", ""))
		if !s.saveSettings(w, set) {
			return
		}
		redir(w, "tunnel disimpan - Regenerate config untuk efektif")
		return

	case "/pass":
		u := strings.TrimSpace(fval(f, "user", ""))
		p := fval(f, "pass", "")
		if u == "" {
			redir(w, "username tidak boleh kosong")
			return
		}
		if p != "" {
			if utf8.RuneCountInString(p) < 4 {
				redir(w, "password minimal 4 karakter")
				return
			}
			if err := settings.SaveLogin(s.loginPath(), u, p); err != nil {
				writeJSON(w, map[string]any{"error": err.Error()}, http.StatusInternalServerError)
				return
			}
			if l, err := settings.LoadLogin(s.loginPath()); err == nil {
				s.loginMu.Lock()
				s.login = l
				s.loginMu.Unlock()
			}
		} else {
			l := s.currentLogin()
			l.User = u
			if err := saveLoginKeep(l, s.loginPath()); err != nil {
				writeJSON(w, map[string]any{"error": err.Error()}, http.StatusInternalServerError)
				return
			}
			s.loginMu.Lock()
			s.login = l
			s.loginMu.Unlock()
		}
		redir(w, "login disimpan")
		return
	}

	writeHTML(w, "?", http.StatusNotFound)
}

// saveSettings menyimpan settings; error ditulis sebagai 500 JSON (Python: exception).
func (s *Server) saveSettings(w http.ResponseWriter, set settings.Settings) bool {
	if err := s.d.SaveSettings(set); err != nil {
		writeJSON(w, map[string]any{"error": err.Error()}, http.StatusInternalServerError)
		return false
	}
	return true
}

func formInt(v string) int {
	n, err := strconv.Atoi(v)
	if err != nil {
		return -1
	}
	return n
}

func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
