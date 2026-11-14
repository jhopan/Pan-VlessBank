package ops

import "testing"

func TestTunnelIngressListsHostsAndPorts(t *testing.T) {
	in := TunnelIngress([]byte("ingress:\n  - hostname: socks1.example.com\n    service: tcp://localhost:1081\n  - hostname: http1.example.com\n    service: tcp://localhost:2081\n"))
	if len(in) != 2 || in[0].Host != "socks1.example.com" || in[0].Port != 1081 || in[1].Port != 2081 {
		t.Fatalf("ingress=%#v", in)
	}
}
