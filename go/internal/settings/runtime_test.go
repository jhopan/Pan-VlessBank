package settings

import (
	"path/filepath"
	"testing"
)

func TestRuntimeDefaultsXrayDisabled(t *testing.T) {
	s := NewRuntime(filepath.Join(t.TempDir(), "runtime.json"))
	if s.Load().XrayEnabled {
		t.Fatal("missing runtime enabled xray")
	}
}
func TestRuntimePersistsXrayIntent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "runtime.json")
	s := NewRuntime(p)
	if err := s.SetXrayEnabled(true); err != nil {
		t.Fatal(err)
	}
	if !NewRuntime(p).Load().XrayEnabled {
		t.Fatal("enabled xray intent lost")
	}
}
