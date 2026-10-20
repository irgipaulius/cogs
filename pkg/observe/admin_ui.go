package observe

const adminHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>CoGS Admin</title>
  <style>
    * { box-sizing: border-box; }
    body { margin: 0; font: 14px/1.45 ui-monospace, SFMono-Regular, Menlo, monospace;
      background: #0a0c10; color: #e6edf3; }
    header { padding: 1rem 1.25rem; border-bottom: 1px solid #21262d; }
    h1 { margin: 0; font-size: 1rem; font-weight: 600; letter-spacing: 0.08em; text-transform: uppercase; }
    p.sub { margin: 0.35rem 0 0; color: #8b949e; font-size: 12px; }
    main { display: grid; grid-template-columns: 280px 1fr; gap: 0; min-height: calc(100vh - 64px); }
    aside { border-right: 1px solid #21262d; padding: 1rem; }
    .stat { margin-bottom: 1rem; }
    .stat label { display: block; color: #8b949e; font-size: 11px; text-transform: uppercase; letter-spacing: 0.06em; }
    .stat strong { font-size: 1.25rem; }
    section.logs { padding: 1rem; overflow: auto; max-height: calc(100vh - 64px); }
    .line { padding: 0.2rem 0; border-bottom: 1px solid #161b22; white-space: pre-wrap; word-break: break-all; }
    .line .t { color: #484f58; margin-right: 0.5rem; }
    @media (max-width: 720px) { main { grid-template-columns: 1fr; } aside { border-right: none; border-bottom: 1px solid #21262d; } }
  </style>
</head>
<body>
  <header>
    <h1>CoGS · Local admin</h1>
    <p class="sub">Loopback only — not exposed via tunnel Hostname. Real-time logs + host + SQLite.</p>
  </header>
  <main>
    <aside id="stats"></aside>
    <section class="logs" id="logs"></section>
  </main>
  <script>
    const statsEl = document.getElementById('stats');
    const logsEl = document.getElementById('logs');
    function renderStats(s) {
      const h = s.host || {}, db = s.db || {}, http = s.http || {};
      statsEl.innerHTML = [
        ['CPU', (h.cpu_percent||0).toFixed(1) + '%'],
        ['Memory', (h.mem_used_mb||0).toFixed(0) + ' / ' + (h.mem_total_mb||0).toFixed(0) + ' MB'],
        ['Process RSS', (h.process_rss_mb||0).toFixed(1) + ' MB'],
        ['HTTP total', http.total],
        ['In flight', http.in_flight],
        ['Last req ms', http.last_ms],
        ['DB size', ((db.size_bytes||0)/1024).toFixed(1) + ' KB'],
        ['SQL queries', db.sql_queries],
        ['SQL execs', db.sql_execs],
      ].map(([l,v]) => '<div class="stat"><label>'+l+'</label><strong>'+v+'</strong></div>').join('');
    }
    async function poll() {
      try {
        const r = await fetch('/admin/api/snapshot');
        renderStats(await r.json());
      } catch (e) {}
    }
    setInterval(poll, 2000);
    poll();
    const es = new EventSource('/admin/api/stream');
    es.onmessage = (ev) => {
      try {
        const line = JSON.parse(ev.data);
        const div = document.createElement('div');
        div.className = 'line';
        const t = new Date(line.time).toLocaleTimeString();
        div.innerHTML = '<span class="t">' + t + '</span>' + (line.message || '');
        logsEl.prepend(div);
        while (logsEl.children.length > 400) logsEl.lastChild.remove();
      } catch (e) {}
    };
  </script>
</body>
</html>`
