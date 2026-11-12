package web

// pages_settings.go: body_settings() dari webui.py.

func (s *Server) bodySettings() string {
	set := s.set()
	bh := set.BaseHTTP
	if bh == 0 {
		bh = set.Base + 1000
	}
	sel := func(v string) string {
		if set.Listen == v {
			return " selected"
		}
		return " "
	}
	checked := " "
	if set.Insecure {
		checked = " checked"
	}
	s.loginMu.RLock()
	user := s.login.User
	s.loginMu.RUnlock()
	return fill(settingsTmpl,
		"@@XRAYPATH@@", esc(set.XrayPath),
		"@@BASE@@", itoa(set.Base),
		"@@BH@@", itoa(bh),
		"@@SEL_LOCAL@@", sel("127.0.0.1"),
		"@@SEL_LAN@@", sel("0.0.0.0"),
		"@@AUTHV@@", esc(set.Auth),
		"@@CHECKED@@", checked,
		"@@TUNNEL@@", esc(set.Tunnel),
		"@@TUNNELID@@", esc(set.TunnelID),
		"@@USER@@", esc(user))
}

const settingsTmpl = `
<div class="card"><h2>Xray</h2>
<form method="post" action="/settings">
<label>Lokasi binary xray</label>
<input name="xray_path" value="@@XRAYPATH@@" style="width:100%" placeholder="C:/.../xray.exe atau xray">
<label>Port SOCKS awal (base)</label>
<input name="base" type="number" value="@@BASE@@" style="width:140px">
<label>Port HTTP proxy awal (base_http, 0 = SOCKS+1000)</label>
<input name="base_http" type="number" value="@@BH@@" style="width:140px">
<label>SOCKS+HTTP listen</label>
<select name="listen">
<option value="127.0.0.1"@@SEL_LOCAL@@>127.0.0.1 (lokal saja)</option>
<option value="0.0.0.0"@@SEL_LAN@@>0.0.0.0 (LAN+publik - wajib auth)</option>
</select>
<label>Auth user:pass untuk SOCKS+HTTP (kosong = tanpa auth)</label>
<input name="auth" value="@@AUTHV@@" style="width:280px" placeholder="user:pass">
<label style="margin-top:12px"><input type="checkbox" name="insecure" value="1"@@CHECKED@@ style="width:auto;margin-right:6px">allowInsecure (abaikan sertifikat TLS tidak valid)</label>
<label>Domain tunnel Cloudflare (kosong = mode tunnel mati)</label>
<input name="tunnel" value="@@TUNNEL@@" style="width:100%" placeholder="socks.example.com">
<label>UUID tunnel Cloudflare</label>
<input name="tunnel_id" value="@@TUNNELID@@" style="width:100%" placeholder="opsional">
<button style="margin-top:12px">Simpan Semua</button>
</form>
<div class="hint">Perubahan baru efektif setelah klik Regenerate config (+ Restart xray).</div></div>
<div class="card"><h2>Login WebUI</h2>
<form method="post" action="/pass">
<label>Username</label>
<input name="user" value="@@USER@@" style="width:280px">
<label>Password baru (kosong = tetap)</label>
<input name="pass" type="password" style="width:280px">
<button style="margin-top:12px">Simpan Login</button>
</form></div>`
