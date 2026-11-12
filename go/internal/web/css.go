package web

// css.go: salinan persis CSS = """...""" di webui.py (tanpa newline di akhir).
// Tema gelap sesuai DESIGN.md: token di :root, semua warna lewat var() supaya
// satu tempat saja yang diubah. Kontras tiap pasangan teks sudah dihitung (AA >= 4.5).

const cssMain = `:root{--bg:#0f1115;--surface:#161a21;--surface-2:#1b2027;--surface-3:#191e26;--pre:#12161d;
--border:#2a3038;--border-soft:#232a33;--text:#e6e9ef;--muted:#9aa4b2;--muted-2:#94a3b8;
--accent:#3b82f6;--accent-ink:#0b1220;--accent-soft:#152a45;--accent-text:#93c5fd;
--ok:#22c55e;--ok-text:#4ade80;--ok-soft:#14301f;--ok-text-2:#86efac;
--warn:#f59e0b;--warn-text:#fbbf24;--warn-soft:#3a2f10;
--err:#f87171;--err-text:#fca5a5;--err-soft:#3a1a1c;
--sidebar:#141821;--sidebar-hover:#1d2330;--sidebar-text:#cbd5e1;--sidebar-muted:#8b96a5;
--shadow:0 1px 3px rgba(0,0,0,.4)}
*{box-sizing:border-box}
html{height:100%}
body{font-family:system-ui;background:var(--bg);color:var(--text);margin:0;min-height:100vh}
.layout{display:flex;width:100%;min-height:100vh}
.sidebar{width:230px;flex:0 0 230px;background:var(--sidebar);color:var(--sidebar-text);padding:18px 14px;display:flex;flex-direction:column;position:sticky;top:0;height:100vh;align-self:flex-start}
.brand{display:flex;align-items:center;gap:10px;padding:6px 8px 20px;border-bottom:1px solid var(--border);margin-bottom:14px}
.brand .dot{width:36px;height:36px;border-radius:10px;background:var(--accent);color:var(--accent-ink);display:flex;align-items:center;justify-content:center;font-weight:800;font-size:17px;flex-shrink:0}
.brand b{color:var(--text);font-size:16px;display:block;line-height:1.1}
.brand small{color:var(--sidebar-muted);font-size:11px}
.menu a,.mobile-menu a{display:flex;align-items:center;gap:10px;padding:11px 12px;border-radius:10px;color:var(--sidebar-text);text-decoration:none;font-size:14px;font-weight:500;margin-bottom:4px}
.menu a:hover,.mobile-menu a:hover{background:var(--sidebar-hover);color:var(--text)}
.menu a.on,.mobile-menu a.on{background:var(--accent);color:var(--accent-ink);font-weight:600}
.menu .ico,.mobile-menu .ico{width:20px;text-align:center;flex-shrink:0}
.mobile-menu{display:none}
.sidebar .foot{margin-top:auto;padding:12px 8px 0;border-top:1px solid var(--border);font-size:12px;color:var(--sidebar-muted)}
.main{flex:1 1 auto;min-width:0;min-height:100vh;display:flex;flex-direction:column}
header{background:var(--surface);border-bottom:1px solid var(--border);padding:14px 28px;display:flex;justify-content:space-between;align-items:center;position:sticky;top:0;z-index:5}
header h1{font-size:18px;margin:0;color:var(--text)}
header .right{display:flex;align-items:center;gap:14px}
header .who{font-size:13px;color:var(--muted)}
header a.out{color:var(--err);text-decoration:none;font-size:13px;font-weight:600;padding:7px 14px;border:1px solid var(--err);border-radius:8px}
header a.out:hover{background:var(--err-soft)}
.content{flex:1 1 auto;width:100%;max-width:none;margin:0;padding:22px 26px}
.card{background:var(--surface);border:1px solid var(--border);border-radius:12px;padding:20px;margin-bottom:20px;max-width:100%;overflow-x:auto}
h2{margin:0 0 12px;font-size:16px;color:var(--text)}
table{width:100%;border-collapse:collapse;font-size:14px}
th,td{padding:8px 10px;border-bottom:1px solid var(--border-soft);text-align:left;overflow-wrap:anywhere}
th{color:var(--muted);font-weight:600;background:var(--surface-2)}
code{background:var(--surface-2);padding:2px 6px;border-radius:6px;font-size:13px;color:var(--text);font-family:ui-monospace,SFMono-Regular,Consolas,monospace;overflow-wrap:anywhere}
input,select,textarea{padding:8px;border:1px solid var(--border);border-radius:8px;margin:4px 0;font-size:14px;background:var(--surface-2);color:var(--text)}
input:focus,textarea:focus,select:focus{outline:2px solid var(--accent);border-color:var(--accent)}
textarea{width:100%;height:90px}
label{display:block;font-size:13px;color:var(--muted);font-weight:600;margin:10px 0 4px}
button{padding:8px 14px;min-height:44px;background:var(--accent);color:var(--accent-ink);border:0;border-radius:8px;cursor:pointer;font-weight:600;font-size:13px}
button:hover{background:#2f6fe0}
button.del{background:transparent;color:var(--err);border:1px solid var(--err)}
button.del:hover{background:var(--err-soft)}
button.sec{background:var(--surface-2);color:var(--text);border:1px solid var(--border)}
button.sec:hover{background:var(--sidebar-hover)}
.btnrow{display:flex;gap:10px;flex-wrap:wrap}
.badge{padding:3px 10px;border-radius:999px;font-size:12px;font-weight:600}
.paid{background:var(--ok-soft);color:var(--ok-text)}
.pending{background:var(--warn-soft);color:var(--warn-text)}
.expired{background:var(--err-soft);color:var(--err-text)}
td small{color:var(--muted-2);font-size:12px}
td.empty{text-align:center;color:var(--muted-2);padding:18px}
.stats{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:12px;margin-bottom:20px}
.stat{background:var(--surface);border:1px solid var(--border);border-radius:12px;padding:16px;text-align:center}
.stat .n{font-size:26px;font-weight:800;color:var(--text);font-variant-numeric:tabular-nums;overflow-wrap:anywhere}
.stat .n code{font-size:12px;white-space:normal;overflow-wrap:anywhere;max-width:100%}
.stat .l{font-size:12px;color:var(--muted);margin-top:2px}
.flash{background:var(--ok-soft);color:var(--ok-text);padding:10px 14px;border-radius:8px;margin-bottom:14px}
.flash.err{background:var(--err-soft);color:var(--err-text)}
small{color:var(--muted)}
pre{background:var(--pre);border:1px solid var(--border);border-radius:8px;padding:12px;font-size:12px;overflow-x:auto;max-height:420px;overflow-y:auto;white-space:pre-wrap;word-break:break-all;color:var(--text)}
.ip{font-variant-numeric:tabular-nums}
.ip.run{color:var(--accent-text);font-weight:600}
.hint{color:var(--muted);font-size:12px;margin-top:8px}
a{color:var(--accent)}
.akun-grid{display:flex;flex-wrap:wrap;gap:14px}
.akun-card{background:var(--surface);border:1px solid var(--border);border-radius:12px;padding:14px;min-width:0;flex:1 1 400px}
.akun-head{display:flex;align-items:center;gap:10px;margin-bottom:12px;flex-wrap:wrap}
.akun-idx{width:26px;height:26px;border-radius:8px;background:var(--accent);color:var(--accent-ink);display:flex;align-items:center;justify-content:center;font-size:12px;font-weight:700;flex-shrink:0}
.akun-name{font-weight:600;color:var(--text);font-size:14px}
.akun-src{color:var(--muted);font-size:11.5px;margin-left:auto;text-align:right;word-break:break-all;max-width:100%}
.chips{display:flex;flex-direction:column;gap:6px}
.chip{display:grid;grid-template-columns:auto minmax(0,1fr) auto;align-items:center;gap:8px;background:var(--surface-2);border:1px solid var(--border-soft);border-radius:8px;padding:8px;min-width:0}
.chip.sub{background:var(--surface-3)}
.chip-labels{display:flex;gap:4px;align-items:center;min-width:0}
.chip code{min-width:0;white-space:normal;overflow-wrap:anywhere;font-size:12px;color:var(--text);background:transparent;padding:0}
.chip.sub{align-items:start}
.chip.sub code{font-size:11.5px}
.tag{font-size:10px;font-weight:700;letter-spacing:.4px;color:var(--muted);background:var(--border);border-radius:4px;padding:2px 6px;flex-shrink:0}
.tag.lan{color:var(--accent-text);background:var(--accent-soft)}
.tag.pub{color:var(--ok-text-2);background:var(--ok-soft)}
.akun-foot{display:flex;align-items:center;gap:8px;margin-top:12px;border-top:1px solid var(--border-soft);padding-top:10px;flex-wrap:wrap}
.akun-foot .spacer{flex:1}
.empty{padding:18px;text-align:center;color:var(--muted);font-size:13px}
button.cp{background:var(--surface-2);color:var(--text);border:1px solid var(--border);border-radius:6px;font-size:11px;padding:2px 7px;min-height:0;height:26px;cursor:pointer;width:auto;flex-shrink:0}
button.cp:hover{background:var(--sidebar-hover)}
button.cp.ok{background:var(--ok-soft);border-color:var(--ok);color:var(--ok-text)}
form.inline{display:inline}
@media(max-width:800px){.sidebar{display:none}
.main{min-height:100vh}
.mobile-menu{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:4px;padding:8px 12px;background:var(--surface);border-bottom:1px solid var(--border)}
.mobile-menu a{justify-content:center;gap:6px;min-height:44px;padding:8px 6px;margin:0;font-size:12px;white-space:nowrap}
.mobile-menu .ico{width:auto}
.content{padding:12px}
.card{padding:14px}
h2{font-size:15px}
.stats{grid-template-columns:repeat(2,minmax(0,1fr));gap:10px}
.stats .stat:last-child:nth-child(odd){grid-column:1/-1}
.stat{padding:12px}
.stat .n{font-size:20px}
.akun-grid{gap:10px}
.akun-card{padding:12px}
header{padding:12px 14px}
header h1{font-size:16px}
.btnrow{gap:8px}
.btnrow button{flex:1 1 auto}
textarea{min-height:110px}
.chip{grid-template-columns:minmax(0,1fr) auto;gap:6px 8px}
.chip-labels{grid-column:1/-1}
.chip code{font-size:11.5px}
button.cp{height:28px}}`
