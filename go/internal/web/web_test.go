package web

// web_test.go: tes hermetic http.Handler (httptest + Deps palsu).
// Tidak butuh network, akun vless asli, atau binary xray/cloudflared.

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vlessbank/internal/config"
	"vlessbank/internal/settings"
	"vlessbank/internal/xray"
)

const (
	testUser = "admin"
	testPass = "rahasia"
	xrayStub = "vlessbank-stub-xray-tidak-ada"
)

type stub struct {
	settings settings.Settings
	saved    []settings.Settings
	added    [][]string
	deleted  []int
	regen    int
	ipPorts  []int
}

type env struct {
	srv *Server
	st  *stub
	mgr *xray.Manager
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()

	acc := filepath.Join(dir, "accounts.txt")
	body := "# komentar\n" +
		"vless://11111111-1111-1111-1111-111111111111@example.com:443?type=tcp&security=tls#akun-a\n" +
		"\n" +
		"bukan-vless\n" +
		"vless://22222222-2222-2222-2222-222222222222@example.org:8443?type=ws&security=tls&path=%2Fws#akun-b\n"
	if err := os.WriteFile(acc, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	loginPath := filepath.Join(dir, "webui.login")
	if err := settings.SaveLogin(loginPath, testUser, testPass); err != nil {
		t.Fatal(err)
	}
	login, err := settings.LoadLogin(loginPath)
	if err != nil {
		t.Fatal(err)
	}

	st := &stub{settings: settings.Settings{
		Xray: "xray", Base: 1081, BaseHTTP: 0, Listen: "127.0.0.1",
		Auth: "", Tunnel: "", TunnelID: "", XrayPath: "xray", Insecure: false,
	}}
	mgr := xray.New(filepath.Join(dir, "config.json"), filepath.Join(dir, "xray.log"),
		xrayStub, func() string { return xrayStub })

	srv := New(Deps{
		Dir:         dir,
		DefaultXray: xrayStub,
		Settings:    func() settings.Settings { return st.settings },
		SaveSettings: func(s settings.Settings) error {
			st.settings = s
			st.saved = append(st.saved, s)
			return nil
		},
		Login:        login,
		Sessions:     settings.NewSessions(filepath.Join(dir, "webui.sessions")),
		ClientTokens: settings.NewClientTokens(filepath.Join(dir, "webui.client-tokens.json")),
		Runtime:      settings.NewRuntime(filepath.Join(dir, "webui.runtime.json")),
		Xray:         mgr,
		Accounts: func() []config.Account {
			accs, _, _ := config.ParseFile(acc)
			return accs
		},
		Binds: func() []config.Bind { return nil },
		Regen: func() string {
			st.regen++
			return "config digenerate"
		},
		AddAccounts: func(links []string) map[string]any {
			st.added = append(st.added, links)
			return map[string]any{"added": len(links), "skipped": 0, "duplicates": 0}
		},
		DeleteAccount: func(line int) bool {
			st.deleted = append(st.deleted, line)
			return line == 2
		},
		IPTest: func(port int) string {
			st.ipPorts = append(st.ipPorts, port)
			if !mgr.Running() {
				return "xray mati"
			}
			return "1.2.3.4 · ID (Indonesia)"
		},
		IPAll: func() map[string]string { return map[string]string{} },
	})
	return &env{srv: srv, st: st, mgr: mgr}
}

func basic(u, p string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(u+":"+p))
}

func req(t *testing.T, s *Server, method, target, body string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	rq := httptest.NewRequest(method, target, r)
	if body != "" {
		rq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range hdr {
		rq.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, rq)
	return w
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("body bukan JSON: %v (%s)", err, w.Body.String())
	}
	return m
}

func tokenFrom(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	raw := w.Header().Get("Set-Cookie")
	if !strings.HasPrefix(raw, "vb_session=") {
		t.Fatalf("Set-Cookie tidak berisi vb_session: %q", raw)
	}
	return strings.SplitN(strings.TrimPrefix(raw, "vb_session="), ";", 2)[0]
}

func sessionHeader(w *httptest.ResponseRecorder) map[string]string {
	raw := w.Header().Get("Set-Cookie")
	return map[string]string{"Cookie": strings.SplitN(raw, ";", 2)[0]}
}

func TestUnauthedRootShowsPublicStatusPage(t *testing.T) {
	e := newEnv(t)
	w := req(t, e.srv, http.MethodGet, "/", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, mau 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("content-type = %q", ct)
	}
	body := w.Body.String()
	for _, want := range []string{"VLESS BANK", `href="/admin"`, "2 akun aktif", `<title>Vlessbank</title>`} {
		if !strings.Contains(body, want) {
			t.Fatalf("halaman status tidak memuat %q:\n%s", want, body)
		}
	}
}

func TestUnauthedAdminShowsLoginPageAndAPI401(t *testing.T) {
	e := newEnv(t)
	for _, path := range []string{"/admin", "/admin/log", "/admin/settings", "/login"} {
		w := req(t, e.srv, http.MethodGet, path, "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s code = %d, mau 200", path, w.Code)
		}
		body := w.Body.String()
		for _, want := range []string{"<title>Login, Vlessbank</title>", `id="u"`, "Masuk"} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s bukan halaman login (tidak ada %q)", path, want)
			}
		}
	}
	for _, path := range []string{"/api/status", "/api/accounts", "/api/ipall", "/api/cf/status"} {
		w := req(t, e.srv, http.MethodGet, path, "", nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s code = %d, mau 401", path, w.Code)
		}
		if got := decodeJSON(t, w)["error"]; got != "unauthorized" {
			t.Fatalf("%s error = %v", path, got)
		}
	}
}

func TestLoginSetsCookieAndAuthFlow(t *testing.T) {
	e := newEnv(t)
	w := req(t, e.srv, http.MethodPost, "/api/login", "", map[string]string{"Authorization": basic(testUser, testPass)})
	if w.Code != http.StatusOK {
		t.Fatalf("login code = %d body=%s", w.Code, w.Body.String())
	}
	j := decodeJSON(t, w)
	if j["ok"] != true || j["user"] != testUser || j["expires_days"] != float64(30) {
		t.Fatalf("body login = %v", j)
	}
	ck := w.Header().Get("Set-Cookie")
	if !strings.Contains(ck, "Max-Age=2592000") || !strings.Contains(ck, "HttpOnly") || !strings.Contains(ck, "SameSite=Lax") {
		t.Fatalf("Set-Cookie = %q", ck)
	}

	hdr := sessionHeader(w)
	w = req(t, e.srv, http.MethodGet, "/api/status", "", hdr)
	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d", w.Code)
	}
	j = decodeJSON(t, w)
	if _, ok := j["xray"].(bool); !ok {
		t.Fatalf("xray bukan bool: %v", j["xray"])
	}
	if j["base"] != float64(1081) || j["listen"] != "127.0.0.1" || j["accounts"] != float64(2) {
		t.Fatalf("body status = %v", j)
	}

	for _, path := range []string{"/", "/login"} {
		w = req(t, e.srv, http.MethodGet, path, "", hdr)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/admin" {
			t.Fatalf("%s code=%d loc=%q, mau 303 /admin", path, w.Code, w.Header().Get("Location"))
		}
	}
}

func TestLogoutDropsSession(t *testing.T) {
	e := newEnv(t)
	w := req(t, e.srv, http.MethodPost, "/api/login", "", map[string]string{"Authorization": basic(testUser, testPass)})
	hdr := sessionHeader(w)
	tok := tokenFrom(t, w)

	w = req(t, e.srv, http.MethodGet, "/logout", "", hdr)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login" {
		t.Fatalf("logout code=%d loc=%q", w.Code, w.Header().Get("Location"))
	}
	ck := w.Header().Get("Set-Cookie")
	if !strings.Contains(ck, "vb_session=;") || !strings.Contains(ck, "Max-Age=0") {
		t.Fatalf("Set-Cookie logout = %q", ck)
	}

	w = req(t, e.srv, http.MethodGet, "/api/status", "", map[string]string{"Cookie": "vb_session=" + tok})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("token lama masih valid (code=%d)", w.Code)
	}
}

func TestBasicAuthAcceptedPerRequest(t *testing.T) {
	e := newEnv(t)
	a := map[string]string{"Authorization": basic(testUser, testPass)}
	if w := req(t, e.srv, http.MethodGet, "/api/accounts", "", a); w.Code != http.StatusOK {
		t.Fatalf("basic auth code = %d", w.Code)
	}
	bad := map[string]string{"Authorization": basic(testUser, "salah")}
	if w := req(t, e.srv, http.MethodGet, "/api/accounts", "", bad); w.Code != http.StatusUnauthorized {
		t.Fatalf("password salah code = %d", w.Code)
	}
	if w := req(t, e.srv, http.MethodPost, "/api/login", "", bad); w.Code != http.StatusUnauthorized {
		t.Fatalf("login salah code = %d", w.Code)
	}
	if got := decodeJSON(t, req(t, e.srv, http.MethodPost, "/api/login", "", bad))["error"]; got != "salah" {
		t.Fatalf("login salah error = %v", got)
	}
}

func TestXrayRouteReachesManager(t *testing.T) {
	e := newEnv(t)
	a := map[string]string{"Authorization": basic(testUser, testPass)}

	wantStop := e.mgr.Stop()
	w := req(t, e.srv, http.MethodPost, "/api/xray", "act=stop", a)
	if w.Code != http.StatusOK {
		t.Fatalf("stop code = %d", w.Code)
	}
	j := decodeJSON(t, w)
	if j["action"] != "stop" || j["result"] != wantStop {
		t.Fatalf("stop body = %v (mau result %q)", j, wantStop)
	}

	wantStart := e.mgr.Start()
	w = req(t, e.srv, http.MethodPost, "/api/xray", "act=start", a)
	j = decodeJSON(t, w)
	if j["action"] != "start" || j["result"] != wantStart {
		t.Fatalf("start body = %v (mau result %q)", j, wantStart)
	}

	wantRestart := e.mgr.Stop()
	w = req(t, e.srv, http.MethodPost, "/api/xray", "act=restart", a)
	if j := decodeJSON(t, w); j["action"] != "restart" || j["result"] == "" {
		t.Fatalf("restart body = %v (stop dulu: %q)", j, wantRestart)
	}

	w = req(t, e.srv, http.MethodPost, "/api/xray", "act=ngawur", a)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("act ngawur code = %d", w.Code)
	}
	if got := decodeJSON(t, w)["error"]; got != "act harus start/stop/restart" {
		t.Fatalf("act ngawur error = %v", got)
	}
}

func TestAccountAddDeleteWiring(t *testing.T) {
	e := newEnv(t)
	a := map[string]string{"Authorization": basic(testUser, testPass)}

	w := req(t, e.srv, http.MethodPost, "/api/add", "links="+urlEncode("vless://33333333-3333-3333-3333-333333333333@example.net:443?type=ws#akun-baru\nbogus"), a)
	if w.Code != http.StatusOK {
		t.Fatalf("add code = %d", w.Code)
	}
	j := decodeJSON(t, w)
	if j["added"] != float64(1) || j["skipped"] != float64(0) || j["duplicates"] != float64(0) {
		t.Fatalf("add body = %v", j)
	}
	if len(e.st.added) != 1 || len(e.st.added[0]) != 1 || e.st.added[0][0] != "vless://33333333-3333-3333-3333-333333333333@example.net:443?type=ws#akun-baru" {
		t.Fatalf("AddAccounts dipanggil dengan %v", e.st.added)
	}

	// tidak ada link vless: dep tidak boleh dipanggil ("# c" dibuang lebih dulu, sama seperti webui.py)
	w = req(t, e.srv, http.MethodPost, "/api/add", "links="+urlEncode("# c\nbogus"), a)
	j = decodeJSON(t, w)
	if j["added"] != float64(0) || j["skipped"] != float64(1) || j["duplicates"] != float64(0) {
		t.Fatalf("add tanpa vless body = %v", j)
	}
	if len(e.st.added) != 1 {
		t.Fatalf("AddAccounts tidak boleh dipanggil: %v", e.st.added)
	}

	w = req(t, e.srv, http.MethodPost, "/api/delete", "line=2", a)
	if w.Code != http.StatusOK {
		t.Fatalf("delete code = %d", w.Code)
	}
	if j = decodeJSON(t, w); j["deleted"] != float64(2) {
		t.Fatalf("delete body = %v", j)
	}
	if len(e.st.deleted) != 1 || e.st.deleted[0] != 2 {
		t.Fatalf("DeleteAccount dipanggil dengan %v", e.st.deleted)
	}

	w = req(t, e.srv, http.MethodPost, "/api/delete", "line=99", a)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("delete invalid code = %d", w.Code)
	}
	if j = decodeJSON(t, w); j["error"] != "line tidak valid" {
		t.Fatalf("delete invalid body = %v", j)
	}
	w = req(t, e.srv, http.MethodPost, "/api/delete", "line=abc", a)
	if e.st.deleted[len(e.st.deleted)-1] != -1 {
		t.Fatalf("line=abc harus jadi -1: %v", e.st.deleted)
	}
}

func TestRegenRoute(t *testing.T) {
	e := newEnv(t)
	w := req(t, e.srv, http.MethodPost, "/api/regen", "", map[string]string{"Authorization": basic(testUser, testPass)})
	if j := decodeJSON(t, w); j["result"] != "config digenerate" || e.st.regen != 1 {
		t.Fatalf("regen body = %v, calls = %d", j, e.st.regen)
	}
}

func TestAdminPagesRender(t *testing.T) {
	e := newEnv(t)
	a := map[string]string{"Authorization": basic(testUser, testPass)}

	// regresi: div tidak boleh ada yang berlebih/kurang (dulu </div> ekstra per
	// akun bikin grid & card ketutup lebih awal -> halaman rusak).
	for _, path := range []string{"/admin", "/admin/akun", "/admin/log", "/admin/settings", "/admin/cf"} {
		body := req(t, e.srv, http.MethodGet, path, "", a).Body.String()
		open := strings.Count(body, "<div")
		closed := strings.Count(body, "</div>")
		if open != closed {
			t.Fatalf("%s: <div> %d vs </div> %d (tidak seimbang)", path, open, closed)
		}
	}

	w := req(t, e.srv, http.MethodGet, "/admin", "", a)
	if w.Code != http.StatusOK {
		t.Fatalf("dashboard code = %d", w.Code)
	}
	for _, want := range []string{"Ringkasan", "Kelola Akun", "Tunnel", "1081", `class="menu"`, "/admin/akun"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("dashboard tidak memuat %q", want)
		}
	}
	if strings.Contains(w.Body.String(), "Daftar Akun") {
		t.Fatal("dashboard masih memuat daftar akun (harusnya di halaman Akun)")
	}

	w = req(t, e.srv, http.MethodGet, "/admin/akun", "", a)
	if w.Code != http.StatusOK {
		t.Fatalf("halaman akun code = %d", w.Code)
	}
	for _, want := range []string{"Daftar Akun", "akun-a", "akun-b", "del(1,'akun-a')", "del(4,'akun-b')",
		`id="newlinks"`, "<h1>Akun</h1>", "socks5://", "http://", "Test Semua IP", "Tambah Akun"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatalf("halaman akun tidak memuat %q", want)
		}
	}

	w = req(t, e.srv, http.MethodGet, "/admin/log?msg=halo", "", a)
	if body := w.Body.String(); !strings.Contains(body, "Log Xray") ||
		!strings.Contains(body, `class="flash">halo</div>`) || !strings.Contains(body, "log kosong") {
		t.Fatalf("halaman log salah:\n%s", body)
	}

	w = req(t, e.srv, http.MethodGet, "/admin/settings", "", a)
	if body := w.Body.String(); !strings.Contains(body, `name="base" type="number" value="1081"`) ||
		!strings.Contains(body, `<option value="127.0.0.1" selected>`) ||
		!strings.Contains(body, `value="admin"`) {
		t.Fatalf("halaman pengaturan salah:\n%s", body)
	}

	w = req(t, e.srv, http.MethodGet, "/log", "", a)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/admin/log" {
		t.Fatalf("/log code=%d loc=%q", w.Code, w.Header().Get("Location"))
	}
	w = req(t, e.srv, http.MethodGet, "/settings", "", a)
	if w.Header().Get("Location") != "/admin/settings" {
		t.Fatalf("/settings loc=%q", w.Header().Get("Location"))
	}
	// path tak dikenal (sudah login) -> halaman status publik
	w = req(t, e.srv, http.MethodGet, "/entah", "", a)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "VLESS BANK") {
		t.Fatalf("/entah code=%d", w.Code)
	}
}

func TestAPIIPAndAccounts(t *testing.T) {
	e := newEnv(t)
	a := map[string]string{"Authorization": basic(testUser, testPass)}

	w := req(t, e.srv, http.MethodGet, "/api/accounts", "", a)
	var res struct {
		Accounts []struct {
			Line   int    `json:"line"`
			Idx    int    `json:"idx"`
			Socks  int    `json:"port_socks"`
			HTTP   int    `json:"port_http"`
			Name   string `json:"name"`
			Server string `json:"server"`
			Tipe   string `json:"tipe"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Accounts) != 2 {
		t.Fatalf("accounts = %v", res.Accounts)
	}
	got := res.Accounts[0]
	if got.Line != 1 || got.Idx != 0 || got.Socks != 1081 || got.HTTP != 2081 ||
		got.Name != "akun-a" || got.Server != "example.com:443" || got.Tipe != "tcp/tls" {
		t.Fatalf("akun pertama = %+v", got)
	}
	if res.Accounts[1].Line != 4 || res.Accounts[1].Idx != 1 || res.Accounts[1].Socks != 1082 {
		t.Fatalf("akun kedua = %+v", res.Accounts[1])
	}

	// xray tidak jalan di env test -> dep IPTest balas "xray mati" (sama seperti webui.ip_test)
	w = req(t, e.srv, http.MethodGet, "/api/ip?port=1081", "", a)
	j := decodeJSON(t, w)
	if j["port"] != "1081" || j["result"] != "xray mati" || len(e.st.ipPorts) != 1 || e.st.ipPorts[0] != 1081 {
		t.Fatalf("api/ip = %v (dipanggil: %v)", j, e.st.ipPorts)
	}
	w = req(t, e.srv, http.MethodGet, "/ip?port=abc", "", a)
	if w.Body.String() != "xray mati" {
		t.Fatalf("/ip = %q", w.Body.String())
	}
	if w = req(t, e.srv, http.MethodGet, "/api/ipall", "", a); w.Body.String() != "{}" {
		t.Fatalf("/api/ipall = %q", w.Body.String())
	}
}

func TestLegacyFormRoutes(t *testing.T) {
	e := newEnv(t)
	a := map[string]string{"Authorization": basic(testUser, testPass)}

	w := req(t, e.srv, http.MethodPost, "/add", "links="+urlEncode("vless://33333333-3333-3333-3333-333333333333@example.net:443?type=ws#akun-baru\nbogus"), a)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("add code = %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/admin/akun?msg=1%20akun%20ditambah%2C%201%20dilewati%20%28bukan%20vless%29" {
		t.Fatalf("/add location = %q", loc)
	}

	w = req(t, e.srv, http.MethodPost, "/del", "line=2", a)
	if loc := w.Header().Get("Location"); loc != "/admin/akun?msg=akun%20dihapus" {
		t.Fatalf("/del location = %q", loc)
	}
	w = req(t, e.srv, http.MethodPost, "/del", "line=99", a)
	if loc := w.Header().Get("Location"); loc != "/admin/akun?msg=line%20tidak%20valid" {
		t.Fatalf("/del invalid location = %q", loc)
	}

	for path, want := range map[string]string{
		"/start":   "/?msg=" + pyQuote(e.mgr.Start()),
		"/stop":    "/?msg=" + pyQuote(e.mgr.Stop()),
		"/restart": "/?msg=" + pyQuote(e.mgr.Restart()),
	} {
		w = req(t, e.srv, http.MethodPost, path, "", a)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != want {
			t.Fatalf("%s code=%d loc=%q mau %q", path, w.Code, w.Header().Get("Location"), want)
		}
	}

	w = req(t, e.srv, http.MethodPost, "/regen", "", a)
	if w.Header().Get("Location") != "/?msg=config%20digenerate" {
		t.Fatalf("/regen location = %q", w.Header().Get("Location"))
	}

	// /settings: clamp base + tolak listen publik tanpa auth
	w = req(t, e.srv, http.MethodPost, "/settings",
		"xray_path=xray&base=10&base_http=0&listen=127.0.0.1&auth=&tunnel=&tunnel_id=", a)
	if loc := w.Header().Get("Location"); loc != "/?msg=pengaturan%20disimpan%20-%20Regenerate%20config%20untuk%20efektif" {
		t.Fatalf("/settings location = %q", loc)
	}
	if e.st.settings.Base != 1024 || e.st.settings.BaseHTTP != 0 {
		t.Fatalf("settings tersimpan = %+v", e.st.settings)
	}
	w = req(t, e.srv, http.MethodPost, "/settings",
		"xray_path=xray&base=1081&base_http=2081&listen=0.0.0.0&auth=&tunnel=&tunnel_id=", a)
	if loc := w.Header().Get("Location"); loc != "/?msg=ditolak%3A%20listen%20publik%20wajib%20auth" {
		t.Fatalf("/settings publik location = %q", loc)
	}
	if e.st.settings.Listen == "0.0.0.0" {
		t.Fatalf("settings publik tidak boleh tersimpan: %+v", e.st.settings)
	}
}

func TestPassRouteChangesPassword(t *testing.T) {
	e := newEnv(t)
	a := map[string]string{"Authorization": basic(testUser, testPass)}

	w := req(t, e.srv, http.MethodPost, "/pass", "user=admin&pass=abc", a)
	if loc := w.Header().Get("Location"); loc != "/?msg=password%20minimal%204%20karakter" {
		t.Fatalf("pass pendek location = %q", loc)
	}
	w = req(t, e.srv, http.MethodPost, "/pass", "user=&pass=", a)
	if loc := w.Header().Get("Location"); loc != "/?msg=username%20tidak%20boleh%20kosong" {
		t.Fatalf("pass tanpa username location = %q", loc)
	}

	w = req(t, e.srv, http.MethodPost, "/pass", "user=admin&pass=passbaru123", a)
	if loc := w.Header().Get("Location"); loc != "/?msg=login%20disimpan" {
		t.Fatalf("pass location = %q", loc)
	}
	// password baru langsung berlaku (login reload dari file)
	w = req(t, e.srv, http.MethodPost, "/api/login", "",
		map[string]string{"Authorization": basic(testUser, "passbaru123")})
	if w.Code != http.StatusOK {
		t.Fatalf("login password baru code = %d", w.Code)
	}
	w = req(t, e.srv, http.MethodPost, "/api/login", "",
		map[string]string{"Authorization": basic(testUser, testPass)})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("password lama masih bisa (code=%d)", w.Code)
	}

	// ganti username saja -> password lama tetap berlaku
	w = req(t, e.srv, http.MethodPost, "/pass", "user=bos&pass=",
		map[string]string{"Authorization": basic(testUser, "passbaru123")})
	if loc := w.Header().Get("Location"); loc != "/?msg=login%20disimpan" {
		t.Fatalf("rename location = %q", loc)
	}
	w = req(t, e.srv, http.MethodPost, "/api/login", "",
		map[string]string{"Authorization": basic("bos", "passbaru123")})
	if w.Code != http.StatusOK {
		t.Fatalf("login user baru code = %d", w.Code)
	}
}

func TestUnauthedPostRoutes(t *testing.T) {
	e := newEnv(t)
	for _, path := range []string{"/api/add", "/api/delete", "/api/xray", "/api/regen", "/add", "/settings"} {
		w := req(t, e.srv, http.MethodPost, path, "links=vless://a", nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s code = %d, mau 401", path, w.Code)
		}
		if got := decodeJSON(t, w)["error"]; got != "unauthorized" {
			t.Fatalf("%s error = %v", path, got)
		}
	}
	if len(e.st.added) != 0 || e.st.regen != 0 {
		t.Fatalf("dep tidak boleh terpanggil: %v %d", e.st.added, e.st.regen)
	}
}

func TestUnknownPostIs404(t *testing.T) {
	e := newEnv(t)
	w := req(t, e.srv, http.MethodPost, "/entah", "", map[string]string{"Authorization": basic(testUser, testPass)})
	if w.Code != http.StatusNotFound || w.Body.String() != "?" {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
}

func urlEncode(s string) string { return url.QueryEscape(s) }

func TestAPIProxies(t *testing.T) {
	e := newEnv(t)
	a := map[string]string{"Authorization": basic(testUser, testPass)}

	// tanpa auth -> 401
	if w := req(t, e.srv, http.MethodGet, "/api/proxies", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth code = %d", w.Code)
	}

	w := req(t, e.srv, http.MethodGet, "/api/proxies", "", a)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	var body struct {
		Count   int `json:"count"`
		Host    string
		Proxies []struct {
			Idx   int    `json:"idx"`
			Name  string `json:"name"`
			Socks struct {
				Host string `json:"host"`
				Port int    `json:"port"`
				URL  string `json:"url"`
			} `json:"socks"`
			HTTP struct {
				Port int    `json:"port"`
				URL  string `json:"url"`
			} `json:"http"`
		} `json:"proxies"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Count != 2 || len(body.Proxies) != 2 {
		t.Fatalf("count = %d / %d", body.Count, len(body.Proxies))
	}
	p := body.Proxies[0]
	if p.Idx != 1 || p.Socks.Port != 1081 || p.HTTP.Port != 2081 || p.Socks.URL == "" {
		t.Fatalf("proxy pertama = %+v", p)
	}
	if !strings.Contains(p.Socks.URL, "socks5://") || !strings.Contains(p.Socks.URL, ":1081") {
		t.Fatalf("url socks = %q", p.Socks.URL)
	}

	// format teks: satu URI per baris
	w = req(t, e.srv, http.MethodGet, "/api/proxies?format=text", "", a)
	txt := w.Body.String()
	if !strings.Contains(txt, "socks5://") || !strings.Contains(txt, "http://") {
		t.Fatalf("text = %q", txt)
	}
	if lines := strings.Count(strings.TrimSpace(txt), "\n") + 1; lines != 4 {
		t.Fatalf("baris teks = %d (%q)", lines, txt)
	}

	// only=socks: http dikosongkan
	w = req(t, e.srv, http.MethodGet, "/api/proxies?only=socks", "", a)
	if strings.Contains(w.Body.String(), `"http"`) {
		t.Fatalf("only=socks masih memuat http: %s", w.Body.String())
	}
}

func TestClientProxyAPIOnlyReturnsPublicEndpoints(t *testing.T) {
	e := newEnv(t)
	e.st.settings.Tunnel = "proxy.example.com"
	store := settings.NewClientTokens(filepath.Join(t.TempDir(), "webui.client-tokens.json"))
	raw, _, err := store.Create("camoufox", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	e.srv.d.ClientTokens = store
	w := req(t, e.srv, http.MethodGet, "/api/client/proxies", "", map[string]string{"Authorization": "Bearer " + raw})
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "upstream") || strings.Contains(w.Body.String(), "access_socks") || strings.Contains(w.Body.String(), "127.0.0.1") {
		t.Fatalf("restricted response leaked: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "socks_host") || !strings.Contains(w.Body.String(), "http_host") {
		t.Fatalf("missing public endpoints: %s", w.Body.String())
	}
}

func TestAdminOperationsAPIs(t *testing.T) {
	e := newEnv(t)
	e.st.settings.Tunnel = "proxy.example.com"
	admin := map[string]string{"Authorization": basic(testUser, testPass)}
	w := req(t, e.srv, http.MethodPost, "/api/client-tokens", "label=camoufox&hours=24", admin)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"token"`) {
		t.Fatalf("create token: %d %s", w.Code, w.Body.String())
	}
	if w = req(t, e.srv, http.MethodGet, "/api/health", "", admin); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"tunnel"`) {
		t.Fatalf("health: %d %s", w.Code, w.Body.String())
	}
	if w = req(t, e.srv, http.MethodGet, "/api/backup", "", admin); w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Type"), "application/zip") {
		t.Fatalf("backup: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	if w = req(t, e.srv, http.MethodGet, "/api/health", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthed health: %d", w.Code)
	}
}

func TestClientTokenListAndRevokeRequireAdmin(t *testing.T) {
	e := newEnv(t)
	admin := map[string]string{"Authorization": basic(testUser, testPass)}
	w := req(t, e.srv, http.MethodPost, "/api/client-tokens", "label=camoufox&hours=24", admin)
	if w.Code != http.StatusOK {
		t.Fatalf("create: %d", w.Code)
	}
	var body struct {
		Record struct {
			ID string `json:"id"`
		} `json:"record"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w = req(t, e.srv, http.MethodGet, "/api/client-tokens", "", admin); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), body.Record.ID) {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	if w = req(t, e.srv, http.MethodPost, "/api/client-tokens/revoke", "id="+body.Record.ID, admin); w.Code != http.StatusOK {
		t.Fatalf("revoke: %d %s", w.Code, w.Body.String())
	}
	if w = req(t, e.srv, http.MethodGet, "/api/client-tokens", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthed list: %d", w.Code)
	}
}

func TestClientProxyAPIFiltersTokenScope(t *testing.T) {
	e := newEnv(t)
	e.st.settings.Tunnel = "proxy.example.com"
	raw, _, err := e.srv.d.ClientTokens.CreateScoped("account-two", time.Hour, []int{2})
	if err != nil {
		t.Fatal(err)
	}
	w := req(t, e.srv, http.MethodGet, "/api/client/proxies", "", map[string]string{"Authorization": "Bearer " + raw})
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), `"idx":1`) || !strings.Contains(w.Body.String(), `"idx":2`) {
		t.Fatalf("scope response: %d %s", w.Code, w.Body.String())
	}
}
