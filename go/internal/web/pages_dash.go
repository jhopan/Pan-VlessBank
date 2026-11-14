package web

// pages_dash.go: body_dash() dari webui.py.

import "strconv"

func (s *Server) bodyDash() string {
	set := s.set()
	stat := `<span class="badge expired" id="xstat">MATI</span>`
	if s.xrayRunning() {
		stat = `<span class="badge paid" id="xstat">RUNNING</span>`
	}
	rs := s.rows()
	tun := `<span class="badge expired">BELUM</span>`
	if set.Tunnel != "" {
		tun = `<span class="badge paid">` + esc(set.Tunnel) + `</span>`
	}
	cfgstat := `<b style="color:var(--err)">belum ada - klik Regenerate</b>`
	if fileExists(s.cfgPath()) {
		cfgstat = `<code>sudah ada</code>`
	}
	portRange := itoa(set.Base)
	if len(rs) > 0 {
		portRange = itoa(set.Base) + "-" + itoa(set.Base+len(rs)-1)
	}
	portRangeH := strconv.Itoa(set.Base + 1000)
	if set.BaseHTTP != 0 {
		portRangeH = strconv.Itoa(set.BaseHTTP)
	}
	if len(rs) > 0 {
		start, _ := strconv.Atoi(portRangeH)
		portRangeH = strconv.Itoa(start) + "-" + strconv.Itoa(start+len(rs)-1)
	}
	lan := lanIP()
	authLine := `<span class="badge expired">tanpa auth</span>`
	if set.Auth != "" {
		authLine = `<span class="badge paid">aktif</span>`
	}
	return fill(dashTmpl,
		"@@NA@@", itoa(len(rs)),
		"@@STAT@@", stat,
		"@@RANGE@@", portRange,
		"@@CFGSTAT@@", cfgstat,
		"@@LISTEN@@", esc(set.Listen),
		"@@RANGEH@@", portRangeH,
		"@@TUN@@", tun,
		"@@LAN@@", esc(lan),
		"@@AUTH@@", authLine)
}

type dashTmplT struct{}

const dashTmpl = `<div class="stats">
<div class="stat"><div class="n">@@NA@@</div><div class="l">Akun</div></div>
<div class="stat"><div class="n">@@STAT@@</div><div class="l">Xray</div></div>
<div class="stat"><div class="n" style="font-size:14px">@@TUN@@</div><div class="l">Tunnel</div></div>
</div>
<div id="msg" class="flash" style="display:none"></div>
<div class="card"><h2>Kontrol</h2><div class="btnrow">
<button onclick="xray('start')">Start</button>
<button class="sec" onclick="xray('restart')">Restart</button>
<button class="del" onclick="xray('stop')">Stop</button>
<button class="sec" onclick="regen()">Regenerate config</button>
</div><div class="hint">config.json @@CFGSTAT@@</div></div>
<div class="card"><h2>Ringkasan</h2>
<table>
<tr><th>IP LAN</th><td><code>@@LAN@@</code></td></tr>
<tr><th>Bind proxy</th><td><code>@@LISTEN@@</code>, SOCKS <code>@@RANGE@@</code>, HTTP <code>@@RANGEH@@</code></td></tr>
<tr><th>Auth proxy</th><td>@@AUTH@@</td></tr>
<tr><th>Tunnel</th><td>@@TUN@@</td></tr>
</table>
<div class="btnrow" style="margin-top:10px">
<a href="/admin/akun" onclick="nav(event,'/admin/akun')"><button>Kelola Akun →</button></a>
<button class="sec" onclick="regen()">Regenerate config</button>
</div>
<div class="hint">URL SOCKS/HTTP per akun (LAN + publik) ada di menu <b>Akun</b>.</div></div>`
