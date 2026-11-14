package service

import "testing"

func TestManagerUsesSystemctlServiceName(t *testing.T) {
	var got []string
	m := New(func(args ...string) (string, error) { got = append([]string{}, args...); return "active\n", nil }, "pan-vlessbank-proxy-tunnel.service")
	if !m.Active() {
		t.Fatal("active service rejected")
	}
	if got[0] != "is-active" || got[1] != "pan-vlessbank-proxy-tunnel.service" {
		t.Fatalf("args=%q", got)
	}
}
