package web

// pages_akun.go: halaman "Akun", satu kartu per akun, rapi dan gampang di-copy:
// URL SOCKS5 + HTTP (LAN & publik), status IP keluar, tombol test/hapus.

import "strings"

func (s *Server) bodyAkun() string {
	set := s.set()
	rs := s.rows()
	lan := lanIP()
	cred := ""
	if set.Auth != "" {
		cred = set.Auth + "@"
	}
	var cards strings.Builder
	for _, x := range rs {
		pub := ""
		if set.Tunnel != "" {
			n := itoa(x.Idx + 1)
			pub = fill(akunPubChip,
				"@@PSHOST@@", esc("socks"+n+"."+set.Tunnel),
				"@@PHHOST@@", esc("http"+n+"."+set.Tunnel),
				"@@ACCS@@", esc("cloudflared access tcp --hostname socks"+n+"."+set.Tunnel+
					" --url 127.0.0.1:"+itoa(7000+x.Idx+1)),
				"@@ACCH@@", esc("cloudflared access tcp --hostname http"+n+"."+set.Tunnel+
					" --url 127.0.0.1:"+itoa(8000+x.Idx+1)))
		}
		cards.WriteString(fill(akunCardTmpl,
			"@@IDX@@", itoa(x.Idx+1),
			"@@NM@@", esc(x.Name),
			"@@SERVER@@", esc(x.Server),
			"@@TIPE@@", x.Tipe),
		)
		cards.WriteString(fill(akunChipsTmpl,
			"@@PS@@", itoa(x.Socks),
			"@@PH@@", itoa(x.HTTP),
			"@@SOCKSURL@@", esc("socks5://"+cred+lan+":"+itoa(x.Socks)),
			"@@HTTPURL@@", esc("http://"+cred+lan+":"+itoa(x.HTTP)),
			"@@PUB@@", pub))
		cards.WriteString(fill(akunFootTmpl,
			"@@PS@@", itoa(x.Socks),
			"@@LINE@@", itoa(x.Line),
			"@@NM@@", esc(x.Name)))
	}
	empty := ""
	if len(rs) == 0 {
		empty = `<div class="card"><div class="empty">Belum ada akun, tempel link <code>vless://</code> di bawah.</div></div>`
	}
	tunnelInfo := `<span class="badge expired">tunnel belum diatur</span>, buka menu <a href="/admin/cf" onclick="nav(event,'/admin/cf')">Cloudflare</a> supaya ada URL publik.`
	if set.Tunnel != "" {
		tunnelInfo = `<span class="badge paid">tunnel ` + esc(set.Tunnel) + `</span>, hostname publik: <code>socksN.` +
			esc(set.Tunnel) + `</code> / <code>httpN.` + esc(set.Tunnel) + `</code>`
	}
	authInfo := `<span class="badge expired">tanpa auth</span>`
	if set.Auth != "" {
		authInfo = `<code>` + esc(set.Auth) + `</code>`
	}
	return fill(akunTmpl,
		"@@NA@@", itoa(len(rs)),
		"@@LAN@@", esc(lan),
		"@@LISTEN@@", esc(set.Listen),
		"@@AUTH@@", authInfo,
		"@@TUNNELINFO@@", tunnelInfo,
		"@@CARDS@@", cards.String(),
		"@@EMPTY@@", empty)
}

const akunCardTmpl = `<div class="akun-card">
<div class="akun-head">
<span class="akun-idx">@@IDX@@</span>
<span class="akun-name">@@NM@@</span>
<span class="akun-src">@@SERVER@@ · @@TIPE@@</span>
</div>`

const akunChipsTmpl = `<div class="chips">
<div class="chip"><span class="chip-labels"><span class="tag">SOCKS5</span><span class="tag lan">LAN</span></span><code>@@SOCKSURL@@</code><button class="cp" onclick="cp(event)">copy</button></div>
<div class="chip"><span class="chip-labels"><span class="tag">HTTP</span><span class="tag lan">LAN</span></span><code>@@HTTPURL@@</code><button class="cp" onclick="cp(event)">copy</button></div>
@@PUB@@
</div>`

const akunPubChip = `<div class="chip"><span class="chip-labels"><span class="tag">SOCKS5</span><span class="tag pub">PUBLIK</span></span><code>@@PSHOST@@</code><button class="cp" onclick="cp(event)">copy</button></div>
<div class="chip"><span class="chip-labels"><span class="tag">HTTP</span><span class="tag pub">PUBLIK</span></span><code>@@PHHOST@@</code><button class="cp" onclick="cp(event)">copy</button></div>
<div class="chip sub"><span class="chip-labels"><span class="tag">AKSES</span></span><code>@@ACCS@@</code><button class="cp" onclick="cp(event)">copy</button></div>
<div class="chip sub"><span class="chip-labels"><span class="tag">AKSES</span></span><code>@@ACCH@@</code><button class="cp" onclick="cp(event)">copy</button></div>`

const akunFootTmpl = `<div class="akun-foot">
<span class="ip" id="ip@@PS@@"><small>IP keluar: belum dites</small></span>
<span class="spacer"></span>
<button class="sec" onclick="t(@@PS@@)">Test IP</button>
<button class="del" onclick="del(@@LINE@@,'@@NM@@')">Hapus</button>
</div>
</div>`

const akunTmpl = `<div class="stats">
<div class="stat"><div class="n">@@NA@@</div><div class="l">Akun</div></div>
<div class="stat"><div class="n" style="font-size:15px"><code>@@LAN@@</code></div><div class="l">IP LAN</div></div>
<div class="stat"><div class="n" style="font-size:14px">@@AUTH@@</div><div class="l">Auth proxy</div></div>
</div>
<div id="msg" class="flash" style="display:none"></div>
<div class="card"><h2>Daftar Akun</h2>
<div class="hint" style="margin-top:0">@@TUNNELINFO@@ &nbsp;·&nbsp; bind <code>@@LISTEN@@</code></div>
<div class="btnrow" style="margin:10px 0 14px"><button class="sec" onclick="tall()">Test Semua IP</button>
<span id="tallstat" class="hint"></span></div>
<div class="akun-grid">@@EMPTY@@@@CARDS@@</div>
</div>
<div class="card"><h2>Tambah Akun</h2>
<textarea id="newlinks" placeholder="tempel link vless:// di sini, satu baris satu akun"></textarea>
<button style="margin-top:8px" onclick="add()">Tambah</button>
<div class="hint">1 baris = 1 akun = 1 IP keluar. Akun baru otomatis dapat port SOCKS/HTTP berikutnya.</div></div>`
