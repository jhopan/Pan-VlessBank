package netcheck

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------- stub server

type socksTarget struct {
	atyp byte
	host string
	port int
}

// stubSOCKS5 = server SOCKS5 in-process: handshake no-auth sungguhan, lalu
// membalas HTTP mentah atau echo. Tidak butuh xray/curl/network.
type stubSOCKS5 struct {
	ln        net.Listener
	authReply byte             // metode balasan greeting (default 0x00)
	wantUser  string           // dipakai saat authReply == 0x02 (RFC 1929)
	wantPass  string
	repCode   byte             // REP saat CONNECT (default 0x00)
	hang      bool             // tidak pernah balas CONNECT (test timeout)
	after     func(c net.Conn) // dipanggil setelah CONNECT sukses; nil = echo
	mu        sync.Mutex
	seen      []socksTarget
}

func newStub(t *testing.T) *stubSOCKS5 {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &stubSOCKS5{ln: ln}
	go s.serve()
	t.Cleanup(func() { ln.Close() })
	return s
}

func (s *stubSOCKS5) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *stubSOCKS5) targets() []socksTarget {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]socksTarget(nil), s.seen...)
}

func (s *stubSOCKS5) serve() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(c)
	}
}

func (s *stubSOCKS5) handle(c net.Conn) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(30 * time.Second))

	var greet [2]byte
	if _, err := io.ReadFull(c, greet[:]); err != nil {
		return
	}
	methods := make([]byte, int(greet[1]))
	if _, err := io.ReadFull(c, methods); err != nil {
		return
	}
	if _, err := c.Write([]byte{5, s.authReply}); err != nil {
		return
	}
	if s.authReply == 0x02 {
		var ver, n [1]byte
		if _, err := io.ReadFull(c, ver[:]); err != nil {
			return
		}
		if _, err := io.ReadFull(c, n[:]); err != nil {
			return
		}
		u := make([]byte, int(n[0]))
		if _, err := io.ReadFull(c, u); err != nil {
			return
		}
		if _, err := io.ReadFull(c, n[:]); err != nil {
			return
		}
		pw := make([]byte, int(n[0]))
		if _, err := io.ReadFull(c, pw); err != nil {
			return
		}
		status := byte(0x00)
		if string(u) != s.wantUser || string(pw) != s.wantPass {
			status = 0x01
		}
		if _, err := c.Write([]byte{1, status}); err != nil {
			return
		}
		if status != 0x00 {
			return
		}
	} else if s.authReply != 0x00 {
		return
	}

	var req [4]byte
	if _, err := io.ReadFull(c, req[:]); err != nil {
		return
	}
	var host string
	switch req[3] {
	case atypIPv4:
		b := make([]byte, 4)
		if _, err := io.ReadFull(c, b); err != nil {
			return
		}
		host = net.IP(b).String()
	case atypIPv6:
		b := make([]byte, 16)
		if _, err := io.ReadFull(c, b); err != nil {
			return
		}
		host = net.IP(b).String()
	case atypDomain:
		var l [1]byte
		if _, err := io.ReadFull(c, l[:]); err != nil {
			return
		}
		b := make([]byte, int(l[0]))
		if _, err := io.ReadFull(c, b); err != nil {
			return
		}
		host = string(b)
	}
	var pb [2]byte
	if _, err := io.ReadFull(c, pb[:]); err != nil {
		return
	}
	s.mu.Lock()
	s.seen = append(s.seen, socksTarget{atyp: req[3], host: host, port: int(binary.BigEndian.Uint16(pb[:]))})
	s.mu.Unlock()

	if s.hang {
		io.Copy(io.Discard, c) // diam sampai client menyerah
		return
	}
	if s.repCode != 0 {
		c.Write([]byte{5, s.repCode, 0, atypIPv4, 0, 0, 0, 0, 0, 0})
		return
	}
	if _, err := c.Write([]byte{5, 0, 0, atypIPv4, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	if s.after != nil {
		s.after(c)
		return
	}
	io.Copy(c, c) // echo
}

// httpJSON = balas HTTP/1.1 200 mentah. Request dibaca dulu dan koneksi
// ditutup dengan CloseWrite supaya respons tidak hilang karena RST.
func httpJSON(body string) func(net.Conn) {
	return func(c net.Conn) {
		readHTTPHeader(c)
		resp := "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: " +
			strconv.Itoa(len(body)) + "\r\nConnection: close\r\n\r\n" + body
		io.WriteString(c, resp)
		if tc, ok := c.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
		io.Copy(io.Discard, c)
	}
}

// readHTTPHeader membaca request sampai baris kosong penutup header.
func readHTTPHeader(c net.Conn) {
	br := bufio.NewReader(c)
	for {
		line, err := br.ReadString('\n')
		if err != nil || line == "\r\n" || line == "\n" {
			return
		}
	}
}

// ---------------------------------------------------------------- DialSOCKS5

func TestDialSOCKS5Hostname(t *testing.T) {
	stub := newStub(t)
	conn, err := DialSOCKS5(net.JoinHostPort("127.0.0.1", strconv.Itoa(stub.port())), "example.com:80", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	got := stub.targets()
	if len(got) != 1 {
		t.Fatalf("target diterima proxy = %+v, want 1", got)
	}
	if got[0].atyp != atypDomain || got[0].host != "example.com" || got[0].port != 80 {
		t.Fatalf("target = %+v, want ATYP=domain example.com:80", got[0])
	}

	// Koneksi hasil handshake harus bisa dipakai (stub echo).
	if _, err := conn.Write([]byte("halo")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "halo" {
		t.Fatalf("echo = %q", buf)
	}
}

func TestDialSOCKS5IPTargets(t *testing.T) {
	cases := []struct {
		target string
		atyp   byte
		host   string
	}{
		{"1.2.3.4:443", atypIPv4, "1.2.3.4"},
		{"[2001:db8::1]:8080", atypIPv6, "2001:db8::1"},
	}
	for _, c := range cases {
		stub := newStub(t)
		conn, err := DialSOCKS5(net.JoinHostPort("127.0.0.1", strconv.Itoa(stub.port())), c.target, 5*time.Second)
		if err != nil {
			t.Fatalf("%s: %v", c.target, err)
		}
		conn.Close()
		got := stub.targets()
		if len(got) != 1 || got[0].atyp != c.atyp || got[0].host != c.host {
			t.Fatalf("%s -> %+v, want ATYP=0x%02x host %s", c.target, got, c.atyp, c.host)
		}
	}
}

func TestDialSOCKS5Refused(t *testing.T) {
	stub := newStub(t)
	stub.repCode = 0x05 // connection refused
	conn, err := DialSOCKS5(net.JoinHostPort("127.0.0.1", strconv.Itoa(stub.port())), "example.com:80", 5*time.Second)
	if err == nil {
		conn.Close()
		t.Fatal("REP=0x05 harus jadi error")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("error = %v, want menyebut connection refused", err)
	}
}

func TestDialSOCKS5AuthRequired(t *testing.T) {
	stub := newStub(t)
	stub.authReply = 0xFF // no acceptable methods
	conn, err := DialSOCKS5(net.JoinHostPort("127.0.0.1", strconv.Itoa(stub.port())), "example.com:80", 5*time.Second)
	if err == nil {
		conn.Close()
		t.Fatal("metode 0xFF harus jadi error")
	}
	if !strings.Contains(err.Error(), "auth") {
		t.Fatalf("error = %v", err)
	}
}

func TestDialSOCKS5ProxyDown(t *testing.T) {
	// Ambil port yang sudah pasti kosong: listen lalu tutup.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	conn, err := DialSOCKS5(addr, "example.com:80", 2*time.Second)
	if err == nil {
		conn.Close()
		t.Fatal("proxy mati harus jadi error")
	}
}

func TestDialSOCKS5BadTarget(t *testing.T) {
	stub := newStub(t)
	for _, target := range []string{"example.com", ":80", "example.com:abc"} {
		if conn, err := DialSOCKS5(net.JoinHostPort("127.0.0.1", strconv.Itoa(stub.port())), target, time.Second); err == nil {
			conn.Close()
			t.Fatalf("target %q harus ditolak", target)
		}
	}
}

func TestDialSOCKS5Timeout(t *testing.T) {
	stub := newStub(t)
	stub.hang = true
	start := time.Now()
	conn, err := DialSOCKS5(net.JoinHostPort("127.0.0.1", strconv.Itoa(stub.port())), "example.com:80", 400*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Fatal("handshake yang digantung harus timeout")
	}
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("error = %v (%T), want net.Error timeout", err, err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("timeout terlalu lama: %v", d)
	}
}

// -------------------------------------------------------------------- IPTest

func TestIPTestXrayDead(t *testing.T) {
	if got := IPTest(1081, false, time.Second); got != "xray mati" {
		t.Fatalf("IPTest running=false = %q", got)
	}
}

func TestIPTestBadPort(t *testing.T) {
	if got := IPTest(-1, true, time.Second); got != "?" {
		t.Fatalf("IPTest(port=-1) = %q, want \"?\"", got)
	}
}

func TestIPTestFormat(t *testing.T) {
	stub := newStub(t)
	stub.after = httpJSON(`{"query":"1.2.3.4","countryCode":"ID","country":"Indonesia"}`)
	got := IPTest(stub.port(), true, 5*time.Second)
	want := "1.2.3.4 · ID (Indonesia)"
	if got != want {
		t.Fatalf("IPTest = %q, want %q", got, want)
	}
	// ip-api dipanggil dengan hostname (bukan IP) lewat SOCKS5.
	got2 := stub.targets()
	if len(got2) != 1 || got2[0].atyp != atypDomain || got2[0].host != "ip-api.com" || got2[0].port != 80 {
		t.Fatalf("target = %+v", got2)
	}
}

func TestIPTestMissingFields(t *testing.T) {
	stub := newStub(t)
	stub.after = httpJSON(`{"status":"fail","message":"reserved range"}`)
	if got := IPTest(stub.port(), true, 5*time.Second); got != "? · ?? ()" {
		t.Fatalf("IPTest tanpa field = %q", got)
	}
}

func TestIPTestGagal(t *testing.T) {
	// Dial SOCKS gagal (port mati).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	if got := IPTest(dead, true, 2*time.Second); got != MsgDialFail {
		t.Fatalf("port mati = %q, want %q", got, MsgDialFail)
	}

	// Balasan bukan JSON.
	stub := newStub(t)
	stub.after = httpJSON("<html>oops</html>")
	if got := IPTest(stub.port(), true, 5*time.Second); got != MsgDialFail {
		t.Fatalf("balasan non-JSON = %q, want %q", got, MsgDialFail)
	}

	// Proxy menolak CONNECT.
	stub2 := newStub(t)
	stub2.repCode = 0x05
	if got := IPTest(stub2.port(), true, 5*time.Second); got != MsgDialFail {
		t.Fatalf("CONNECT ditolak = %q, want %q", got, MsgDialFail)
	}

	// Koneksi ditutup di tengah jalan.
	stub3 := newStub(t)
	stub3.after = func(c net.Conn) { c.Close() }
	if got := IPTest(stub3.port(), true, 5*time.Second); got != MsgDialFail {
		t.Fatalf("koneksi ditutup = %q, want %q", got, MsgDialFail)
	}
}

func TestIPTestTimeout(t *testing.T) {
	stub := newStub(t)
	stub.hang = true
	// Anggaran ketat (<= batas curl 12s): yang habis anggaran pemanggil -> "timeout".
	if got := IPTest(stub.port(), true, 300*time.Millisecond); got != "timeout" {
		t.Fatalf("IPTest hang = %q, want \"timeout\"", got)
	}
}

// Anggaran longgar (webui.py pakai 20s): yang habis adalah batas curl --max-time 12,
// dan di Python stdout kosong itu jadi "gagal (dial upstream gagal)".
func TestMsgOnTimeout(t *testing.T) {
	cases := []struct {
		budget time.Duration
		want   string
	}{
		{DefaultTimeout, MsgDialFail},
		{curlTimeout, MsgTimeout},
		{5 * time.Second, MsgTimeout},
	}
	for _, c := range cases {
		if got := msgOnTimeout(c.budget); got != c.want {
			t.Errorf("msgOnTimeout(%v) = %q, want %q", c.budget, got, c.want)
		}
	}
	// DefaultTimeout (anggaran webui.py) harus lebih longgar dari batas curl.
	if DefaultTimeout <= curlTimeout {
		t.Fatalf("DefaultTimeout=%v harus > curlTimeout=%v", DefaultTimeout, curlTimeout)
	}
}

// --------------------------------------------------------------------- IPAll

func TestIPAllXrayDead(t *testing.T) {
	if got := IPAll([]int{1081, 1082}, false); len(got) != 0 {
		t.Fatalf("IPAll running=false = %v, want kosong", got)
	}
	if got := IPAll(nil, true); len(got) != 0 {
		t.Fatalf("IPAll tanpa port = %v", got)
	}
}

func TestIPAll(t *testing.T) {
	// 3 port SOCKS palsu: dua menjawab, satu menolak -> pool 8 worker.
	ok1 := newStub(t)
	ok1.after = httpJSON(`{"query":"1.1.1.1","countryCode":"SG","country":"Singapore"}`)
	ok2 := newStub(t)
	ok2.after = httpJSON(`{"query":"2.2.2.2","countryCode":"ID","country":"Indonesia"}`)
	bad := newStub(t)
	bad.repCode = 0x05

	ports := []int{ok1.port(), ok2.port(), bad.port()}
	got := IPAll(ports, true)
	if len(got) != 3 {
		t.Fatalf("IPAll = %v, want 3 entri", got)
	}
	want := map[string]string{
		strconv.Itoa(ok1.port()): "1.1.1.1 · SG (Singapore)",
		strconv.Itoa(ok2.port()): "2.2.2.2 · ID (Indonesia)",
		strconv.Itoa(bad.port()): MsgDialFail,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("hasil[%s] = %q, want %q", k, got[k], v)
		}
	}
}

// TestDialSOCKS5Auth: inbound xray dengan auth: password (RFC 1929).
func TestDialSOCKS5Auth(t *testing.T) {
	st := newStub(t)
	st.authReply = 0x02
	st.wantUser, st.wantPass = "vb", "rahasia"

	conn, err := DialSOCKS5Auth(net.JoinHostPort("127.0.0.1", strconv.Itoa(st.port())),
		"example.com:80", "vb", "rahasia", 5*time.Second)
	if err != nil {
		t.Fatalf("auth benar harus lolos: %v", err)
	}
	conn.Close()

	if _, err := DialSOCKS5Auth(net.JoinHostPort("127.0.0.1", strconv.Itoa(st.port())),
		"example.com:80", "vb", "salah", 5*time.Second); err == nil {
		t.Fatal("password salah harus gagal")
	}
	if _, err := DialSOCKS5(net.JoinHostPort("127.0.0.1", strconv.Itoa(st.port())),
		"example.com:80", 5*time.Second); err == nil {
		t.Fatal("tanpa kredensial harus gagal saat proxy minta auth")
	}
}
