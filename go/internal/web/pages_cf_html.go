package web

// pages_cf_html.go: kerangka HTML body_cf() yang dirender (webui.py baris 710-803).

const cfTmpl = `
<div class="stats">
<div class="stat"><div class="n" style="font-size:13px">@@BINBADGE@@</div><div class="l">Binary</div></div>
<div class="stat"><div class="n" style="font-size:13px">@@LOGINBADGE@@</div><div class="l">Login CF</div></div>
<div class="stat"><div class="n" style="font-size:13px">@@TUNBADGE@@</div><div class="l">Tunnel</div></div>
</div>
<div id="cfmsg" class="flash" style="display:none"></div>

<div class="card"><h2>1 · Binary cloudflared</h2>
<div class="hint">Platform: <code>@@OS@@</code> / <code>@@ARCH@@</code> &nbsp;·&nbsp; Lokasi: <code>@@BIN@@</code></div>
@@DLNOTE@@
<div class="btnrow" style="margin-top:8px">
@@DLBTN@@
@@DLPCT@@
@@DLMAN@@
</div>
@@DLERR@@
</div>

<div class="card"><h2>2 · Login Cloudflare</h2>
<p style="font-size:13px;color:var(--muted)">Login ke akun Cloudflare kamu. Link login akan muncul di sini, klik, authorize di browser, lanjut ke step 3.</p>
<div class="btnrow">
<button onclick="cfLogin()"@@BINDISABLED@@>Login ke Cloudflare</button>
@@CANCELBTN@@
</div>
<div id="cfloginbox" style="margin-top:14px;display:@@LOGINBOX@@">
  <div style="background:var(--warn-soft);border:1px solid var(--warn);border-radius:8px;padding:14px">
    <div style="font-size:13px;color:var(--muted);margin-bottom:8px"><b>Klik link berikut untuk login:</b></div>
    <div id="cfloginurl" style="word-break:break-all;font-size:13px">
      @@LOGINURL@@
    </div>
    <div style="font-size:12px;color:var(--muted-2);margin-top:8px">Setelah authorize di browser, status di sini akan update otomatis.</div>
  </div>
</div>
@@LOGINDONE@@
</div>

<div class="card"><h2>3 · Buat atau Pilih Tunnel</h2>
<label>Nama tunnel baru</label>
<div class="btnrow">
<input id="tname" placeholder="vlessbank" style="width:220px">
<button onclick="cfCreate()">Buat Tunnel</button>
</div>
<div style="margin-top:14px">
<table><tr><th>Tunnel ID</th><th>Nama</th><th></th></tr>@@TUNROWS@@</table>
<button class="sec" style="margin-top:8px" onclick="cfRefreshTunnels()">↻ Refresh</button>
</div></div>

<div class="card"><h2>4 · Setup Domain & Generate Config</h2>
<p style="font-size:13px;color:var(--muted)">Masukkan CF API Token untuk load daftar domain, lalu pilih domain dan generate config tunnel.</p>
<div class="hint" style="margin-bottom:10px">Buat token di <a href="https://dash.cloudflare.com/profile/api-tokens" target="_blank" style="color:var(--accent)">CF dashboard → API Tokens</a> dengan permission <b>Zone:DNS:Edit</b>.</div>
<div class="btnrow">
<input id="cftoken" type="password" placeholder="CF API Token" style="width:320px">
<button class="sec" onclick="cfLoadZones()">Load Domain</button>
</div>
<div id="zonebox" style="margin-top:12px;display:none">
  <label>Pilih domain</label>
  <select id="zoneSel" style="width:300px"><option value="">-- pilih --</option></select>
  <label style="margin-top:10px">Subdomain prefix (opsional)</label>
  <input id="subdom" value="" placeholder="kosong = 1 level" style="width:140px">
  <div class="hint">Kosongkan untuk hostname 1 level: <code>socks1.domain.com</code>, cert Universal SSL free plan cuma cover 1 level. Isi <code>socks</code> hanya kalau zone-nya <code>socks.domain.com</code>.</div>
  <div class="btnrow" style="margin-top:12px">
    <button onclick="cfGenYml()">Generate config.tunnel.yml</button>
    <button class="sec" onclick="cfCreateDNS()">Buat DNS Wildcard via API</button>
  </div>
</div>
<div style="margin-top:10px">
  <label>Atau isi manual:</label>
  <div class="btnrow">
    <input id="manDomain" placeholder="domain.com" style="width:220px" value="@@MANDOMAIN@@">
    <button class="sec" onclick="cfGenYmlManual()">Generate (manual)</button>
  </div>
</div>
@@YMLPREVIEW@@
</div>

<div class="card"><h2>5 · DNS Wildcard</h2>
<p style="font-size:13px;color:var(--muted)">Satu CNAME wildcard cover semua akun sekarang dan nanti. Buat via tombol di step 4, atau manual:</p>
@@DNSTABLE@@
<div class="hint">Proxy <b>harus ON</b> (orange). Record DNS-only membuat hostname tidak resolve, cloudflared access gagal "no such host".</div>
</div>

<div class="card"><h2>6 · Onlinekan WebUI</h2>
<p style="font-size:13px;color:var(--muted)">Tunnel Cloudflare hanya untuk panel ini. Proxy akun dan tunnel proxy tidak diubah.</p>
<div class="hint">LAN: <a href="@@WEBLANURL@@" target="_blank"><code>@@WEBLANURL@@</code></a></div>
<div class="hint">Publik: <a href="@@WEBPUBLICURL@@" target="_blank"><code>@@WEBPUBLICURL@@</code></a> · @@WEBSTATE@@ · connector: @@WEBCONNECTIONS@@</div>
<label>Hostname publik penuh</label>
<input id="webhostname" value="@@WEBHOST@@" placeholder="panvlessbank.example.com" style="width:100%">
<div class="btnrow" style="margin-top:12px">
  <button class="sec" onclick="cfWebTunnel('create')"@@WEB_CREATEDISABLED@@>Buat Tunnel WebUI</button>
  <button class="sec" onclick="cfWebTunnel('generate')"@@WEB_GENERATEDISABLED@@>Simpan Config</button>
  <button class="sec" onclick="cfWebDNS()"@@WEB_GENERATEDISABLED@@>Hubungkan Hostname</button>
  <button onclick="cfWebTunnel('start')"@@WEB_STARTDISABLED@@>Start WebUI</button>
  <button class="del" onclick="cfWebTunnel('stop')">Stop WebUI</button>
</div>
<div class="hint" style="margin-top:10px">Tunnel ID: <code>@@WEBTUNNELID@@</code>. Hubungkan hostname memakai login cloudflared, tanpa API token.</div>
</div>

<div class="card"><h2>7 · Jalankan Tunnel Proxy</h2>
<p style="font-size:13px;color:var(--muted)">config.tunnel.yml: @@YMLSTATE@@. Cloudflare Access opsional, tunnel tetap dapat dipakai via listener cloudflared client tanpa policy Access.</p>
<div class="btnrow">
<button onclick="cfTunnel('start')"@@YMLDISABLED@@>▶ Start Tunnel</button>
<button class="del" onclick="cfTunnel('stop')">■ Stop Tunnel</button>
<button class="sec" onclick="cfTunnelDiagnostics()">Cek Tunnel</button>
</div>
<pre id="cftdiag" hidden></pre>
<div style="margin-top:14px"><b style="font-size:13px">Log cloudflared:</b><pre id="cflog">@@LOGHTML@@</pre></div>
</div>

<script>
@@SCRIPT@@
</script>`
