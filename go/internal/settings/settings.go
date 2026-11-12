// Package settings menyimpan state panel yang di versi Python ditangani webui.py:
// webui.settings.json (konfigurasi), webui.login (kredensial), dan webui.sessions
// (token cookie). Bentuk file/JSON dan peraturannya harus tetap kompatibel dengan
// versi Python: hash = sha256(pw + salt), salt 16 hex, sesi 30 hari.
package settings

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"
)

// Settings = dict DEFAULTS/SET di webui.py. Urutan field sengaja sama dengan urutan
// key Python supaya isi webui.settings.json identik byte-per-byte.
type Settings struct {
	Xray     string `json:"xray"`
	Base     int    `json:"base"`
	BaseHTTP int    `json:"base_http"`
	Listen   string `json:"listen"`
	Auth     string `json:"auth"`
	Tunnel   string `json:"tunnel"`
	TunnelID string `json:"tunnel_id"`
	XrayPath string `json:"xray_path"`
	Insecure bool   `json:"insecure"`
}

// Defaults = webui.py DEFAULTS. defaultXray = nilai --xray.
func Defaults(defaultXray string) Settings {
	return Settings{
		Xray:     "",
		Base:     1081,
		BaseHTTP: 0,
		Listen:   "127.0.0.1",
		Auth:     "",
		Tunnel:   "",
		TunnelID: "",
		XrayPath: defaultXray,
		Insecure: false,
	}
}

// LoadSettings = load_set(): mulai dari DEFAULTS lalu ditimpa key yang ada di file;
// key yang tidak ada tetap default. File tidak ada atau JSON rusak -> default
// (Python menelan exception-nya).
func LoadSettings(path, defaultXray string) Settings {
	b, err := os.ReadFile(path)
	if err != nil {
		return Defaults(defaultXray)
	}
	s := Defaults(defaultXray)
	if err := json.Unmarshal(b, &s); err != nil {
		return Defaults(defaultXray)
	}
	return s
}

// SaveSettings = save_set(): indent 2, key lengkap, tanpa newline penutup.
func SaveSettings(path string, s Settings) error {
	return writeJSON(path, s, 0o644)
}

// Login = isi webui.login.
type Login struct {
	User string `json:"user"`
	Salt string `json:"salt"`
	Hash string `json:"hash"`
}

// hashPW = sha256(pw + salt) hex (webui.check_login / save_login).
func hashPW(pass, salt string) string {
	sum := sha256.Sum256([]byte(pass + salt))
	return hex.EncodeToString(sum[:])
}

// LoadLogin = load_login(): kalau webui.login belum ada, file dibuat berisi
// admin/admin (salt 16 hex acak) lalu dikembalikan.
func LoadLogin(path string) (Login, error) {
	b, err := os.ReadFile(path)
	switch {
	case err == nil:
		var l Login
		if err := json.Unmarshal(b, &l); err != nil {
			return Login{}, err
		}
		return l, nil
	case errors.Is(err, os.ErrNotExist):
		salt, err := tokenHex(8)
		if err != nil {
			return Login{}, err
		}
		l := Login{User: "admin", Salt: salt, Hash: hashPW("admin", salt)}
		if err := writeLogin(path, l); err != nil {
			return Login{}, err
		}
		return l, nil
	default:
		return Login{}, err
	}
}

// CheckLogin = check_login(): user dan hash dibandingkan constant-time,
// hash dihitung dari sha256(pass + salt).
func CheckLogin(l Login, user, pass string) bool {
	okUser := subtle.ConstantTimeCompare([]byte(user), []byte(l.User)) == 1
	okHash := subtle.ConstantTimeCompare([]byte(hashPW(pass, l.Salt)), []byte(l.Hash)) == 1
	return okUser && okHash
}

// SaveLogin = save_login(): salt baru 16 hex, hash dari password baru.
func SaveLogin(path, user, pass string) error {
	salt, err := tokenHex(8)
	if err != nil {
		return err
	}
	return writeLogin(path, Login{User: user, Salt: salt, Hash: hashPW(pass, salt)})
}

func writeLogin(path string, l Login) error {
	return writeJSON(path, l, 0o600)
}

// sessTTL = SESS_TTL di webui.py: 30 hari (detik).
const sessTTL = 30 * 24 * 3600

// sessionEntry = satu nilai di webui.sessions. Urutan field = urutan key Python.
type sessionEntry struct {
	User    string `json:"user"`
	Exp     int64  `json:"exp"`
	Created int64  `json:"created"`
}

// Sessions menyimpan token cookie vb_session di webui.sessions,
// formatnya {token: {"user","exp","created"}}. Aman dipakai banyak goroutine.
type Sessions struct {
	path string
	mu   sync.Mutex
}

// NewSessions membuat handle untuk file webui.sessions (file belum tentu ada).
func NewSessions(path string) *Sessions { return &Sessions{path: path} }

// load = _sess_load(): entri kadaluarsa dibuang, file hilang/JSON rusak -> kosong.
// Entri yang bukan object dilewati (seperti Python yang mengecek isinstance dict).
func (s *Sessions) load() map[string]sessionEntry {
	d := make(map[string]sessionEntry)
	b, err := os.ReadFile(s.path)
	if err != nil {
		return d
	}
	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(b, &raw); err != nil {
		return d
	}
	now := time.Now().Unix()
	for tok, v := range raw {
		var e sessionEntry
		if err := json.Unmarshal(v, &e); err != nil {
			continue
		}
		if e.Exp > now {
			d[tok] = e
		}
	}
	return d
}

// save = _sess_save(): error ditelan (Python juga begitu).
func (s *Sessions) save(d map[string]sessionEntry) {
	_ = writeJSON(s.path, d, 0o600)
}

// New = session_new(): token 32 byte urlsafe (base64 raw url, 43 karakter),
// masa berlaku 30 hari. Entri kadaluarsa sekalian dipangkas saat menulis.
func (s *Sessions) New(user string) string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	tok := base64.RawURLEncoding.EncodeToString(buf)
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.load()
	now := time.Now().Unix()
	d[tok] = sessionEntry{User: user, Exp: now + sessTTL, Created: now}
	s.save(d)
	return tok
}

// Check = session_check(): hanya membaca (tidak menulis file) dan menganggap
// entri kadaluarsa sebagai tidak ada.
func (s *Sessions) Check(tok string) bool {
	if tok == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.load()[tok]
	return ok
}

// Drop = session_drop(): hapus token; token kosong atau tidak ada -> tidak menulis.
func (s *Sessions) Drop(tok string) {
	if tok == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.load()
	if _, ok := d[tok]; !ok {
		return
	}
	delete(d, tok)
	s.save(d)
}

// writeJSON menulis JSON indent 2 persis seperti json.dump(..., indent=2) Python:
// tanpa newline penutup.
func writeJSON(path string, v any, perm os.FileMode) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, perm)
}

// tokenHex = secrets.token_hex(n): n byte acak -> 2n karakter hex.
func tokenHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
