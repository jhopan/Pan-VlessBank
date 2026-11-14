package settings

import (
	"encoding/json"
	"os"
)

type Runtime struct {
	XrayEnabled bool `json:"xray_enabled"`
}
type RuntimeStore struct{ path string }

func NewRuntime(path string) *RuntimeStore { return &RuntimeStore{path: path} }
func (s *RuntimeStore) Load() Runtime {
	b, e := os.ReadFile(s.path)
	if e != nil {
		return Runtime{}
	}
	var r Runtime
	if json.Unmarshal(b, &r) != nil {
		return Runtime{}
	}
	return r
}
func (s *RuntimeStore) SetXrayEnabled(enabled bool) error {
	b, e := json.MarshalIndent(Runtime{XrayEnabled: enabled}, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(s.path, b, 0600)
}
