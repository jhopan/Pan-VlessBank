package service

import "strings"

type Runner func(args ...string) (string, error)
type Manager struct {
	run  Runner
	name string
}

func New(run Runner, name string) *Manager { return &Manager{run: run, name: name} }
func (m *Manager) Active() bool {
	out, e := m.run("is-active", m.name)
	return e == nil && strings.TrimSpace(out) == "active"
}
func (m *Manager) Action(action string) (string, error) { return m.run(action, m.name) }
func (m *Manager) Name() string                         { return m.name }
