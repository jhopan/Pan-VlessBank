package main

import (
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"

	"vlessbank/internal/client"
)

var page = template.Must(template.New("p").Parse(`<!doctype html><meta name="viewport" content="width=device-width,initial-scale=1"><title>Pan-VlessBank Client</title><style>body{font:14px system-ui;margin:32px;max-width:900px}input{width:100%;box-sizing:border-box;padding:8px;margin:4px 0 12px}button{padding:9px 14px;margin:4px}table{border-collapse:collapse;width:100%;margin-top:16px}th,td{border:1px solid #ccc;padding:8px;text-align:left}code{word-break:break-all}</style><h1>Pan-VlessBank Client</h1><p>Panel hanya di loopback. Cloudflared Access TCP harus login di mesin ini.</p><form method=post action=/save><label>Domain proxy penuh</label><input name=domain value="{{.Config.Domain}}" placeholder="proxy.example.com"><label>Jumlah akun</label><input name=accounts value="{{.Config.Accounts}}"><label>Cloudflared</label><input name=cloudflared value="{{.Config.Cloudflared}}"><label>SOCKS base</label><input name=socks value="{{.Config.SocksBase}}"><label>HTTP base</label><input name=http value="{{.Config.HTTPBase}}"><button>Simpan</button></form><form method=post action=/start style="display:inline"><button>Start Listener</button></form><form method=post action=/stop style="display:inline"><button>Stop Semua</button></form>{{if .Message}}<p><b>{{.Message}}</b></p>{{end}}<table><tr><th>Akun</th><th>Jenis</th><th>Cloudflare Hostname</th><th>Port lokal</th><th>State</th></tr>{{range .Rows}}<tr><td>{{.Index}}</td><td>{{.Kind}}</td><td><code>{{.Host}}</code></td><td><code>{{.Port}}</code></td><td>{{.State}} {{.Error}}</td></tr>{{end}}</table>`))

type app struct {
	dir string
	cfg client.Config
	mgr *client.Manager
	msg string
}

func (a *app) render(w http.ResponseWriter) {
	_ = page.Execute(w, map[string]any{"Config": a.cfg, "Rows": a.mgr.Snapshot(), "Message": a.msg})
	a.msg = ""
}
func main() {
	dir, _ := os.UserConfigDir()
	dir = filepath.Join(dir, "pan-vlessbank-client")
	_ = os.MkdirAll(dir, 0700)
	p := flag.Int("port", 9900, "port panel lokal")
	domain := flag.String("domain", "", "domain proxy")
	accounts := flag.Int("accounts", 0, "jumlah akun")
	cf := flag.String("cloudflared", "cloudflared", "cloudflared path")
	flag.Parse()
	cfg, err := client.Load(client.ConfigPath(dir))
	if err != nil {
		cfg = client.Config{Domain: *domain, Accounts: *accounts, Cloudflared: *cf, SocksBase: 7001, HTTPBase: 8001, Listen: "127.0.0.1"}
	}
	if *domain != "" {
		cfg.Domain = *domain
	}
	if *accounts > 0 {
		cfg.Accounts = *accounts
	}
	a := &app{dir: dir, cfg: cfg, mgr: client.New(cfg)}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { a.render(w) })
	http.HandleFunc("/save", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		fmt.Sscanf(r.FormValue("accounts"), "%d", &a.cfg.Accounts)
		fmt.Sscanf(r.FormValue("socks"), "%d", &a.cfg.SocksBase)
		fmt.Sscanf(r.FormValue("http"), "%d", &a.cfg.HTTPBase)
		a.cfg.Domain = r.FormValue("domain")
		a.cfg.Cloudflared = r.FormValue("cloudflared")
		a.cfg.Listen = "127.0.0.1"
		if e := client.Save(client.ConfigPath(a.dir), a.cfg); e != nil {
			a.msg = e.Error()
		} else {
			a.mgr = client.New(a.cfg)
			a.msg = "konfigurasi disimpan"
		}
		http.Redirect(w, r, "/", 303)
	})
	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		if e := a.mgr.Start(); e != nil {
			a.msg = e.Error()
		} else {
			a.msg = "listener dimulai, login Cloudflare Access bila browser meminta"
		}
		http.Redirect(w, r, "/", 303)
	})
	http.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
		a.mgr.Stop()
		a.msg = "semua listener dihentikan"
		http.Redirect(w, r, "/", 303)
	})
	addr := fmt.Sprintf("127.0.0.1:%d", *p)
	fmt.Println("Pan-VlessBank Client:", "http://"+addr)
	if e := http.ListenAndServe(addr, nil); e != nil {
		fmt.Println(e)
	}
}
