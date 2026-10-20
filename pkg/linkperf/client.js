/**
 * Browser helper for CoGS linkperf endpoints (copy into apps or import from built bundle).
 * @param {string} apiBase e.g. https://todo-api.example.com
 * @param {{ downloadBytes?: number, uploadBytes?: number }} [opts]
 */
export async function measureLink(apiBase, opts = {}) {
  const base = apiBase.replace(/\/$/, "");
  const downloadBytes = opts.downloadBytes ?? 128 * 1024;
  const uploadBytes = opts.uploadBytes ?? 64 * 1024;

  const pings = [];
  for (let i = 0; i < 3; i++) {
    const t0 = performance.now();
    const res = await fetch(base + "/link/ping", { cache: "no-store", mode: "cors" });
    if (!res.ok) throw new Error("ping failed");
    await res.json();
    pings.push(performance.now() - t0);
  }
  pings.sort((a, b) => a - b);
  const pingMs = Math.round(pings[1]);

  const t1 = performance.now();
  const dl = await fetch(base + "/link/bench?bytes=" + downloadBytes, {
    cache: "no-store",
    mode: "cors",
  });
  if (!dl.ok) throw new Error("bench download failed");
  await dl.arrayBuffer();
  const dlSec = (performance.now() - t1) / 1000;
  const downloadMbps = dlSec > 0 ? (downloadBytes * 8) / dlSec / 1e6 : 0;

  const payload = new Uint8Array(uploadBytes);
  const t2 = performance.now();
  const ul = await fetch(base + "/link/bench?bytes=" + uploadBytes, {
    method: "POST",
    body: payload,
    cache: "no-store",
    mode: "cors",
  });
  if (!ul.ok) throw new Error("bench upload failed");
  await ul.json();
  const ulSec = (performance.now() - t2) / 1000;
  const uploadMbps = ulSec > 0 ? (uploadBytes * 8) / ulSec / 1e6 : 0;

  return {
    pingMs,
    downloadMbps: Math.round(downloadMbps * 10) / 10,
    uploadMbps: Math.round(uploadMbps * 10) / 10,
  };
}
