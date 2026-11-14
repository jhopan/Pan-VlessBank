package settings

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestClientTokenScopePersists(t *testing.T) {
	s := NewClientTokens(filepath.Join(t.TempDir(), "tokens.json"))
	raw, r, e := s.CreateScoped("one", time.Hour, []int{2})
	if e != nil {
		t.Fatal(e)
	}
	scope, ok := s.Scope(raw)
	if !ok || !reflect.DeepEqual(scope, []int{2}) {
		t.Fatalf("scope=%v ok=%v record=%#v", scope, ok, r)
	}
}
