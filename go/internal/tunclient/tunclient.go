// Package tunclient: `vlessbank tunnel-client` — jalankan cloudflared access tcp
// untuk semua akun sekaligus (1 hostname = 1 port lokal). Pengganti Go untuk
// tunnel-client.py, supaya client publik tidak butuh Python.
package tunclient

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type Job struct {
	Host string // hostname tunnel, mis. socks1.example.com
	Port int    // port lokal yang dibuka
	Kind string // socks | http
}

// Build menyusun daftar listener. sub kosong = hostname 1 level (wajib untuk
// cert Universal SSL free plan).
func Build(domain, sub string, accounts, socksBase, httpBase int, noHTTP bool) []Job {
	prefix := ""
	if sub != "" {
		prefix = sub + "."
	}
	jobs := make([]Job, 0, accounts*2)
	for i := range accounts {
		n := i + 1
		jobs = append(jobs, Job{Host: "socks" + strconv.Itoa(n) + "." + prefix + domain,
			Port: socksBase + i, Kind: "socks"})
		if !noHTTP {
			jobs = append(jobs, Job{Host: "http" + strconv.Itoa(n) + "." + prefix + domain,
				Port: httpBase + i, Kind: "http"})
		}
	}
	return jobs
}

func uri(scheme string, port int, auth string) string {
	cred := ""
	if auth != "" {
		cred = auth + "@"
	}
	return scheme + "://" + cred + "127.0.0.1:" + strconv.Itoa(port)
}

// Print menulis tabel listener + URI client (dipakai juga oleh --dry).
func Print(jobs []Job, auth string, out io.Writer) {
	total := 0
	for _, j := range jobs {
		if j.Kind == "socks" {
			total++
		}
	}
	fmt.Fprintf(out, "%d akun -> %d listener lokal\n", total, len(jobs))
	for _, j := range jobs {
		fmt.Fprintf(out, "  %-5s 127.0.0.1:%-5d -> %s\n", j.Kind, j.Port, j.Host)
	}
	fmt.Fprint(out, "\nURI client:\n")
	for _, j := range jobs {
		scheme := "socks5"
		if j.Kind == "http" {
			scheme = "http"
		}
		fmt.Fprintf(out, "  %-32s (%s)\n", uri(scheme, j.Port, auth), j.Host)
	}
}

// Run menjalankan satu `cloudflared access tcp` per job, lalu menunggu sampai
// Ctrl+C atau ada listener yang mati. Semua anak proses dibersihkan saat keluar.
func Run(cfBin string, jobs []Job, out io.Writer) error {
	procs := make([]*exec.Cmd, 0, len(jobs))
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	defer func() {
		for _, c := range procs {
			killTree(c)
		}
		if len(procs) > 0 {
			fmt.Fprintln(out, "stop semua listener")
		}
	}()
	for _, j := range jobs {
		cmd := exec.Command(cfBin, "access", "tcp", "--hostname", j.Host,
			"--url", "127.0.0.1:"+strconv.Itoa(j.Port))
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("gagal start cloudflared: %w", err)
		}
		procs = append(procs, cmd)
		time.Sleep(350 * time.Millisecond) // beri waktu bind port
	}
	fmt.Fprintf(out, "\n%d listener jalan. Ctrl+C untuk stop.\n", len(procs))

	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	var mu sync.Mutex
	for {
		select {
		case <-stop:
			return nil
		case <-tick.C:
			mu.Lock()
			for i, c := range procs {
				if c.ProcessState != nil && c.ProcessState.Exited() {
					fmt.Fprintf(out, "WARN listener mati: 127.0.0.1:%d (%s)\n", jobs[i].Port, jobs[i].Host)
					mu.Unlock()
					return nil
				}
			}
			mu.Unlock()
		}
	}
}

func killTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
		return
	}
	_ = cmd.Process.Kill()
}
