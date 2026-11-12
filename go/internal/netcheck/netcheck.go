// Package netcheck mengecek IP keluar tiap akun lewat SOCKS5 lokal, pengganti
// ip_test()/ip_all() di webui.py (ThreadPoolExecutor(8)) — tapi tanpa curl,
// karena seluruh stack proxy dikerjakan package ini.
package netcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// IPAPIURL = perintah curl di webui.py ip_test().
const IPAPIURL = "http://ip-api.com/json/?fields=query,countryCode,country"

// DefaultTimeout = timeout subprocess.run() di webui.py ip_test(): anggaran
// total untuk satu pengecekan IP.
const DefaultTimeout = 20 * time.Second

// curlTimeout = "--max-time 12" pada curl webui.py: batas satu request.
const curlTimeout = 12 * time.Second

// maxWorkers = ThreadPoolExecutor(max_workers=8) di webui.py ip_all().
const maxWorkers = 8

// Pesan-pesan hasil sama persis dengan webui.py.
const (
	MsgXrayDead = "xray mati"
	MsgUnknown  = "?"
	MsgTimeout  = "timeout"
	MsgDialFail = "gagal (dial upstream gagal)"
)

// IPTest = ip_test(): tanya ip-api.com lewat SOCKS5 127.0.0.1:port, hasilnya
// "1.2.3.4 · ID (Indonesia)". running=false (xray mati) selalu dicek lebih dulu.
// timeout = anggaran total (webui.py: subprocess timeout 20s); di dalamnya tetap
// berlaku batas curlTimeout ala curl --max-time 12. timeout <= 0 -> DefaultTimeout.
func IPTest(port int, running bool, timeout time.Duration) string {
	return IPTestAuth(port, running, "", "", timeout)
}

// IPTestAuth = IPTest dengan username/password SOCKS5 (xray inbound yang
// memakai `auth: password`, mis. listen 0.0.0.0). user kosong = tanpa auth.
func IPTestAuth(port int, running bool, user, pass string, timeout time.Duration) string {
	if !running {
		return MsgXrayDead
	}
	if port < 0 { // Python: not str(port).isdigit()
		return MsgUnknown
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	// curl berhenti di --max-time 12 walau anggaran subprocess masih longgar;
	// sisa stdout yang kosong itu yang di Python jadi "gagal (dial upstream gagal)".
	reqTimeout := min(curlTimeout, timeout)
	proxy := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	ctx, cancel := context.WithTimeout(context.Background(), reqTimeout)
	defer cancel()

	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			t := reqTimeout
			if dl, ok := ctx.Deadline(); ok {
				if d := time.Until(dl); d < t {
					t = d
				}
			}
			return DialSOCKS5Auth(proxy, addr, user, pass, t)
		},
		DisableKeepAlives: true,
	}
	defer tr.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, IPAPIURL, nil)
	if err != nil {
		return MsgDialFail
	}
	resp, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		return msgOnError(err, timeout)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return msgOnError(err, timeout)
	}
	var j struct {
		Query       *string `json:"query"`
		CountryCode *string `json:"countryCode"`
		Country     *string `json:"country"`
	}
	if err := json.Unmarshal(body, &j); err != nil {
		return MsgDialFail
	}
	q, cc, country := MsgUnknown, "??", ""
	if j.Query != nil {
		q = *j.Query
	}
	if j.CountryCode != nil {
		cc = *j.CountryCode
	}
	if j.Country != nil {
		country = *j.Country
	}
	return fmt.Sprintf("%s · %s (%s)", q, cc, country)
}

// IPAll = ip_all(): tes semua port SOCKS dengan pool 8 worker, hasil
// {port: pesan}. xray mati -> map kosong.
func IPAll(ports []int, running bool) map[string]string {
	return IPAllAuth(ports, running, "", "")
}

// IPAllAuth = IPAll dengan kredensial SOCKS5.
func IPAllAuth(ports []int, running bool, user, pass string) map[string]string {
	res := make(map[string]string)
	if !running {
		return res
	}
	jobs := make(chan int)
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)
	workers := maxWorkers
	if len(ports) < workers {
		workers = len(ports)
	}
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				r := IPTestAuth(p, true, user, pass, DefaultTimeout)
				mu.Lock()
				res[strconv.Itoa(p)] = r
				mu.Unlock()
			}
		}()
	}
	for _, p := range ports {
		jobs <- p
	}
	close(jobs)
	wg.Wait()
	return res
}

// msgOnError menerjemahkan error jadi pesan webui.py: deadline/timeout terpisah
// dari kegagalan dial (di Python: subprocess.TimeoutExpired vs json.loads gagal).
func msgOnError(err error, budget time.Duration) string {
	if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
		return msgOnTimeout(budget)
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return msgOnTimeout(budget)
	}
	return MsgDialFail
}

// msgOnTimeout: anggaran habis. Kalau yang habis adalah batas ala curl
// --max-time 12 sementara anggaran pemanggil lebih longgar, Python berakhir di
// json.loads(stdout kosong) = "gagal (dial upstream gagal)", bukan "timeout".
func msgOnTimeout(budget time.Duration) string {
	if budget > curlTimeout {
		return MsgDialFail
	}
	return MsgTimeout
}
