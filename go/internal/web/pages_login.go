package web

// pages_login.go: login_page() dan status_page() dari webui.py.

// loginPage = webui.login_page(msg).
func (s *Server) loginPage(msg string) string {
	m := ""
	if msg != "" {
		m = `<p class="e">` + esc(msg) + `</p>`
	}
	return fill(loginTmpl, "@@M@@", m)
}

const loginTmpl = `<!doctype html><html lang="id"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>Login, Vlessbank</title>
<style>:root{--bg:#0f1115;--surface:#161a21;--surface-2:#1b2027;--border:#2a3038;--text:#e6e9ef;--muted:#9aa4b2;--accent:#3b82f6;--accent-ink:#0b1220;--err:#f87171;--err-soft:#3a1a1c}
body{font-family:system-ui;background:var(--bg);color:var(--text);display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0}
.c{background:var(--surface);border:1px solid var(--border);padding:36px;border-radius:16px;width:340px}
.logo{display:flex;align-items:center;gap:10px;margin-bottom:20px}
.logo .dot{width:38px;height:38px;border-radius:10px;background:var(--accent);color:var(--accent-ink);display:flex;align-items:center;justify-content:center;font-weight:800;font-size:18px}
.logo b{font-size:19px} .logo small{display:block;color:var(--muted);font-size:12px}
label{font-size:13px;color:var(--muted);font-weight:600;display:block;margin:10px 0 4px}
input{width:100%;box-sizing:border-box;padding:10px;margin:0 0 4px;border:1px solid var(--border);border-radius:8px;font-size:14px;background:var(--surface-2);color:var(--text)}
input:focus{outline:2px solid var(--accent);border-color:var(--accent)}
button{width:100%;padding:11px;min-height:44px;background:var(--accent);color:var(--accent-ink);border:0;border-radius:8px;font-weight:600;cursor:pointer;margin-top:14px;font-size:15px}
button:hover{background:#2f6fe0}
.e{color:var(--err);font-size:14px;background:var(--err-soft);padding:8px 12px;border-radius:8px;margin-bottom:10px}
.hint{color:var(--muted);font-size:12px;margin-top:14px;text-align:center}</style></head>
<body><div class="c">
<div class="logo"><div class="dot">V</div><div><b>Vlessbank</b><small>by JhopanStore</small></div></div>
@@M@@
<label>Username</label><input type="text" id="u" placeholder="username" autofocus autocomplete="username">
<label>Password</label><input type="password" id="p" placeholder="password" autocomplete="current-password">
<button onclick="doLogin()">Masuk</button>
<div class="hint">default: admin / admin, ganti di Pengaturan</div></div>
<script>
function doLogin(){
  var u = document.getElementById('u').value, p = document.getElementById('p').value;
  var h = 'Basic ' + btoa(u + ':' + p);
  fetch('/api/login', {method:'POST', headers:{'Authorization': h}}).then(function(r){
    if(r.ok) {
      sessionStorage.setItem('vbauth', h);   // kredensial di browser saja, server stateless
      renderApp();
    } else {
      var m = document.querySelector('.e') || document.createElement('p');
      m.className = 'e'; m.textContent = 'Username atau password salah';
      document.querySelector('.c').prepend(m);
    }
  }).catch(function(){
    var m = document.querySelector('.e') || document.createElement('p');
    m.className = 'e'; m.textContent = 'Koneksi gagal';
    document.querySelector('.c').prepend(m);
  });
}
document.getElementById('p').addEventListener('keydown', function(e){ if(e.key==='Enter') doLogin(); });
// dashboard dirender via fetch (semua request bawa Basic header) - bukan navigasi
function renderApp(){
  fetch('/admin', {headers: {'Authorization': sessionStorage.getItem('vbauth')}})
    .then(function(r){ return r.text(); })
    .then(function(html){
      document.open(); document.write(html); document.close();
    });
}
</script></body></html>`

// statusPage = webui.status_page().
func (s *Server) statusPage() string {
	run := s.xrayRunning()
	dot := "var(--ok)"
	teks := "VLESS BANK BERJALAN"
	if !run {
		dot = "var(--err)"
		teks = "VLESS BANK MATI"
	}
	return fill(statusTmpl,
		"@@DOT@@", dot,
		"@@TEKS@@", teks,
		"@@N@@", itoa(len(s.rows())))
}

const statusTmpl = `<!doctype html><html lang="id"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>Vlessbank</title>
<style>:root{--bg:#0f1115;--surface:#161a21;--border:#2a3038;--text:#e6e9ef;--muted:#9aa4b2;--accent:#3b82f6;--ok:#22c55e;--err:#f87171}
body{font-family:system-ui;background:var(--bg);color:var(--text);display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0}
.c{background:var(--surface);border:1px solid var(--border);padding:40px 48px;border-radius:16px;text-align:center}
.dot{width:16px;height:16px;border-radius:50%;background:@@DOT@@;display:inline-block;margin-right:10px;vertical-align:middle}
h1{font-size:20px;color:var(--text);margin:0;display:inline-block;vertical-align:middle}
p{color:var(--muted);font-size:13px;margin:14px 0 0}
a{color:var(--accent);font-weight:600;text-decoration:none}</style></head>
<body><div class="c">
<h1><span class="dot"></span>@@TEKS@@</h1>
<p>@@N@@ akun aktif · <a href="/admin">Kelola →</a></p>
</div></body></html>`
