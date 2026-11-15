package web

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"vlessbank/internal/settings"
)

func TestClientTokenCanReadTunnelDiagnostics(t *testing.T) {
	e := newEnv(t)
	store := settings.NewClientTokens(filepath.Join(t.TempDir(), "webui.client-tokens.json"))
	raw, _, err := store.CreateScoped("client", time.Hour, []int{2})
	if err != nil {
		t.Fatal(err)
	}
	e.srv.d.ClientTokens = store
	w := req(t, e.srv, http.MethodGet, "/api/cf/tunnel-diagnostics", "", map[string]string{"Authorization": "Bearer " + raw})
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if w = req(t, e.srv, http.MethodGet, "/api/cf/tunnel-diagnostics", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized code=%d", w.Code)
	}
}
