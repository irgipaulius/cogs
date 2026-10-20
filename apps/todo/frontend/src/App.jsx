import clsx from "clsx";
import {
  Check,
  Circle,
  CloudUpload,
  Loader2,
  Radio,
  RefreshCw,
  Satellite,
  Trash2,
  Zap,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { getClientId, getConfig } from "./lib/config.js";
import { measureLink } from "./lib/linkperf.js";
import { loadJSON, saveJSON } from "./lib/storage.js";

export default function App() {
  const cfg = useMemo(() => getConfig(), []);
  const clientId = useMemo(() => getClientId(), []);

  const [todos, setTodos] = useState(() => loadJSON(cfg.localKey, []));
  const [title, setTitle] = useState("");
  const [status, setStatus] = useState({ msg: "Connecting…", ok: true });
  const [busy, setBusy] = useState(false);
  const [showStress, setShowStress] = useState(false);
  const [linkStats, setLinkStats] = useState(null);

  const queue = () => loadJSON(cfg.offlineQueueKey, []);

  const api = useCallback(
    async (path, opts = {}) => {
      const headers = { "X-Client-Id": clientId, ...(opts.headers || {}) };
      const res = await fetch(cfg.apiBase + path, { ...opts, headers });
      if (!res.ok) throw new Error((await res.text()) || res.statusText);
      if (res.status === 204) return null;
      return res.json();
    },
    [cfg.apiBase, clientId],
  );

  const persistLocal = useCallback(
    (next) => {
      setTodos(next);
      saveJSON(cfg.localKey, next);
    },
    [cfg.localKey],
  );

  const hostLabel = cfg.apiBase.replace(/^https?:\/\//, "");

  const syncFromServer = useCallback(async () => {
    const data = await api("/api/todos");
    persistLocal(data.todos || []);
    setStatus({ msg: "Linked · " + hostLabel, ok: true });
  }, [api, hostLabel, persistLocal]);

  const runLinkTest = useCallback(async () => {
    try {
      setLinkStats(await measureLink(cfg.apiBase));
    } catch {
      setLinkStats(null);
    }
  }, [cfg.apiBase]);

  const pushTodo = useCallback(
    async (t) => {
      try {
        await api("/api/todos", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(t),
        });
      } catch (e) {
        const q = queue();
        q.push({ op: "upsert", todo: t, at: Date.now() });
        saveJSON(cfg.offlineQueueKey, q);
        setStatus({ msg: "Offline — queued on device", ok: false });
        throw e;
      }
    },
    [api, cfg.offlineQueueKey],
  );

  const flushOffline = async () => {
    setBusy(true);
    try {
      const q = queue();
      const next = [];
      for (const item of q) {
        try {
          if (item.op === "upsert") {
            await api("/api/todos", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify(item.todo),
            });
          }
        } catch {
          next.push(item);
        }
      }
      saveJSON(cfg.offlineQueueKey, next);
      await syncFromServer();
      setStatus({
        msg: next.length ? `${next.length} items still queued` : "Queue synchronized",
        ok: next.length === 0,
      });
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => {
    syncFromServer().catch(() =>
      setStatus({ msg: "API unreachable — start Go server or tunnel", ok: false }),
    );
  }, [syncFromServer]);

  useEffect(() => {
    runLinkTest();
    const id = setInterval(runLinkTest, 90_000);
    return () => clearInterval(id);
  }, [runLinkTest]);

  const pendingQueue = queue().length;
  const doneCount = todos.filter((t) => t.done).length;

  const addTodo = async (e) => {
    e.preventDefault();
    const text = title.trim();
    if (!text) return;
    const t = {
      id: crypto.randomUUID(),
      client_id: clientId,
      title: text,
      done: false,
      updated_at: new Date().toISOString(),
    };
    persistLocal([t, ...todos]);
    setTitle("");
    try {
      await pushTodo(t);
      await syncFromServer();
    } catch {
      /* queued */
    }
  };

  const toggleTodo = async (id, done) => {
    const next = todos.map((t) =>
      t.id === id ? { ...t, done, updated_at: new Date().toISOString() } : t,
    );
    persistLocal(next);
    const item = next.find((x) => x.id === id);
    try {
      await pushTodo(item);
    } catch {
      /* queued */
    }
  };

  const deleteTodo = async (id) => {
    persistLocal(todos.filter((t) => t.id !== id));
    try {
      await api(`/api/todos/${id}`, { method: "DELETE" });
    } catch {
      /* ignore */
    }
  };

  const uploadImage = async (id, file) => {
    const fd = new FormData();
    fd.append("image", file);
    setBusy(true);
    try {
      const res = await fetch(`${cfg.apiBase}/api/todos/${id}/image`, {
        method: "POST",
        headers: { "X-Client-Id": clientId },
        body: fd,
      });
      if (!res.ok) throw new Error(await res.text());
      setStatus({ msg: "Attachment stored (R2 + SQLite)", ok: true });
      await syncFromServer();
    } catch (err) {
      setStatus({ msg: "Upload failed: " + err.message, ok: false });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mx-auto flex min-h-screen max-w-lg flex-col px-4 pb-10 pt-8 sm:max-w-xl sm:px-6">
      <header className="mb-8">
        <div className="mb-6 flex items-start justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="flex h-11 w-11 items-center justify-center rounded-2xl border border-white/10 bg-white/[0.06] shadow-glow">
              <Satellite className="h-5 w-5 text-white" strokeWidth={1.75} />
            </div>
            <div>
              <p className="star-label">CoGS</p>
              <h1 className="text-2xl font-semibold tracking-tight text-white">Todo</h1>
            </div>
          </div>
          <button
            type="button"
            className="star-btn !px-3 !py-2"
            onClick={() => {
              syncFromServer().catch(() => setStatus({ msg: "Sync failed", ok: false }));
              runLinkTest();
            }}
            aria-label="Refresh"
          >
            <RefreshCw className={clsx("h-4 w-4", busy && "animate-spin")} />
          </button>
        </div>

        <div className="star-card flex items-center justify-between gap-3 px-4 py-3">
          <div className="flex items-center gap-2.5">
            <span
              className={clsx(
                "relative flex h-2.5 w-2.5 rounded-full",
                status.ok ? "bg-emerald-400" : "bg-amber-400",
              )}
            >
              <span
                className={clsx(
                  "absolute inset-0 animate-ping rounded-full opacity-40",
                  status.ok ? "bg-emerald-400" : "bg-amber-400",
                )}
              />
            </span>
            <div>
              <span className="text-sm text-white/85">{status.msg}</span>
              {linkStats && (
                <p className="mt-0.5 text-xs text-star-mist/90">
                  {linkStats.pingMs} ms · ↓ {linkStats.downloadMbps} · ↑ {linkStats.uploadMbps} Mbps
                </p>
              )}
            </div>
          </div>
          <Radio className="h-4 w-4 shrink-0 text-star-mist/60" strokeWidth={1.5} />
        </div>

        <div className="mt-4 grid grid-cols-2 gap-3">
          <Stat label="Open" value={todos.length - doneCount} />
          <Stat label="Complete" value={doneCount} />
        </div>
      </header>

      <form onSubmit={addTodo} className="star-card mb-6 p-4">
        <p className="star-label mb-3">New task</p>
        <div className="flex flex-col gap-3 sm:flex-row">
          <input
            className="star-input flex-1"
            placeholder="What needs doing?"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <button type="submit" className="star-btn star-btn-primary shrink-0 px-6">
            Add
          </button>
        </div>
      </form>

      <section className="flex-1">
        <div className="mb-3 flex items-center justify-between">
          <p className="star-label">Your list</p>
          {pendingQueue > 0 && (
            <span className="text-xs text-amber-400/90">{pendingQueue} queued offline</span>
          )}
        </div>

        {todos.length === 0 ? (
          <div className="star-card px-6 py-12 text-center">
            <Circle className="mx-auto mb-3 h-8 w-8 text-white/20" strokeWidth={1.25} />
            <p className="text-sm text-white/50">No tasks yet — add one above.</p>
          </div>
        ) : (
          <ul className="space-y-3">
            {todos.map((t) => (
              <TodoRow
                key={t.id}
                todo={t}
                apiBase={cfg.apiBase}
                onToggle={(done) => toggleTodo(t.id, done)}
                onDelete={() => deleteTodo(t.id)}
                onUpload={(file) => uploadImage(t.id, file)}
              />
            ))}
          </ul>
        )}
      </section>

      <footer className="mt-10 border-t border-white/[0.06] pt-6">
        <button
          type="button"
          className="star-label flex w-full items-center justify-between text-left"
          onClick={() => setShowStress((s) => !s)}
        >
          <span>Developer stress tools</span>
          <Zap className="h-3.5 w-3.5 text-star-mist/50" />
        </button>
        {showStress && (
          <div className="mt-3 flex flex-wrap gap-2">
            <button type="button" className="star-btn text-xs" onClick={flushOffline} disabled={busy}>
              {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
              Flush offline queue
            </button>
            <button
              type="button"
              className="star-btn text-xs"
              onClick={async () => {
                await api("/api/stress/seed?n=500", { method: "POST" });
                await syncFromServer();
              }}
            >
              Seed 500 rows
            </button>
            <button
              type="button"
              className="star-btn text-xs"
              onClick={async () => {
                await Promise.all(Array.from({ length: 20 }, () => api("/api/todos")));
                setStatus({ msg: "20 parallel GETs completed", ok: true });
              }}
            >
              20 parallel syncs
            </button>
          </div>
        )}
        <p className="mt-4 text-center text-[11px] leading-relaxed text-white/30">
          SQLite on your Mac · Tunnel API · localStorage offline queue
        </p>
      </footer>
    </div>
  );
}

function Stat({ label, value }) {
  return (
    <div className="star-card px-4 py-3">
      <p className="star-label mb-1">{label}</p>
      <p className="text-2xl font-semibold tabular-nums text-white">{value}</p>
    </div>
  );
}

function TodoRow({ todo, apiBase, onToggle, onDelete, onUpload }) {
  return (
    <li className="star-card group overflow-hidden transition hover:border-white/[0.12]">
      <div className="flex items-start gap-3 p-4">
        <button
          type="button"
          onClick={() => onToggle(!todo.done)}
          className={clsx(
            "mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full border transition",
            todo.done
              ? "border-emerald-400/60 bg-emerald-500/20 text-emerald-300"
              : "border-white/20 bg-white/[0.03] text-transparent hover:border-white/35",
          )}
          aria-label={todo.done ? "Mark incomplete" : "Mark complete"}
        >
          <Check className="h-3.5 w-3.5" strokeWidth={2.5} />
        </button>
        <div className="min-w-0 flex-1">
          <p
            className={clsx(
              "text-[15px] font-medium leading-snug text-white/95",
              todo.done && "text-white/40 line-through decoration-white/20",
            )}
          >
            {todo.title}
          </p>
          {todo.image_r2_key && (
            <img
              src={`${apiBase}/api/todos/${todo.id}/image`}
              alt=""
              className="mt-3 max-h-40 w-full rounded-xl border border-white/10 object-cover"
            />
          )}
          <div className="mt-3 flex flex-wrap gap-2 opacity-80 transition group-hover:opacity-100">
            <label className="star-btn cursor-pointer !py-2 text-xs">
              <CloudUpload className="h-3.5 w-3.5" />
              Photo
              <input
                type="file"
                accept="image/*"
                className="hidden"
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (f) onUpload(f);
                  e.target.value = "";
                }}
              />
            </label>
            <button type="button" className="star-btn !py-2 text-xs text-red-300/90" onClick={onDelete}>
              <Trash2 className="h-3.5 w-3.5" />
              Remove
            </button>
          </div>
        </div>
      </div>
    </li>
  );
}
