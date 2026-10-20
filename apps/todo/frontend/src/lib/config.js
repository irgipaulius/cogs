export function getConfig() {
  const c = window.COGS_CONFIG || {};
  return {
    apiBase: c.apiBase || "http://127.0.0.1:8080",
    localKey: c.localKey || "cogs_todo_v1",
    offlineQueueKey: c.offlineQueueKey || "cogs_todo_offline_v1",
    pageOrigin: c.pageOrigin || "",
  };
}

export function getClientId() {
  const key = "cogs_client_id";
  let id = localStorage.getItem(key);
  if (!id) {
    id = crypto.randomUUID();
    localStorage.setItem(key, id);
  }
  return id;
}
