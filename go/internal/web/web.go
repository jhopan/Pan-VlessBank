package web

// web.go: panel HTTP vlessbank, port 1:1 dari webui.py (routing, auth, JSON, redirect).

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"vlessbank/internal/config"
	"vlessbank/internal/ops"
	"vlessbank/internal/settings"
	"vlessbank/internal/xray"
)

// Deps adalah seluruh ketergantungan panel (lihat kontrak).
type Deps struct {
	Dir           string // HERE: folder accounts.txt, config.json, ...
	Port          int    // port panel aktual, target loopback tunnel WebUI
	DefaultXray   string // default flag --xray
	Settings      func() settings.Settings
	SaveSettings  func(settings.Settings) error
	Login         settings.Login
	Sessions      *settings.Sessions
	ClientTokens  *settings.ClientTokens
	Xray          *xray.Manager
	Accounts      func() []config.Account
	Binds         func() []config.Bind
	Regen         func() string
	AddAccounts   func(links []string) map[string]any
	DeleteAccount func(line int) bool
	IPTest        func(port int) string
	IPAll         func() map[string]string
}

// Server mengimplementasikan http.Handler.
type Server struct {
	d Deps

	loginMu sync.RWMutex
	login   settings.Login
}

func New(d Deps) *Server {
	return &Server{d: d, login: d.Login}
}

// ---------- path & setting ----------

func (s *Server) set() settings.Settings { return s.d.Settings() }

func (s *Server) path(name string) string { return filepath.Join(s.d.Dir, name) }

func (s *Server) cfgPath() string    { return s.path("config.json") }
func (s *Server) tunPath() string    { return s.path("config.tunnel.yml") }
func (s *Server) webTunPath() string { return s.path("webui.tunnel.yml") }
func (s *Server) loginPath() string  { return s.path("webui.login") }
func (s *Server) accPath() string    { return s.path("accounts.txt") }

func (s *Server) auditPath() string { return s.path("webui.audit.jsonl") }

func (s *Server) audit(action string)  { _ = ops.AppendAudit(s.auditPath(), action, "") }
func (s *Server) settingsPath() string { return s.path("webui.settings.json") }
func (s *Server) xrayRunning() bool    { return s.d.Xray.Running() }

// cfCredDir sama dengan CF.CF_DIR di cf.py: ~/.cloudflared.
func (s *Server) cfCredDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".cloudflared"
	}
	return filepath.Join(home, ".cloudflared")
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// ---------- auth ----------

// sessionToken sama dengan _cookie().get("vb_session", ""), duplikat terakhir menang.
func (s *Server) sessionToken(r *http.Request) string {
	tok := ""
	for _, c := range r.Cookies() {
		if c.Name == "vb_session" {
			tok = c.Value
		}
	}
	return tok
}

func (s *Server) basicUser(r *http.Request) (string, string, bool) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Basic ") {
		return "", "", false
	}
	raw := strings.TrimSpace(h[len("Basic "):])
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(raw)
		if err != nil {
			return "", "", false
		}
	}
	u, p, ok := strings.Cut(string(b), ":")
	if !ok {
		return "", "", false
	}
	return u, p, true
}

func (s *Server) currentLogin() settings.Login {
	s.loginMu.RLock()
	defer s.loginMu.RUnlock()
	return s.login
}

// checkUser sama dengan check_login(user, pw) di webui.py.
func (s *Server) checkUser(user, pass string) bool {
	return settings.CheckLogin(s.currentLogin(), user, pass)
}

// authOK sama dengan H._auth_ok(): Basic header ATAU cookie sesi.
func (s *Server) authOK(r *http.Request) bool {
	if u, p, ok := s.basicUser(r); ok {
		return s.checkUser(u, p)
	}
	tok := s.sessionToken(r)
	if tok == "" {
		return false
	}
	return s.d.Sessions.Check(tok)
}

func (s *Server) clientTokenOK(r *http.Request) bool {
	if s.d.ClientTokens == nil {
		return false
	}
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return false
	}
	return s.d.ClientTokens.Check(strings.TrimSpace(strings.TrimPrefix(h, "Bearer ")))
}

// ---------- writer ----------

func writeHTML(w http.ResponseWriter, body string, code int) {
	b := []byte(body)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(code)
	_, _ = w.Write(b)
}

func jsonBytes(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return []byte("null")
	}
	return bytes.TrimRight(buf.Bytes(), "\n")
}

func writeJSON(w http.ResponseWriter, v any, code int) {
	b := jsonBytes(v)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(code)
	_, _ = w.Write(b)
}

func redirect(w http.ResponseWriter, loc string) {
	w.Header().Set("Location", loc)
	w.WriteHeader(http.StatusSeeOther)
}

// redir sama dengan H._redir(msg): Location "/" + ?msg=quote(msg).
func redir(w http.ResponseWriter, msg string) {
	redirPath(w, "/", msg)
}

// redirPath mengalihkan ke path tertentu + ?msg=quote(msg) (dipakai halaman Akun).
func redirPath(w http.ResponseWriter, path, msg string) {
	loc := path
	if msg != "" {
		sep := "?"
		if strings.Contains(loc, "?") {
			sep = "&"
		}
		loc += sep + "msg=" + pyQuote(msg)
	}
	redirect(w, loc)
}

// form sama dengan H._form(): parse_qs(body), kosong dianggap tidak ada (keep_blank_values=False).
func form(r *http.Request) url.Values {
	b, _ := io.ReadAll(r.Body)
	v, err := url.ParseQuery(string(b))
	if err != nil {
		return url.Values{}
	}
	return v
}

func fval(f url.Values, key, def string) string {
	if vs, ok := f[key]; ok && len(vs) > 0 && vs[0] != "" {
		return vs[0]
	}
	return def
}

// fvalExact keeps an explicit empty value (Python: f.get(key, [def])[0]).
// Dipakai subdomain: "" berarti hostname 1 level, bukan default "socks".
func fvalExact(f url.Values, key, def string) string {
	if vs, ok := f[key]; ok && len(vs) > 0 {
		return vs[0]
	}
	return def
}

// ---------- ip test ----------

func (s *Server) ipTest(port string) string {
	if !isDigits(port) {
		if !s.xrayRunning() {
			return "xray mati"
		}
		return "?"
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return "?"
	}
	return s.d.IPTest(n)
}

func (s *Server) ipAll() map[string]string {
	if !s.xrayRunning() {
		return map[string]string{}
	}
	return s.d.IPAll()
}
