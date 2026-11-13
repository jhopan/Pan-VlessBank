package client

import "testing"

func TestJobsBuildsStableLocalPorts(t *testing.T) {
	got := Jobs("proxy.example.com", 2, 7001, 8001)
	if len(got) != 4 {
		t.Fatalf("jobs=%d", len(got))
	}
	want := []Job{{1, "socks", "socks1.proxy.example.com", 7001}, {1, "http", "http1.proxy.example.com", 8001}, {2, "socks", "socks2.proxy.example.com", 7002}, {2, "http", "http2.proxy.example.com", 8002}}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("job %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestConfigValidateRejectsPublicPanel(t *testing.T) {
	if err := (Config{Listen: "0.0.0.0", Domain: "proxy.example.com", Accounts: 1}).Validate(); err == nil {
		t.Fatal("public panel accepted")
	}
}
