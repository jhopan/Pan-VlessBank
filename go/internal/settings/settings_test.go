package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDefaults(t *testing.T) {
	d := Defaults("xray")
	want := Settings{Base: 1081, BaseHTTP: 0, Listen: "127.0.0.1", XrayPath: "xray"}
	if d != want {
		t.Fatalf("Defaults = %+v, want %+v", d, want)
	}
}

func TestLoadSettingsMissingFile(t *testing.T) {
	got := LoadSettings(filepath.Join(t.TempDir(), "webui.settings.json"), "C:/xray.exe")
	if got != Defaults("C:/xray.exe") {
		t.Fatalf("LoadSettings file hilang = %+v", got)
	}
}

// File nyata (versi Python lama) tidak punya key "base_http": key yang hilang
// harus tetap default, bukan nol/impor error.
func TestLoadSettingsSubsetKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.settings.json")
	writeFile(t, path, `{
  "xray": "",
  "base": 1081,
  "listen": "127.0.0.1",
  "auth": "",
  "tunnel": "",
  "tunnel_id": "",
  "xray_path": "C:/Users/ACER/Documents/Project/vlessbank/xray.exe",
  "insecure": true
}`)
	got := LoadSettings(path, "xray")
	if got.BaseHTTP != 0 {
		t.Errorf("BaseHTTP = %d, want 0 (key hilang -> default)", got.BaseHTTP)
	}
	if got.Listen != "127.0.0.1" || got.Base != 1081 {
		t.Errorf("nilai dari file tidak terbaca: %+v", got)
	}
	if got.XrayPath != "C:/Users/ACER/Documents/Project/vlessbank/xray.exe" || !got.Insecure {
		t.Errorf("xray_path/insecure salah: %+v", got)
	}

	// Hanya satu key: sisanya wajib default.
	writeFile(t, path, `{"base": 2081, "insecure": true}`)
	got = LoadSettings(path, "/usr/bin/xray")
	want := Defaults("/usr/bin/xray")
	want.Base = 2081
	want.Insecure = true
	if got != want {
		t.Fatalf("merge key sebagian = %+v, want %+v", got, want)
	}
}

func TestLoadSettingsBrokenOrWeirdFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.settings.json")
	for _, content := range []string{"{oops", "", "[]", `"str"`, `{"base": "1081"}`} {
		writeFile(t, path, content)
		if got := LoadSettings(path, "xray"); got != Defaults("xray") {
			t.Errorf("content %q -> %+v, want default", content, got)
		}
	}
}

// Isi file harus identik byte-per-byte dengan json.dump(DEFAULTS, indent=2) Python:
// urutan key sama, indent 2, tanpa newline penutup.
const pySettingsLiteral = `{
  "xray": "",
  "base": 1081,
  "base_http": 0,
  "listen": "127.0.0.1",
  "auth": "",
  "tunnel": "",
  "tunnel_id": "",
  "xray_path": "xray",
  "insecure": false
}`

func TestSaveSettingsBytesMatchPython(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.settings.json")
	if err := SaveSettings(path, Defaults("xray")); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != pySettingsLiteral {
		t.Fatalf("file != literal Python\n got: %q\nwant: %q", got, pySettingsLiteral)
	}
}

func TestSaveSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.settings.json")
	in := Settings{
		Xray: "x", Base: 1081, BaseHTTP: 2081, Listen: "0.0.0.0",
		Auth: "user:pass", Tunnel: "socks.example.com", TunnelID: "uuid-1",
		XrayPath: "C:/x/xray.exe", Insecure: true,
	}
	if err := SaveSettings(path, in); err != nil {
		t.Fatal(err)
	}
	if got := LoadSettings(path, "xray"); got != in {
		t.Fatalf("round-trip = %+v, want %+v", got, in)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(readFile(t, path)), &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"xray", "base", "base_http", "listen", "auth", "tunnel", "tunnel_id", "xray_path", "insecure"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("key %q hilang dari file", k)
		}
	}
}

// Vektor yang dihitung sendiri dengan Python:
//
//	python -c "import hashlib;print(hashlib.sha256(('admin'+'abc').encode()).hexdigest())"
//	-> 14527447ce1fcb65caca45f72481694ae49d2a8389409b4a4b83eab08ceaffd9
//	python -c "import hashlib;print(hashlib.sha256(('admin'+'8d7b330629853b54').encode()).hexdigest())"
//	-> 8ef170de3da73960a7f05857a4588dc19fdf5998e4381683b0a8e3f760668060
func TestCheckLoginPythonVectors(t *testing.T) {
	cases := []struct {
		salt, hash string
		user, pass string
		want       bool
	}{
		{"abc", "14527447ce1fcb65caca45f72481694ae49d2a8389409b4a4b83eab08ceaffd9", "admin", "admin", true},
		{"8d7b330629853b54", "8ef170de3da73960a7f05857a4588dc19fdf5998e4381683b0a8e3f760668060", "admin", "admin", true},
		{"abc", "14527447ce1fcb65caca45f72481694ae49d2a8389409b4a4b83eab08ceaffd9", "admin", "salah", false},
		{"abc", "14527447ce1fcb65caca45f72481694ae49d2a8389409b4a4b83eab08ceaffd9", "root", "admin", false},
		{"abc", "14527447ce1fcb65caca45f72481694ae49d2a8389409b4a4b83eab08ceaffd9", "admin", "", false},
	}
	for _, c := range cases {
		l := Login{User: "admin", Salt: c.salt, Hash: c.hash}
		if got := CheckLogin(l, c.user, c.pass); got != c.want {
			t.Errorf("CheckLogin(%s,%s) dengan salt %q = %v, want %v", c.user, c.pass, c.salt, got, c.want)
		}
	}
	// Hash Go harus sama dengan hash yang dihitung Python untuk salt yang sama.
	if got := hashPW("admin", "abc"); got != "14527447ce1fcb65caca45f72481694ae49d2a8389409b4a4b83eab08ceaffd9" {
		t.Fatalf("hashPW(\"admin\",\"abc\") = %s", got)
	}
}

// File webui.login hasil Python harus langsung diterima apa adanya.
func TestLoadLoginPythonFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.login")
	writeFile(t, path, "{\n  \"user\": \"admin\",\n  \"salt\": \"8d7b330629853b54\",\n  \"hash\": \"8ef170de3da73960a7f05857a4588dc19fdf5998e4381683b0a8e3f760668060\"\n}")
	l, err := LoadLogin(path)
	if err != nil {
		t.Fatal(err)
	}
	if l.User != "admin" || l.Salt != "8d7b330629853b54" {
		t.Fatalf("LoadLogin = %+v", l)
	}
	if !CheckLogin(l, "admin", "admin") {
		t.Fatal("login admin/admin dari file Python ditolak")
	}
	if readFile(t, path) != "{\n  \"user\": \"admin\",\n  \"salt\": \"8d7b330629853b54\",\n  \"hash\": \"8ef170de3da73960a7f05857a4588dc19fdf5998e4381683b0a8e3f760668060\"\n}" {
		t.Fatal("file yang sudah ada ditulis ulang")
	}
}

func TestLoginAutoCreate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.login")
	l, err := LoadLogin(path)
	if err != nil {
		t.Fatal(err)
	}
	if l.User != "admin" || l.Salt == "" || l.Hash == "" {
		t.Fatalf("auto-create = %+v", l)
	}
	if len(l.Salt) != 16 || strings.Trim(l.Salt, "0123456789abcdef") != "" {
		t.Fatalf("salt harus 16 hex, dapat %q", l.Salt)
	}
	if l.Hash != hashPW("admin", l.Salt) {
		t.Fatalf("hash bukan sha256(\"admin\"+salt): %s", l.Hash)
	}
	if !CheckLogin(l, "admin", "admin") {
		t.Fatal("admin/admin ditolak")
	}
	if CheckLogin(l, "admin", "admin ") || CheckLogin(l, " admin", "admin") {
		t.Fatal("user/pass salah diterima")
	}
	// File harus ada dan berisi JSON yang sama.
	on, err := LoadLogin(path)
	if err != nil {
		t.Fatal(err)
	}
	if on != l {
		t.Fatalf("file != nilai awal: %+v vs %+v", on, l)
	}
	text := readFile(t, path)
	if !strings.Contains(text, "\n  \"user\": \"admin\",\n  \"salt\": ") || strings.HasSuffix(text, "\n") {
		t.Fatalf("format file login salah: %q", text)
	}
}

func TestLoadLoginError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.login")
	writeFile(t, path, "{rusak")
	if _, err := LoadLogin(path); err == nil {
		t.Fatal("JSON rusak harus mengembalikan error (Python json.load juga raise)")
	}
	dir := t.TempDir()
	if _, err := LoadLogin(dir); err == nil {
		t.Fatal("path direktori harus error")
	}
}

func TestSaveLogin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.login")
	old, err := LoadLogin(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveLogin(path, "budi", "rahasia"); err != nil {
		t.Fatal(err)
	}
	l, err := LoadLogin(path)
	if err != nil {
		t.Fatal(err)
	}
	if l.User != "budi" || l.Salt == old.Salt {
		t.Fatalf("save_login harus pakai salt baru: %+v (lama %q)", l, old.Salt)
	}
	if len(l.Salt) != 16 {
		t.Fatalf("salt = %q", l.Salt)
	}
	if !CheckLogin(l, "budi", "rahasia") || CheckLogin(l, "budi", "admin") || CheckLogin(l, "admin", "rahasia") {
		t.Fatal("kredensial hasil SaveLogin salah")
	}
	text := readFile(t, path)
	iUser, iSalt, iHash := strings.Index(text, "\"user\""), strings.Index(text, "\"salt\""), strings.Index(text, "\"hash\"")
	if !(iUser >= 0 && iUser < iSalt && iSalt < iHash) {
		t.Fatalf("urutan key file login salah: %q", text)
	}
}

func TestSessionsNewCheckDrop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.sessions")
	s := NewSessions(path)
	before := time.Now().Unix()
	tok := s.New("admin")
	if len(tok) != 43 { // secrets.token_urlsafe(32) -> 43 karakter
		t.Fatalf("token = %q (%d karakter), want 43", tok, len(tok))
	}
	if strings.ContainsAny(tok, "+/=") {
		t.Fatalf("token bukan base64 raw url: %q", tok)
	}
	if !s.Check(tok) {
		t.Fatal("token baru tidak valid")
	}
	if s.Check("") || s.Check("token-palsu") {
		t.Fatal("token kosong/palsu diterima")
	}

	var raw map[string]struct {
		User    string `json:"user"`
		Exp     int64  `json:"exp"`
		Created int64  `json:"created"`
	}
	if err := json.Unmarshal([]byte(readFile(t, path)), &raw); err != nil {
		t.Fatal(err)
	}
	e, ok := raw[tok]
	if !ok || e.User != "admin" {
		t.Fatalf("isi file = %+v", raw)
	}
	if e.Exp-e.Created != 30*24*3600 {
		t.Fatalf("TTL = %d detik, want 2592000", e.Exp-e.Created)
	}
	if e.Created < before || e.Created > time.Now().Unix() {
		t.Fatalf("created = %d tidak masuk akal", e.Created)
	}
	if s2 := NewSessions(path); !s2.Check(tok) {
		t.Fatal("sesi tidak terbaca oleh instance lain (file tidak dipakai?)")
	}

	s.Drop(tok)
	if s.Check(tok) {
		t.Fatal("token masih valid setelah Drop")
	}
	// Drop token yang tidak ada / kosong tidak boleh mengubah apa-apa.
	s.Drop(tok)
	s.Drop("")
}

// Format file harus sama dengan json.dump(indent=2) Python.
func TestSessionsFileFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.sessions")
	s := NewSessions(path)
	s.save(map[string]sessionEntry{"tok": {User: "admin", Exp: 2000000000, Created: 1999999999}})
	want := "{\n  \"tok\": {\n    \"user\": \"admin\",\n    \"exp\": 2000000000,\n    \"created\": 1999999999\n  }\n}"
	if got := readFile(t, path); got != want {
		t.Fatalf("format sesi\n got: %q\nwant: %q", got, want)
	}
	// {}/indent=2 di Python -> "{}".
	writeFile(t, path, want)
	s.Drop("tok")
	if got := readFile(t, path); got != "{}" {
		t.Fatalf("file sesi kosong = %q, want {}", got)
	}
}

func TestSessionsPruneExpired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.sessions")
	now := time.Now().Unix()
	expired, live := "tok-kadaluarsa", "tok-hidup"
	body, err := json.MarshalIndent(map[string]sessionEntry{
		expired: {User: "admin", Exp: now - 100, Created: now - 100 - 30*24*3600},
		live:    {User: "admin", Exp: now + 3600, Created: now},
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(body))

	s := NewSessions(path)
	if s.Check(expired) {
		t.Fatal("entri kadaluarsa masih dianggap valid")
	}
	if !s.Check(live) {
		t.Fatal("entri yang masih hidup dianggap kadaluarsa")
	}
	if strings.Contains(readFile(t, path), expired) == false {
		t.Fatal("Check() tidak boleh menulis file (Python _sess_load juga tidak)")
	}
	// Menulis (New) memangkas entri kadaluarsa.
	tok := s.New("admin")
	text := readFile(t, path)
	if strings.Contains(text, expired) {
		t.Fatalf("entri kadaluarsa masih ada di file: %s", text)
	}
	if !strings.Contains(text, live) || !strings.Contains(text, tok) {
		t.Fatalf("entri valid hilang: %s", text)
	}
}

func TestSessionsEdgeCases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.sessions")
	s := NewSessions(path)
	if s.Check("apa-saja") {
		t.Fatal("file belum ada -> tidak ada sesi valid")
	}
	tok := s.New("admin")
	if tok == "" || !s.Check(tok) {
		t.Fatal("New gagal dengan file sesi baru")
	}

	// File rusak dianggap kosong (Python juga begitu).
	writeFile(t, path, "{rusak")
	if s.Check(tok) {
		t.Fatal("file rusak harus dianggap kosong")
	}
	if got := s.New("admin"); got == "" || !s.Check(got) {
		t.Fatal("New setelah file rusak gagal")
	}

	// Entri bukan object dilewati, bukan bikin seluruh file dibuang.
	writeFile(t, path, `{"angka": 5, "string": "x", "kosong": null, "tanpa-exp": {"user": "admin"}}`)
	if s.Check("angka") || s.Check("string") || s.Check("tanpa-exp") {
		t.Fatal("entri non-object / tanpa exp dianggap valid")
	}
	if got := s.New("admin"); got == "" || !s.Check(got) {
		t.Fatal("New setelah entri aneh gagal")
	}
}
