// Client SOCKS5 murni Go (RFC 1928, tanpa auth). Versi Python memakai
// `curl --socks5-hostname 127.0.0.1:PORT`; di sini handshake-nya dikerjakan
// sendiri supaya binary Go tidak butuh curl.
package netcheck

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	socksVersion = 0x05

	socksCmdConnect  = 0x01
	socksMethodNone  = 0x00
	socksMethodUser  = 0x02 // RFC 1929 username/password
	socksMethodNoAcc = 0xFF

	atypIPv4   = 0x01
	atypDomain = 0x03
	atypIPv6   = 0x04

	userPassVersion = 0x01
)

// socksReplyNames = nama pesan balasan SOCKS5 (field REP, RFC 1928 §6).
var socksReplyNames = map[byte]string{
	0x01: "general SOCKS server failure",
	0x02: "connection not allowed by ruleset",
	0x03: "network unreachable",
	0x04: "host unreachable",
	0x05: "connection refused",
	0x06: "TTL expired",
	0x07: "command not supported",
	0x08: "address type not supported",
}

// DialSOCKS5 melakukan handshake SOCKS5 CONNECT tanpa auth lewat proxyAddr
// (mis. "127.0.0.1:1081") dan mengembalikan koneksi siap pakai ke target
// ("host:port"). Nama host dikirim apa adanya (ATYP 0x03, resolusi di sisi
// proxy) sama seperti curl --socks5-hostname; IP literal pakai ATYP 0x01/0x04.
// Deadline hanya berlaku selama handshake, setelah itu koneksi bersih.
func DialSOCKS5(proxyAddr, target string, timeout time.Duration) (net.Conn, error) {
	return DialSOCKS5Auth(proxyAddr, target, "", "", timeout)
}

// DialSOCKS5Auth sama seperti DialSOCKS5 tetapi memakai username/password
// (RFC 1929) kalau user tidak kosong — perlu saat xray inbound memakai
// `auth: password` (panel dengan listen 0.0.0.0 wajib auth).
func DialSOCKS5Auth(proxyAddr, target, user, pass string, timeout time.Duration) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return nil, fmt.Errorf("socks5: target tidak valid (%q): %w", target, err)
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil {
		return nil, fmt.Errorf("socks5: port target tidak valid (%q): %w", portStr, err)
	}
	if host == "" {
		return nil, fmt.Errorf("socks5: target tidak valid (%q): host kosong", target)
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	conn, err := net.DialTimeout("tcp", proxyAddr, timeout)
	if err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		conn.Close()
		return nil, err
	}
	if err := socks5Handshake(conn, host, port, user, pass); err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// socks5Handshake menegosiasikan metode no-auth lalu mengirim CONNECT dan
// membaca balasan (termasuk membuang BND.ADDR/BND.PORT).
func socks5Handshake(c net.Conn, host string, port int, user, pass string) error {
	// greeting: pilih metode no-auth atau username/password
	if user != "" {
		if _, err := c.Write([]byte{socksVersion, 0x01, socksMethodUser}); err != nil {
			return fmt.Errorf("socks5: kirim greeting: %w", err)
		}
	} else if _, err := c.Write([]byte{socksVersion, 0x01, socksMethodNone}); err != nil {
		return fmt.Errorf("socks5: kirim greeting: %w", err)
	}
	var greet [2]byte
	if _, err := io.ReadFull(c, greet[:]); err != nil {
		return fmt.Errorf("socks5: baca metode: %w", err)
	}
	if greet[0] != socksVersion {
		return fmt.Errorf("socks5: versi proxy tidak dikenal (0x%02x)", greet[0])
	}
	switch greet[1] {
	case socksMethodNone:
		// tanpa auth, lanjut
	case socksMethodUser:
		if user == "" {
			return fmt.Errorf("socks5: proxy minta username/password, kredensial tidak diisi")
		}
		if len(user) > 255 || len(pass) > 255 {
			return fmt.Errorf("socks5: username/password terlalu panjang")
		}
		msg := make([]byte, 0, 3+len(user)+len(pass))
		msg = append(msg, userPassVersion, byte(len(user)))
		msg = append(msg, user...)
		msg = append(msg, byte(len(pass)))
		msg = append(msg, pass...)
		if _, err := c.Write(msg); err != nil {
			return fmt.Errorf("socks5: kirim auth: %w", err)
		}
		var st [2]byte
		if _, err := io.ReadFull(c, st[:]); err != nil {
			return fmt.Errorf("socks5: baca auth: %w", err)
		}
		if st[1] != 0x00 {
			return fmt.Errorf("socks5: auth ditolak (status 0x%02x)", st[1])
		}
	default:
		return fmt.Errorf("socks5: proxy minta metode auth yang tidak didukung (0x%02x)", greet[1])
	}

	req := make([]byte, 0, 4+len(host)+2)
	req = append(req, socksVersion, socksCmdConnect, 0x00)
	if ip := net.ParseIP(host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			req = append(req, atypIPv4)
			req = append(req, v4...)
		} else {
			req = append(req, atypIPv6)
			req = append(req, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return fmt.Errorf("socks5: nama host terlalu panjang (%d)", len(host))
		}
		req = append(req, atypDomain, byte(len(host)))
		req = append(req, host...)
	}
	var p [2]byte
	binary.BigEndian.PutUint16(p[:], uint16(port))
	req = append(req, p[:]...)
	if _, err := c.Write(req); err != nil {
		return fmt.Errorf("socks5: kirim CONNECT: %w", err)
	}

	var head [4]byte
	if _, err := io.ReadFull(c, head[:]); err != nil {
		return fmt.Errorf("socks5: baca balasan: %w", err)
	}
	if head[0] != socksVersion {
		return fmt.Errorf("socks5: versi balasan tidak dikenal (0x%02x)", head[0])
	}
	if head[1] != 0x00 {
		name, ok := socksReplyNames[head[1]]
		if !ok {
			name = "kode tidak dikenal"
		}
		return fmt.Errorf("socks5: CONNECT gagal: %s (0x%02x)", name, head[1])
	}
	var err error
	switch head[3] {
	case atypIPv4:
		_, err = io.CopyN(io.Discard, c, 4+2)
	case atypIPv6:
		_, err = io.CopyN(io.Discard, c, 16+2)
	case atypDomain:
		var n [1]byte
		if _, err = io.ReadFull(c, n[:]); err == nil {
			_, err = io.CopyN(io.Discard, c, int64(n[0])+2)
		}
	default:
		return fmt.Errorf("socks5: ATYP balasan tidak dikenal (0x%02x)", head[3])
	}
	if err != nil {
		return fmt.Errorf("socks5: baca bind address: %w", err)
	}
	return nil
}
