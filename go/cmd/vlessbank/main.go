// Command vlessbank — panel + generator, satu binary (stdlib only).
package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"vlessbank/internal/cf"
	"vlessbank/internal/config"
	"vlessbank/internal/diag"
	"vlessbank/internal/netcheck"
	"vlessbank/internal/settings"
	"vlessbank/internal/tunclient"
	"vlessbank/internal/web"
	"vlessbank/internal/xray"
)

type app struct {
	dir        string
	defaultBin string

	mu       sync.Mutex
	set      settings.Settings
	login    settings.Login
	sessions *settings.Sessions
	mgr      *xray.Manager
}

func (a *app) accountsFile() string { return filepath.Join(a.dir, "accounts.txt") }
func (a *app) configFile() string   { return filepath.Join(a.dir, "config.json") }
func (a *app) settingsFile() string { return filepath.Join(a.dir, "webui.settings.json") }
func (a *app) loginFile() string    { return filepath.Join(a.dir, "webui.login") }
func (a *app) xrayLog() string      { return filepath.Join(a.dir, "xray.log") }

func (a *app) findXray() string {
	a.mu.Lock()
	path := a.set.XrayPath
	a.mu.Unlock()
	cands := []string{}
	if path != "" {
		cands = append(cands, path)
	}
	if os.PathSeparator == '\\' {
		cands = append(cands, filepath.Join(a.dir, "xray.exe"))
	} else {
		cands = append(cands, filepath.Join(a.dir, "xray"))
	}
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	if path != "" {
		return path
	}
	return a.defaultBin
}

// regen mirrors gen.py main(): parse accounts, build config, write config.json (+ tunnel yml).
func (a *app) regen() string {
	a.mu.Lock()
	set := a.set
	a.mu.Unlock()

	accs, dup, err := config.ParseFile(a.accountsFile())
	if err != nil {
		return "accounts.txt tidak ada"
	}
	if len(accs) == 0 {
		return "ERROR: tidak ada akun valid"
	}
	major := config.XrayMajor(a.findXray())
	baseHTTP := set.BaseHTTP
	if baseHTTP == 0 {
		baseHTTP = set.Base + 1000
	}
	cfg, binds := config.BuildConfig(accs, set.Listen, set.Base, set.BaseHTTP, set.Auth, set.Insecure, major)
	if err := config.WriteJSON(a.configFile(), cfg); err != nil {
		return "gagal tulis config.json: " + err.Error()
	}
	// urutan pesan mengikuti gen.py: NOTE -> tunnel yml -> ringkasan -> WARN -> tabel akun
	var lines []string
	if set.Insecure && major >= 26 {
		lines = append(lines, fmt.Sprintf("NOTE: xray %d.x sudah hapus allowInsecure -> opsi dilewati", major))
	}
	if set.Tunnel != "" {
		yml := filepath.Join(a.dir, "config.tunnel.yml")
		cred, cerr := tunnelCreds(set.TunnelID, yml)
		if err := os.WriteFile(yml, []byte(config.TunnelYML(set.TunnelID, set.Tunnel, binds, set.Base, cred)), 0o644); err == nil {
			lines = append(lines, "tunnel yml -> "+yml)
		}
		if cred == "" {
			lines = append(lines, "WARN: kredensial tunnel tidak ketemu ("+fmt.Sprint(cerr)+") - pakai placeholder, jalankan 'Generate config.tunnel.yml' di menu Cloudflare")
		}
	}
	summary := fmt.Sprintf("%d akun -> %s (listen %s, SOCKS %d+, HTTP %d+)",
		len(binds), a.configFile(), set.Listen, set.Base, baseHTTP)
	if dup > 0 {
		summary += fmt.Sprintf(", %d duplikat dibuang", dup)
	}
	if set.Insecure {
		if major >= 26 {
			summary += fmt.Sprintf(", allowInsecure dilewati (xray %d.x)", major)
		} else {
			summary += ", allowInsecure ON"
		}
	}
	lines = append(lines, summary)
	if uniq := uniqEndpoints(binds); uniq < len(binds) {
		lines = append(lines, fmt.Sprintf(
			"WARN: %d akun tapi cuma %d endpoint upstream unik -> akun dengan endpoint sama akan keluar IP sama",
			len(binds), uniq))
	}
	for _, b := range binds {
		lines = append(lines, fmt.Sprintf("  SOCKS :%d | HTTP :%d  <-  %s", b.SocksPort, b.HTTPPort, b.Name))
	}
	return strings.Join(lines, "\n")
}

func uniqEndpoints(binds []config.Bind) int {
	seen := map[string]struct{}{}
	for _, b := range binds {
		seen[b.Endpoint] = struct{}{}
	}
	return len(seen)
}

// splitAuth memecah "user:pass" jadi dua bagian; kosong = tanpa auth.
func splitAuth(s string) (user, pass string) {
	if s == "" {
		return "", ""
	}
	if i := strings.Index(s, ":"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// tunnelCreds cari path kredensial tunnel yang benar: ~/.cloudflared/<id>.json,
// lalu path valid yang sudah tertulis di yml (jangan timpa yml panel dengan placeholder).
func tunnelCreds(tunnelID, ymlPath string) (string, error) {
	if tunnelID != "" {
		home, err := os.UserHomeDir()
		if err == nil {
			p := filepath.Join(home, ".cloudflared", tunnelID+".json")
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, nil
			}
		}
	}
	b, err := os.ReadFile(ymlPath)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "credentials-file:") {
			cand := strings.TrimSpace(strings.TrimPrefix(line, "credentials-file:"))
			if cand != "" {
				if st, err := os.Stat(cand); err == nil && !st.IsDir() {
					return cand, nil
				}
			}
			break
		}
	}
	return "", os.ErrNotExist
}

func (a *app) binds() []config.Bind {
	a.mu.Lock()
	set := a.set
	a.mu.Unlock()
	accs, _, err := config.ParseFile(a.accountsFile())
	if err != nil {
		return nil
	}
	_, binds := config.BuildConfig(accs, set.Listen, set.Base, set.BaseHTTP, set.Auth, set.Insecure, 0)
	return binds
}

// addAccounts mirrors webui /api/add: filter vless, append, rewrite deduped, regen, restart if running.
func (a *app) addAccounts(links []string) map[string]any {
	var ok []string
	for _, l := range links {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if config.ValidLink(l) {
			ok = append(ok, l)
		}
	}
	dup := 0
	if len(ok) > 0 {
		f, err := os.OpenFile(a.accountsFile(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			f.WriteString(strings.Join(ok, "\n") + "\n")
			f.Close()
		}
		dup, _ = config.DedupFile(a.accountsFile())
		a.regen()
		if a.mgr.Running() {
			a.mgr.Restart()
		}
	}
	return map[string]any{
		"added":      len(ok) - dup,
		"skipped":    len(links) - len(ok) + dup,
		"duplicates": dup,
	}
}

// deleteAccount drops the line at a 0-based index (webui.py /del semantics), then regen + restart.
func (a *app) deleteAccount(line int) bool {
	raw, err := os.ReadFile(a.accountsFile())
	if err != nil {
		return false
	}
	lines := strings.Split(string(raw), "\n")
	if line < 0 || line >= len(lines) {
		return false
	}
	lines = append(lines[:line], lines[line+1:]...)
	os.WriteFile(a.accountsFile(), []byte(strings.Join(lines, "\n")), 0o644)
	a.regen()
	if a.mgr.Running() {
		a.mgr.Restart()
	}
	return true
}

// runGen = `vlessbank gen` — pengganti gen.py, tanpa Python.
// Menulis config.json (+ config.tunnel.yml kalau --tunnel) lalu mencetak ringkasan.
func runGen(args []string) int {
	fs := flag.NewFlagSet("gen", flag.ExitOnError)
	out := fs.String("o", "config.json", "file config.json keluaran")
	outLong := fs.String("out", "", "sama dengan -o")
	base := fs.Int("base", 1081, "port SOCKS pertama")
	baseHTTPFlag := fs.Int("base-http", 0, "port HTTP pertama (0 = base+1000)")
	listen := fs.String("listen", "127.0.0.1", "alamat listen inbound")
	auth := fs.String("auth", "", "user:pass untuk SOCKS+HTTP (wajib kalau listen 0.0.0.0)")
	tunnel := fs.String("tunnel", "", "domain tunnel cloudflare (opsional)")
	tunnelID := fs.String("tunnel-id", "", "UUID tunnel cloudflare")
	insecure := fs.Bool("insecure", false, "allowInsecure untuk xray < 26")
	flags, pos := reorderArgs(args, map[string]bool{"-insecure": true, "--insecure": true})
	fs.Parse(flags)

	accounts := "accounts.txt"
	if len(pos) > 0 {
		accounts = pos[0]
	}
	outPath := *out
	if *outLong != "" {
		outPath = *outLong
	}
	if *listen == "0.0.0.0" && *auth == "" {
		fmt.Println("ERROR: --listen 0.0.0.0 wajib pakai --auth (open proxy = IP cepat ke-flag)")
		return 2
	}
	if *auth != "" && !strings.Contains(*auth, ":") {
		fmt.Println("ERROR: format --auth USER:PASS")
		return 2
	}

	accs, dup, err := config.ParseFile(accounts)
	if err != nil {
		fmt.Printf("ERROR: %s tidak ada\n", accounts)
		return 2
	}
	if len(accs) == 0 {
		fmt.Println("ERROR: tidak ada akun valid")
		return 1
	}
	xrayBin := findXrayBin()
	major := config.XrayMajor(xrayBin)
	skipInsecure := *insecure && major >= 26
	if skipInsecure {
		fmt.Printf("NOTE: xray %d.x sudah hapus allowInsecure -> opsi dilewati\n", major)
	}
	baseHTTP := *baseHTTPFlag
	if baseHTTP == 0 {
		baseHTTP = *base + 1000
	}
	cfg, binds := config.BuildConfig(accs, *listen, *base, *baseHTTPFlag, *auth, *insecure, major)
	if err := config.WriteJSON(outPath, cfg); err != nil {
		fmt.Println("ERROR:", err)
		return 1
	}
	if *tunnel != "" {
		yml := "config.tunnel.yml"
		if strings.HasSuffix(outPath, ".json") {
			yml = strings.TrimSuffix(outPath, ".json") + ".tunnel.yml"
		}
		cred, _ := tunnelCreds(*tunnelID, yml)
		if err := os.WriteFile(yml, []byte(config.TunnelYML(*tunnelID, *tunnel, binds, *base, cred)), 0o644); err == nil {
			fmt.Printf("tunnel yml -> %s\n", yml)
		}
	}
	summary := fmt.Sprintf("%d akun -> %s (listen %s, SOCKS %d+, HTTP %d+)",
		len(binds), outPath, *listen, *base, baseHTTP)
	if dup > 0 {
		summary += fmt.Sprintf(", %d duplikat dibuang", dup)
	}
	if *insecure {
		if skipInsecure {
			summary += fmt.Sprintf(", allowInsecure dilewati (xray %d.x)", major)
		} else {
			summary += ", allowInsecure ON"
		}
	}
	fmt.Println(summary)
	if uniq := uniqEndpoints(binds); uniq < len(binds) {
		fmt.Printf("WARN: %d akun tapi cuma %d endpoint upstream unik -> akun dengan endpoint sama akan keluar IP sama\n",
			len(binds), uniq)
	}
	for _, b := range binds {
		fmt.Printf("  SOCKS :%d | HTTP :%d  <-  %s\n", b.SocksPort, b.HTTPPort, b.Name)
	}
	return 0
}

// findXrayBin mencari binary xray: folder kerja -> PATH -> "xray".
func findXrayBin() string {
	wd, _ := os.Getwd()
	cand := filepath.Join(wd, "xray.exe")
	if runtime.GOOS != "windows" {
		cand = filepath.Join(wd, "xray")
	}
	if _, err := os.Stat(cand); err == nil {
		return cand
	}
	return "xray"
}

// runUnzip = `vlessbank unzip <zip> <dir>` — pengganti fallback python3 di install.sh.
func runUnzip(args []string) int {
	if len(args) < 2 {
		fmt.Println("pakai: vlessbank unzip <file.zip> <folder>")
		return 2
	}
	zr, err := zip.OpenReader(args[0])
	if err != nil {
		fmt.Println("ERROR:", err)
		return 1
	}
	defer zr.Close()
	out := args[1]
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Println("ERROR:", err)
		return 1
	}
	for _, f := range zr.File {
		name := filepath.Base(filepath.Clean(f.Name))
		if name == "." || name == ".." || name == "" {
			continue
		}
		dst := filepath.Join(out, name)
		rc, err := f.Open()
		if err != nil {
			fmt.Println("ERROR:", err)
			return 1
		}
		w, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			rc.Close()
			fmt.Println("ERROR:", err)
			return 1
		}
		if _, err := io.Copy(w, rc); err != nil {
			rc.Close()
			w.Close()
			fmt.Println("ERROR:", err)
			return 1
		}
		rc.Close()
		w.Close()
	}
	return 0
}

func main() {
	// subcommand: serve (default) | check | tunnel-client
	cmd := "serve"
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		cmd = os.Args[1]
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}
	switch cmd {
	case "gen":
		os.Exit(runGen(os.Args[1:]))
	case "unzip":
		os.Exit(runUnzip(os.Args[1:]))
	case "check":
		os.Exit(runCheck(os.Args[1:]))
	case "tunnel-client":
		os.Exit(runTunnelClient(os.Args[1:]))
	case "serve":
		runServe(os.Args[1:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Printf("perintah tidak dikenal: %s\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`vlessbank - panel + generator vless (satu binary, stdlib only)

  vlessbank [serve] [--port 9000] [--listen 127.0.0.1] [--xray xray] [--dir FOLDER]
  vlessbank gen [accounts.txt] [-o config.json] [--base 1081] [--base-http 0]
                 [--listen 127.0.0.1] [--auth user:pass] [--tunnel DOMAIN] [--tunnel-id UUID] [--insecure]
  vlessbank check [accounts.txt] [--base 1081] [--timeout 12] [--workers 6]
  vlessbank tunnel-client --domain example.com --accounts 3 [--sub socks] [--dry]
`)
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9000, "port panel web")
	listen := fs.String("listen", "127.0.0.1", "alamat listen panel")
	xrayFlag := fs.String("xray", "xray", "nama/path binary xray")
	dir := fs.String("dir", "", "folder kerja (default: folder binary)")
	fs.Parse(args)

	wd := *dir
	if wd == "" {
		if exe, err := os.Executable(); err == nil {
			wd = filepath.Dir(exe)
		}
		if _, err := os.Stat(filepath.Join(wd, "accounts.txt")); err != nil {
			if c, err2 := os.Getwd(); err2 == nil {
				wd = c
			}
		}
	}
	if abs, err := filepath.Abs(wd); err == nil {
		wd = abs
	}

	a := &app{dir: wd, defaultBin: *xrayFlag}
	cf.SetDir(wd)
	a.set = settings.LoadSettings(a.settingsFile(), *xrayFlag)
	login, err := settings.LoadLogin(a.loginFile())
	if err != nil {
		fmt.Println("ERROR: gagal baca webui.login:", err)
		os.Exit(1)
	}
	a.login = login
	a.sessions = settings.NewSessions(filepath.Join(wd, "webui.sessions"))
	a.mgr = xray.New(a.configFile(), a.xrayLog(), *xrayFlag, a.findXray)

	srv := web.New(web.Deps{
		Dir:         wd,
		Port:        *port,
		DefaultXray: *xrayFlag,
		Settings: func() settings.Settings {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.set
		},
		SaveSettings: func(s settings.Settings) error {
			a.mu.Lock()
			a.set = s
			a.mu.Unlock()
			return settings.SaveSettings(a.settingsFile(), s)
		},
		Login:    login,
		Sessions: a.sessions,
		Xray:     a.mgr,
		Accounts: func() []config.Account {
			accs, _, _ := config.ParseFile(a.accountsFile())
			return accs
		},
		Binds:         a.binds,
		Regen:         a.regen,
		AddAccounts:   a.addAccounts,
		DeleteAccount: a.deleteAccount,
		IPTest: func(port int) string {
			a.mu.Lock()
			user, pass := splitAuth(a.set.Auth)
			a.mu.Unlock()
			return netcheck.IPTestAuth(port, a.mgr.Running(), user, pass, 20*time.Second)
		},
		IPAll: func() map[string]string {
			var ports []int
			for _, b := range a.binds() {
				ports = append(ports, b.SocksPort)
			}
			a.mu.Lock()
			user, pass := splitAuth(a.set.Auth)
			a.mu.Unlock()
			return netcheck.IPAllAuth(ports, a.mgr.Running(), user, pass)
		},
	})

	addr := net.JoinHostPort(*listen, strconv.Itoa(*port))
	httpSrv := &http.Server{Addr: addr, Handler: srv}
	fmt.Printf("vlessbank webui : http://%s/\n", addr)
	fmt.Println("login default   : admin / admin  (SEGERA GANTI di Pengaturan)")
	if *listen == "0.0.0.0" {
		fmt.Println("WARN: listen 0.0.0.0 - pastikan auth aktif, kalau tidak ini open proxy")
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		fmt.Println(a.mgr.Stop())
		httpSrv.Close()
	}()
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Println("ERROR:", err)
		os.Exit(1)
	}
}

// use: reorderArgs memisahkan flag (beserta nilainya) dari argumen positional,
// supaya `check accounts.txt --base 1300` dan `check --base 1300 accounts.txt` sama-sama jalan.
func reorderArgs(args []string, boolFlag map[string]bool) (flags, positional []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		name := a
		if j := strings.Index(a, "="); j >= 0 {
			name = a[:j]
		}
		if strings.Contains(a, "=") || boolFlag[name] {
			continue
		}
		if i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return flags, positional
}

// runCheck: `vlessbank check` — pengganti check-accounts.py, tanpa Python.
func runCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	base := fs.Int("base", 1081, "port SOCKS pertama")
	timeout := fs.Int("timeout", 12, "timeout per akun (detik)")
	workers := fs.Int("workers", 6, "tes paralel")
	xrayFlag := fs.String("xray", "", "path binary xray (default: folder ini lalu PATH)")
	flags, pos := reorderArgs(args, nil)
	fs.Parse(flags)

	wd, _ := os.Getwd()
	xrayBin := *xrayFlag
	if xrayBin == "" {
		cand := filepath.Join(wd, "xray.exe")
		if runtime.GOOS != "windows" {
			cand = filepath.Join(wd, "xray")
		}
		if _, err := os.Stat(cand); err == nil {
			xrayBin = cand
		} else {
			xrayBin = "xray"
		}
	}
	accounts := filepath.Join(wd, "accounts.txt")
	if len(pos) > 0 {
		accounts = pos[0]
	}
	fmt.Printf("cek %s (xray: %s, base %d)\n\n", accounts, xrayBin, *base)
	return diag.Run(accounts, xrayBin, *base, time.Duration(*timeout)*time.Second, *workers, os.Stdout)
}

// runTunnelClient: `vlessbank tunnel-client` — pengganti tunnel-client.py.
func runTunnelClient(args []string) int {
	fs := flag.NewFlagSet("tunnel-client", flag.ExitOnError)
	domain := fs.String("domain", "", "zone/domain tunnel, mis. example.com (wajib)")
	accounts := fs.Int("accounts", 0, "jumlah akun (wajib)")
	sub := fs.String("sub", "", "subdomain opsional; kosong = hostname 1 level")
	socksBase := fs.Int("socks-base", 7001, "port lokal SOCKS pertama")
	httpBase := fs.Int("http-base", 8001, "port lokal HTTP pertama")
	noHTTP := fs.Bool("no-http", false, "hanya listener SOCKS")
	auth := fs.String("auth", "", "user:pass untuk URI yang dicetak")
	cfBin := fs.String("cloudflared", "cloudflared", "path/command cloudflared")
	dry := fs.Bool("dry", false, "cetak tabel + perintah saja")
	flags, pos := reorderArgs(args, map[string]bool{"-dry": true, "--dry": true,
		"-no-http": true, "--no-http": true})
	fs.Parse(flags)
	if len(pos) > 0 {
		fmt.Printf("argumen tidak dikenal: %s\n", strings.Join(pos, " "))
	}

	if *domain == "" || *accounts < 1 {
		fmt.Println("ERROR: --domain dan --accounts wajib (lihat: vlessbank help)")
		return 2
	}
	jobs := tunclient.Build(*domain, *sub, *accounts, *socksBase, *httpBase, *noHTTP)
	tunclient.Print(jobs, *auth, os.Stdout)
	if *dry {
		fmt.Println()
		for _, j := range jobs {
			fmt.Printf("+ cloudflared access tcp --hostname %s --url 127.0.0.1:%d\n", j.Host, j.Port)
		}
		return 0
	}
	if err := tunclient.Run(*cfBin, jobs, os.Stdout); err != nil {
		fmt.Println("ERROR:", err)
		return 1
	}
	return 0
}
