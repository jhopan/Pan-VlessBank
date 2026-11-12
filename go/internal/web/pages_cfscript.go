package web

// pages_cf_js.go: <script> body_cf() (webui.py baris 804-968).

const cfScript = `function cfMsg(s,ok){
  var m=document.getElementById('cfmsg');
  m.textContent=s; m.style.display='block';
  m.style.background=ok===false?'var(--err-soft)':ok===true?'var(--ok-soft)':'var(--warn-soft)';
  m.style.color=ok===false?'var(--err-text)':ok===true?'var(--ok-text)':'var(--warn-text)';
}
function cfApi(path,data){
  return fetch(path,{method:'POST',headers:authH(),body:data?new URLSearchParams(data).toString():''})
    .then(function(r){if(r.status===401){sessionStorage.removeItem('vbauth');location.href='/login';throw 'unauth';}return r.json();});
}
function cfGet(path){
  return fetch(path,{headers:{'Authorization':sessionStorage.getItem('vbauth')||''}}).then(function(r){return r.json();});
}

function cfDownload(){
  cfApi('/api/cf/download').then(function(){
    cfMsg('Mengunduh cloudflared...',null);
    var iv=setInterval(function(){
      cfGet('/api/cf/dlstatus').then(function(s){
        if(s.state==='done'){clearInterval(iv);cfMsg('Download selesai: '+s.version,true);location.reload();}
        else if(s.state==='error'){clearInterval(iv);cfMsg('Error: '+s.error,false);}
        else{var el=document.getElementById('dlpct');if(el)el.textContent=s.progress;}
      });
    },1500);
  });
}

var _loginIv=null;
function cfLogin(){
  cfMsg('Memulai proses login...',null);
  cfApi('/api/cf/login').then(function(j){
    if(!j.ok){cfMsg('Error: '+j.error,false);return;}
    document.getElementById('cfloginbox').style.display='block';
    if(j.url) _setLoginUrl(j.url);
    if(j.done){cfMsg('Login berhasil!',true);return;}
    if(_loginIv) clearInterval(_loginIv);
    _loginIv=setInterval(function(){
      cfGet('/api/cf/loginstatus').then(function(s){
        if(s.url) _setLoginUrl(s.url);
        if(s.done){clearInterval(_loginIv);cfMsg('✓ Login berhasil! Lanjut ke step 3.',true);
          setTimeout(function(){location.reload();},2000);}
        else if(!s.running){clearInterval(_loginIv);}
      });
    },2000);
  });
}
function _setLoginUrl(url){
  var el=document.getElementById('cfloginurl');
  if(el) el.innerHTML='<a href="'+url+'" target="_blank" style="color:var(--accent);font-weight:600">'+url+'</a>';
}
function cfCancelLogin(){
  if(_loginIv) clearInterval(_loginIv);
  cfApi('/api/cf/cancelLogin').then(function(){
    cfMsg('Login dibatalkan',null); location.reload();
  });
}

function cfCreate(){
  var nm=document.getElementById('tname').value.trim();
  if(!nm){cfMsg('Isi nama tunnel dulu',false);return;}
  cfMsg('Membuat tunnel "'+nm+'"...',null);
  cfApi('/api/cf/create',{name:nm}).then(function(j){
    if(j.ok){cfMsg('✓ Tunnel dibuat! ID: '+j.id,true);setTimeout(function(){location.reload();},1500);}
    else cfMsg('Error: '+j.error,false);
  });
}
function cfRefreshTunnels(){
  cfApi('/api/cf/refreshtunnels').then(function(j){
    cfMsg('List direfresh ('+j.count+' tunnel)',true);setTimeout(function(){location.reload();},800);
  });
}
function cfUseTunnel(id,name){
  cfApi('/api/cf/usetunnel',{id:id,name:name}).then(function(j){
    if(j.ok){cfMsg('✓ Tunnel "'+name+'" dipilih. Lanjut ke step 4.',true);setTimeout(function(){location.reload();},1000);}
    else cfMsg('Error: '+(j.error||'?'),false);
  });
}
function cfDelTunnel(id,name){
  if(!confirm('Hapus tunnel '+name+'?'))return;
  cfApi('/api/cf/deltunnel',{id:id}).then(function(j){
    if(j.ok){cfMsg('Tunnel dihapus',true);setTimeout(function(){location.reload();},800);}
    else cfMsg('Error: '+j.error,false);
  });
}

var _zones=[];
function cfLoadZones(){
  var tok=document.getElementById('cftoken').value.trim();
  if(!tok){cfMsg('Isi API token dulu',false);return;}
  cfMsg('Mengambil daftar domain...',null);
  cfApi('/api/cf/zones',{token:tok}).then(function(j){
    if(!j.ok){cfMsg('Error: '+j.error,false);return;}
    _zones=j.zones;
    var sel=document.getElementById('zoneSel');
    sel.innerHTML='<option value="">-- pilih domain --</option>';
    j.zones.forEach(function(z){
      var o=document.createElement('option');
      o.value=z.id; o.textContent=z.name+' ('+z.status+')'; o.dataset.name=z.name;
      sel.appendChild(o);
    });
    document.getElementById('zonebox').style.display='block';
    cfMsg('✓ '+j.zones.length+' domain ditemukan',true);
    // simpan token ke session (tidak ke server)
    sessionStorage.setItem('cftoken',tok);
  });
}
function _selectedZone(){
  var sel=document.getElementById('zoneSel');
  var opt=sel.options[sel.selectedIndex];
  return {id:sel.value, name:opt?opt.dataset.name:''};
}
function cfGenYml(){
  var z=_selectedZone();
  if(!z.id){cfMsg('Pilih domain dulu',false);return;}
  var sub=document.getElementById('subdom').value.trim();
  var tok=document.getElementById('cftoken').value.trim();
  cfMsg('Generate config.tunnel.yml...',null);
  cfApi('/api/cf/genYml',{zone_name:z.name,subdomain:sub,token:tok}).then(function(j){
    if(j.ok){cfMsg('✓ config.tunnel.yml dibuat ('+j.accounts+' akun)',true);setTimeout(function(){location.reload();},1000);}
    else cfMsg('Error: '+j.error,false);
  });
}
function cfGenYmlManual(){
  var dom=document.getElementById('manDomain').value.trim();
  if(!dom){cfMsg('Isi domain dulu',false);return;}
  cfMsg('Generate config.tunnel.yml...',null);
  cfApi('/api/cf/genYml',{zone_name:dom,subdomain:document.getElementById('subdom').value.trim()}).then(function(j){
    if(j.ok){cfMsg('✓ config.tunnel.yml dibuat ('+j.accounts+' akun)',true);setTimeout(function(){location.reload();},1000);}
    else cfMsg('Error: '+j.error,false);
  });
}
function cfCreateDNS(){
  var z=_selectedZone();
  if(!z.id){cfMsg('Pilih domain dulu',false);return;}
  var sub=document.getElementById('subdom').value.trim();
  var tok=document.getElementById('cftoken').value.trim();
  if(!tok){cfMsg('Isi API token dulu',false);return;}
  cfMsg('Membuat DNS wildcard...',null);
  cfApi('/api/cf/createDNS',{token:tok,zone_id:z.id,zone_name:z.name,subdomain:sub}).then(function(j){
    if(j.ok){cfMsg('✓ CNAME *.'+(sub?sub+'.':'')+z.name+' -> '+j.target+(j.note?' ('+j.note+')':''),true);}
    else cfMsg('Error DNS: '+j.error,false);
  });
}
function cfTunnel(act){
  cfApi('/api/cf/tunnel',{act:act}).then(function(j){
    cfMsg(j.result||j.error||'ok', act==='stop'?null:true);
    setTimeout(function(){
      cfGet('/api/cf/logstatus').then(function(s){
        var el=document.getElementById('cflog');if(el)el.textContent=s.log||'(kosong)';
      });
    },2000);
  });
}
// restore token dari session
(function(){var t=sessionStorage.getItem('cftoken');if(t)document.getElementById('cftoken').value=t;})();
// poll log + badge tiap 5 detik
setInterval(function(){
  cfGet('/api/cf/logstatus').then(function(s){
    var el=document.getElementById('cflog');if(el)el.textContent=s.log||'(kosong)';
    var tb=document.querySelector('.stat:nth-child(3) .n');
    if(tb)tb.innerHTML=s.running?'<span class="badge paid">RUNNING</span>':'<span class="badge expired">MATI</span>';
  }).catch(function(){});
},5000);`
