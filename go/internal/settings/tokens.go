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
	"sort"
	"sync"
	"time"
)

type ClientToken struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Hash     string `json:"hash,omitempty"`
	Accounts []int  `json:"accounts,omitempty"`
	Created  int64  `json:"created"`
	Exp      int64  `json:"exp"`
	Revoked  bool   `json:"revoked"`
}
type ClientTokens struct {
	path string
	mu   sync.Mutex
}

func NewClientTokens(path string) *ClientTokens { return &ClientTokens{path: path} }
func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
func (t *ClientTokens) load() []ClientToken {
	b, e := os.ReadFile(t.path)
	if e != nil {
		return []ClientToken{}
	}
	var v []ClientToken
	if json.Unmarshal(b, &v) != nil {
		return []ClientToken{}
	}
	return v
}
func (t *ClientTokens) save(v []ClientToken) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(t.path, b, 0600)
}
func (t *ClientTokens) Create(label string, ttl time.Duration) (string, ClientToken, error) {
	return t.CreateScoped(label, ttl, nil)
}
func (t *ClientTokens) CreateScoped(label string, ttl time.Duration, accounts []int) (string, ClientToken, error) {
	if label == "" {
		return "", ClientToken{}, errors.New("label token kosong")
	}
	rawb := make([]byte, 32)
	if _, e := rand.Read(rawb); e != nil {
		return "", ClientToken{}, e
	}
	idb := make([]byte, 8)
	if _, e := rand.Read(idb); e != nil {
		return "", ClientToken{}, e
	}
	raw := base64.RawURLEncoding.EncodeToString(rawb)
	now := time.Now().Unix()
	r := ClientToken{ID: hex.EncodeToString(idb), Label: label, Hash: tokenHash(raw), Accounts: append([]int(nil), accounts...), Created: now, Exp: now + int64(ttl.Seconds())}
	t.mu.Lock()
	defer t.mu.Unlock()
	v := append(t.load(), r)
	if e := t.save(v); e != nil {
		return "", ClientToken{}, e
	}
	r.Hash = ""
	return raw, r, nil
}
func (t *ClientTokens) Check(raw string) bool {
	if raw == "" {
		return false
	}
	want := tokenHash(raw)
	now := time.Now().Unix()
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, r := range t.load() {
		if !r.Revoked && r.Exp > now && subtle.ConstantTimeCompare([]byte(r.Hash), []byte(want)) == 1 {
			return true
		}
	}
	return false
}

func (t *ClientTokens) Scope(raw string) ([]int, bool) {
	if raw == "" {
		return nil, false
	}
	want := tokenHash(raw)
	now := time.Now().Unix()
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, r := range t.load() {
		if !r.Revoked && r.Exp > now && subtle.ConstantTimeCompare([]byte(r.Hash), []byte(want)) == 1 {
			return append([]int(nil), r.Accounts...), true
		}
	}
	return nil, false
}
func (t *ClientTokens) List() []ClientToken {
	t.mu.Lock()
	defer t.mu.Unlock()
	v := t.load()
	out := make([]ClientToken, 0, len(v))
	for _, r := range v {
		r.Hash = ""
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created > out[j].Created })
	return out
}
func (t *ClientTokens) Revoke(id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	v := t.load()
	for i := range v {
		if v[i].ID == id {
			v[i].Revoked = true
			return t.save(v)
		}
	}
	return errors.New("token tidak ditemukan")
}
