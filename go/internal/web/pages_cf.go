package web

// pages_cf.go: body_cf() dari webui.py (varian yang benar-benar dirender; varian kedua
// di webui.py baris 969-1141 tak terjangkau karena berada setelah return).

import (
	"strings"

	"vlessbank/internal/cf"
)

func (s *Server) bodyCF() string {
	set := s.set()
	st := cf.Status()
	dl := cf.DownloadStatus()
	pi := cf.PlatformInfo()

	dlFname, err := cf.BundleName()
	if err != nil {
		dlFname = ""
	}
	dlURL := ""
	if dlFname != "" {
		dlURL = cf.CFCDN + dlFname
	}

	var tunRows strings.Builder
	for _, t := range asList(st["tunnels"]) {
		tid := asString(t["id"])
		style, aktif := "", ""
		if tid == set.TunnelID {
			style = ` style="background:var(--ok-soft)"`
			aktif = " ✓"
		}
		tunRows.WriteString(fill(cfTunnelRowTmpl,
			"@@STYLE@@", style,
			"@@TID@@", tid,
			"@@NM@@", esc(asString(t["name"])),
			"@@AKTIF@@", aktif))
	}
	tunnelRows := tunRows.String()
	if tunnelRows == "" {
		tunnelRows = `<tr><td colspan="3" class="empty">Belum ada tunnel</td></tr>`
	}

	state := asString(dl["state"])
	progress := asInt(dl["progress"])
	version := asString(dl["version"])
	binBadge := `<span class="badge expired">BELUM ADA</span>`
	switch state {
	case "done":
		binBadge = `<span class="badge paid">ADA · ` + esc(version) + `</span>`
	case "downloading":
		binBadge = `<span class="badge pending">Mengunduh ` + itoa(progress) + `%...</span>`
	case "error":
		binBadge = `<span class="badge expired">Error download</span>`
	}

	loginSt := cf.LoginStatus()
	loginRunning := asBool(loginSt["running"])
	loginURL := asString(loginSt["url"])
	loginDone := asBool(loginSt["done"])
	loginBadge := `<span class="badge pending">Belum login / siap login baru</span>`
	certNote := ""
	if loginRunning {
		loginBadge = `<span class="badge paid">Proses login aktif</span>`
	} else if asBool(loginSt["cert"]) {
		loginBadge = `<span class="badge paid">Sudah login (cert.pem ada)</span>`
		certNote = `<div class="hint" style="color:var(--ok-text);margin-top:8px">✓ Kredensial Cloudflare sudah tersimpan di <code>` +
			esc(cf.CertPath()) + `</code>, tunnel yang sudah ada bisa langsung dipakai (step 3). Login ulang hanya kalau ganti akun.</div>`
	}
	tunBadge := `<span class="badge expired">MATI</span>`
	if asBool(st["running"]) {
		tunBadge = `<span class="badge paid">RUNNING</span>`
	}
	ymlOK := asBool(st["yml_ok"])

	ymlPreview := ""
	if ymlOK {
		pv := cf.YMLPreview()
		if asBool(pv["ok"]) {
			ymlPreview = `<pre style="margin-top:10px;max-height:180px">` + esc(asString(pv["content"])) + `</pre>`
		}
	}

	dlNote := `<p style="font-size:13px;margin-top:8px">cloudflared belum ada. Download otomatis atau manual.</p>`
	if state == "done" {
		dlNote = `<div class="hint" style="color:var(--ok-text);margin-top:6px">cloudflared sudah ada · ` + esc(version) + `</div>`
	}
	dlBtn := ""
	if state != "done" && state != "downloading" {
		dlBtn = `<button class="sec" onclick="cfDownload()">Download Otomatis</button>`
	}
	dlPct := ""
	if state == "downloading" {
		dlPct = `<div class="hint">Sedang mengunduh... <span id="dlpct">` + itoa(progress) + `</span>%</div>`
	}
	dlMan := ""
	if dlURL != "" && state != "done" && state != "downloading" {
		dlMan = `<a href="` + esc(dlURL) + `" target="_blank"><button class="sec">Download Manual (` +
			esc(dlFname) + `) ↗</button></a>`
	}
	dlErr := ""
	if state == "error" {
		dlErr = `<div class="hint" style="color:var(--err)">Error: ` + esc(asString(dl["error"])) +
			`<br>Download manual lalu taruh di: <code>` + esc(asString(pi["bundle"])) + `</code></div>`
	}
	binDisabled := " "
	if !cf.BinOK() {
		binDisabled = " disabled"
	}
	cancelBtn := ""
	if loginRunning {
		cancelBtn = `<button class="sec" onclick="cfCancelLogin()">Batal</button>`
	}
	loginDisplay := "none"
	if loginRunning {
		loginDisplay = "block"
	}
	loginURLHTML := `Menunggu URL...`
	if loginURL != "" {
		loginURLHTML = `<a href="` + loginURL + `" target="_blank" style="color:var(--accent);font-weight:600">` +
			loginURL + `</a>`
	}
	loginDoneNote := ""
	if loginDone {
		loginDoneNote = `<div class="hint" style="color:var(--ok-text);margin-top:8px">✓ Login berhasil, lanjut ke step 3</div>`
	}
	loginDisabled := " "
	if !loginDone {
		loginDisabled = " disabled"
	}

	dnsTable := `<table><tr><th>Type</th><th>Name</th><th>Target</th><th>Proxy</th></tr><tr><td>CNAME</td><td><code>*.` +
		esc(set.TunnelID) + `.cfargotunnel.com</code>... (isi setelah pilih domain)</td><td></td><td><b style="color:var(--ok-text)">ON</b></td></tr></table>`
	if set.Tunnel != "" {
		dnsTable = `<table><tr><th>Type</th><th>Name</th><th>Target</th><th>Proxy</th></tr><tr><td>CNAME</td><td><code>*.` +
			esc(set.Tunnel) + `</code></td><td><code>` + esc(set.TunnelID) +
			`.cfargotunnel.com</code></td><td><b style="color:var(--ok-text)">ON</b></td></tr></table>`
	}

	ymlState := `<b style="color:var(--err)">belum ada, selesaikan step 4 dulu</b>`
	if ymlOK {
		ymlState = `<span style="color:var(--ok-text)">✓ ada</span>`
	}
	ymlDisabled := " "
	if !ymlOK {
		ymlDisabled = " disabled"
	}

	logHTML := esc(cf.TailLog(30))
	if logHTML == "" {
		logHTML = "(kosong)"
	}

	webHost := set.WebTunnelHost
	if webHost == "" {
		webHost = "panel"
	}
	webHostname := "panel.<domain>"
	if set.Tunnel != "" {
		webHostname = webHost + "." + set.Tunnel
	}
	webState := `<span class="badge expired">BELUM</span>`
	if cf.WebTunnelRunning() {
		webState = `<span class="badge paid">RUNNING</span>`
	}
	webCreateDisabled, webGenerateDisabled, webStartDisabled := "", " disabled", " disabled"
	if set.Tunnel == "" || !cf.BinOK() {
		webCreateDisabled = " disabled"
	}
	if set.WebTunnelID != "" {
		webGenerateDisabled = ""
	}
	if set.WebTunnelID != "" && fileExists(s.webTunPath()) {
		webStartDisabled = ""
	}

	return fill(cfTmpl,
		"@@BINBADGE@@", binBadge,
		"@@LOGINBADGE@@", loginBadge,
		"@@TUNBADGE@@", tunBadge,
		"@@OS@@", esc(asString(pi["os"])),
		"@@ARCH@@", esc(asString(pi["arch"])),
		"@@BIN@@", esc(asString(pi["bin"])),
		"@@DLNOTE@@", dlNote,
		"@@DLBTN@@", dlBtn,
		"@@DLPCT@@", dlPct,
		"@@DLMAN@@", dlMan,
		"@@DLERR@@", dlErr,
		"@@BINDISABLED@@", binDisabled,
		"@@CANCELBTN@@", cancelBtn,
		"@@LOGINBOX@@", loginDisplay,
		"@@LOGINURL@@", loginURLHTML,
		"@@LOGINDONE@@", loginDoneNote+certNote,
		"@@TUNROWS@@", tunnelRows,
		"@@LOGINDISABLED@@", loginDisabled,
		"@@MANDOMAIN@@", esc(set.Tunnel),
		"@@YMLPREVIEW@@", ymlPreview,
		"@@DNSTABLE@@", dnsTable,
		"@@YMLSTATE@@", ymlState,
		"@@YMLDISABLED@@", ymlDisabled,
		"@@LOGHTML@@", logHTML,
		"@@WEBPORT@@", itoa(s.d.Port),
		"@@WEBHOST@@", esc(webHostname),
		"@@WEBSTATE@@", webState,
		"@@WEBTUNNELID@@", esc(set.WebTunnelID),
		"@@WEB_CREATEDISABLED@@", webCreateDisabled,
		"@@WEB_GENERATEDISABLED@@", webGenerateDisabled,
		"@@WEB_STARTDISABLED@@", webStartDisabled,
		"@@SCRIPT@@", cfScript)
}

const cfTunnelRowTmpl = `<tr@@STYLE@@><td><code style="font-size:11px">@@TID@@</code></td>` +
	`<td><b>@@NM@@</b>@@AKTIF@@</td>` +
	`<td style="white-space:nowrap">` +
	`<button class="sec" onclick="cfUseTunnel('@@TID@@','@@NM@@')">Pakai</button> ` +
	`<button class="del" onclick="cfDelTunnel('@@TID@@','@@NM@@')">Hapus</button>` +
	`</td></tr>`
