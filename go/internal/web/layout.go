package web

// layout.go: page(tab, msg, body) dari webui.py, kerangka sidebar + header + JS.

import "strings"

// page = webui.page(tab, msg) dengan body default body_dash().
func (s *Server) page(tab, msg string) string {
	return s.pageWith(tab, msg, s.bodyDash())
}

// pageWith = webui.page(tab, msg, body).
func (s *Server) pageWith(tab, msg, body string) string {
	stat := `<span class="badge expired">MATI</span>`
	if s.xrayRunning() {
		stat = `<span class="badge paid">RUNNING</span>`
	}
	m := ""
	if msg != "" {
		m = `<div class="flash">` + esc(msg) + `</div>`
	}
	var menu strings.Builder
	for _, it := range menus {
		on := ""
		if it.tab == tab {
			on = "on"
		}
		menu.WriteString(`<a href="` + it.url + `" onclick="nav(event,'` + it.url + `')" class="` + on +
			`"><span class="ico">` + it.ico + `</span> ` + it.label + `</a>`)
	}
	return fill(layoutTmpl,
		"@@CSS@@", cssMain,
		"@@MENU@@", menu.String(),
		"@@TITLE@@", menuLabel(tab),
		"@@STAT@@", stat,
		"@@MSG@@", m,
		"@@BODY@@", body)
}

const layoutTmpl = `<!doctype html><html lang="id"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>Vlessbank</title><style>@@CSS@@</style></head>
<body><div class="layout">
<div class="sidebar">
<div class="brand"><div class="dot">V</div><div><b>Vlessbank</b><small>by JhopanStore</small></div></div>
<div class="menu">@@MENU@@</div>
<div class="foot">JhopanStore</div>
</div>
<div class="main">
<header><h1>@@TITLE@@</h1>
<div class="right"><span class="who">@@STAT@@</span><a class="out" href="/logout" onclick="logout(event)">Logout</a></div></header>
<nav class="mobile-menu" aria-label="Navigasi utama">@@MENU@@</nav>
<div class="content">@@MSG@@@@BODY@@</div>
</div></div>
<script>
function logout(e){
  e.preventDefault();
  sessionStorage.removeItem('vbauth');
  location.href = '/logout';   // server hapus cookie sesi juga
}
function nav(e, u){
  // navigasi menu via fetch: header Basic ikut, gak kena 401 loop
  e.preventDefault();
  fetch(u, {headers: authH()}).then(function(r){
    if(r.status === 401) { sessionStorage.removeItem('vbauth'); location.href = '/login'; return; }
    return r.text();
  }).then(function(html){
    if(!html) return;
    document.open(); document.write(html); document.close();
  });
}
function authH(){
  return {'Authorization': sessionStorage.getItem('vbauth') || '',
    'Content-Type':'application/x-www-form-urlencoded'};
}
function api(p, d){
  var o = {method:'POST', headers: authH()};
  if(d) o.body = d;
  return fetch(p, o).then(function(r){
    if(r.status === 401) { sessionStorage.removeItem('vbauth'); location.href = '/login'; throw 'unauth'; }
    return r.json().then(function(j){
      if(j && j.error) flash(j.error, false);
      return j;
    });
  });
}
function badge(){
  api('/api/status').then(function(j) {
    var el = document.getElementById('xstat');
    if(el) el.outerHTML = j.xray
      ? '<span class="badge paid" id="xstat">RUNNING</span>'
      : '<span class="badge expired" id="xstat">MATI</span>';
  }).catch(function(){});
}
function xray(act){
  api('/api/xray', 'act=' + act).then(function(j) {
    api('/api/status').then(function(s) {
      var el = document.getElementById('xstat');
      if(el) el.outerHTML = s.xray
        ? '<span class="badge paid" id="xstat">RUNNING</span>'
        : '<span class="badge expired" id="xstat">MATI</span>';
    });
    flash(j.result || j.error || 'ok');
  }).catch(function(){});
}
function regen(){
  api('/api/regen', '').then(function(j) { flash(j.result || 'config digenerate'); }).catch(function(){});
}
function flash(s, ok){
  var m = document.getElementById('msg');
  if(m){ m.textContent = s; m.className = (ok === false ? 'flash err' : 'flash'); m.style.display = 'block'; }
}
function add(){
  var ta = document.getElementById('newlinks');
  if(!ta){ flash('form tambah tidak ada di halaman ini', false); return; }
  if(!ta.value.trim()){ flash('tempel link vless:// dulu', false); ta.focus(); return; }
  var fd = new URLSearchParams(); fd.set('links', ta.value);
  flash('menambah akun...');
  fetch('/api/add', {method:'POST', headers: authH(), body: fd.toString()})
    .then(function(r){ return r.json().then(function(j){ return {ok:r.ok, status:r.status, j:j || {}}; }); })
    .then(function(res){
      var j = res.j;
      if(!res.ok || j.error){ flash('gagal: ' + (j.error || ('HTTP ' + res.status)), false); return; }
      var msg = (j.added || 0) + ' akun ditambah';
      if(j.duplicates) msg += ', ' + j.duplicates + ' duplikat dibuang';
      if(j.skipped) msg += ', ' + j.skipped + ' dilewati (bukan vless)';
      flash(msg, (j.added || 0) > 0);
      ta.value = '';
      if(j.added) setTimeout(function(){ location.reload(); }, 900);
    })
    .catch(function(e){ flash('gagal kirim: ' + e, false); });
}
function cp(ev){
  ev.preventDefault();
  var btn = ev.target, txt = btn.previousElementSibling.textContent.trim();
  var done = function(){ btn.textContent = '✓'; btn.className = 'cp ok';
    setTimeout(function(){ btn.textContent = 'copy'; btn.className = 'cp'; }, 1200); };
  if(navigator.clipboard && navigator.clipboard.writeText){
    navigator.clipboard.writeText(txt).then(done, done);
  } else {
    var ta = document.createElement('textarea'); ta.value = txt; document.body.appendChild(ta);
    ta.select(); try { document.execCommand('copy'); } catch(e) {} document.body.removeChild(ta); done();
  }
}
function t(p){
  var c = document.getElementById('ip' + p);
  c.textContent = 'IP keluar: mengambil...'; c.className = 'ip run';
  fetch('/api/ip?port=' + p, {headers: authH()}).then(function(r){ return r.json(); }).then(function(j) {
    c.textContent = 'IP keluar: ' + j.result; c.className = 'ip';
  }).catch(function(){ c.textContent = 'IP keluar: err'; c.className = 'ip'; });
}
function tall(){
  var btn = (typeof event !== 'undefined' && event.target) ? event.target : null;
  if(btn) btn.disabled = true;
  document.getElementById('tallstat').textContent = 'mengetes semua port...';
  fetch('/api/ipall', {headers: authH()}).then(function(r){ return r.json(); }).then(function(j) {
    for(var p in j) {
      var c = document.getElementById('ip' + p);
      if(c) { c.textContent = 'IP keluar: ' + j[p]; c.className = 'ip'; }
    }
    document.getElementById('tallstat').textContent = 'selesai';
    if(btn) btn.disabled = false;
  }).catch(function() {
    document.getElementById('tallstat').textContent = 'gagal';
    if(btn) btn.disabled = false;
  });
}
function del(line, nm){
  if(!confirm('hapus ' + nm + '?')) return;
  api('/api/delete', 'line=' + line).then(function(j) {
    if(j.error) return flash(j.error);
    fetch('/admin', {headers: authH()}).then(function(r){ return r.text(); }).then(function(h){
      document.open(); document.write(h); document.close();
    });
  }).catch(function(){});
}
function htmlEscape(s){var d=document.createElement('div');d.textContent=s;return d.innerHTML;}
function apiGet(p){
  return fetch(p,{headers:authH()}).then(function(r){
    if(r.status===401){sessionStorage.removeItem('vbauth');location.href='/login';throw 'unauth';}
    return r.json().then(function(j){if(j&&j.error)flash(j.error,false);return j;});
  });
}
function clientTokenList(){
  var out=document.getElementById('ct-list'); if(!out)return;
  apiGet('/api/client-tokens').then(function(j){
    var a=j.tokens||[]; if(!a.length){out.textContent='belum ada token client';return;}
    out.innerHTML=a.map(function(t){return '<div class="chip"><code>'+htmlEscape(t.label)+' · '+htmlEscape(t.id)+' · '+(t.revoked?'dicabut':'aktif')+'</code><button class="cp" onclick="clientTokenRevoke(\''+htmlEscape(t.id)+'\')">cabut</button></div>';}).join('');
  }).catch(function(){out.textContent='gagal memuat token';});
}
function clientTokenCreate(){
  var label=document.getElementById('ct-label'),hours=document.getElementById('ct-hours'),out=document.getElementById('ct-result');
  if(!label||!label.value.trim()){flash('isi label perangkat',false);label.focus();return;}
  api('/api/client-tokens','label='+encodeURIComponent(label.value.trim())+'&hours='+encodeURIComponent(hours.value)).then(function(j){
    if(!j.token)return; out.hidden=false;out.textContent='Salin sekarang, token tidak akan tampil lagi:\n'+j.token;label.value='';clientTokenList();
  }).catch(function(){});
}
function clientTokenRevoke(id){if(!confirm('cabut token '+id+'?'))return;api('/api/client-tokens/revoke','id='+encodeURIComponent(id)).then(function(){clientTokenList();}).catch(function(){});}
function opsShow(v){var out=document.getElementById('ops-result');if(out){out.hidden=false;out.textContent=JSON.stringify(v,null,2);}}
function healthLoad(){apiGet('/api/health').then(opsShow).catch(function(){});}
function auditLoad(){apiGet('/api/audit').then(opsShow).catch(function(){});}
function backupDownload(){
  fetch('/api/backup',{headers:authH()}).then(function(r){if(!r.ok)throw new Error('HTTP '+r.status);return r.blob();}).then(function(b){var a=document.createElement('a');a.href=URL.createObjectURL(b);a.download='pan-vlessbank-backup.zip';a.click();URL.revokeObjectURL(a.href);}).catch(function(e){flash('backup gagal: '+e,false);});
}
if(location.pathname==='/admin/settings'){clientTokenList();}
</script>
</body></html>`
