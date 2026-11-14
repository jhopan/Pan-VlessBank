package settings

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClientTokenAuthorizesAndRevokes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.client-tokens.json")
	store := NewClientTokens(path)
	raw, record, err := store.Create("camoufox", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || record.ID == "" || record.Hash != "" {
		t.Fatalf("unsafe token response: %#v", record)
	}
	if !store.Check(raw) {
		t.Fatal("created token rejected")
	}
	if err := store.Revoke(record.ID); err != nil {
		t.Fatal(err)
	}
	if store.Check(raw) {
		t.Fatal("revoked token accepted")
	}
}

func TestClientTokenExpiresAndFileDoesNotContainRawToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webui.client-tokens.json")
	store := NewClientTokens(path)
	raw, _, err := store.Create("short", -time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if store.Check(raw) {
		t.Fatal("expired token accepted")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) == "" || contains(string(b), raw) {
		t.Fatal("raw token persisted")
	}
}

func contains(s, part string) bool {
	return len(part) > 0 && len(s) >= len(part) && index(s, part) >= 0
}
func index(s, part string) int {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}
