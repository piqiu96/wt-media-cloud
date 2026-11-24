import { loadBatchSnapshot, saveBatchSnapshot } from "./batchApi.js";
// Isolated browser storage for this offline prototype; never used by runtime APIs.
const DATABASE = "wt-media-publishing-prototype",
  STORE = "snapshots";
const KEY = new URLSearchParams(globalThis.location?.search).get("review") === "chg109" ? "publishing-chg109" : "publishing";
let connection;
function open() {
  if (!connection)
    connection = new Promise((resolve, reject) => {
      const request = indexedDB.open(DATABASE, 1);
      request.onupgradeneeded = () => request.result.createObjectStore(STORE);
      request.onsuccess = () => resolve(request.result);
      request.onerror = () =>
        reject(request.error || new Error("无法读取本地演示记录"));
      request.onblocked = () => reject(new Error("请关闭旧的原型窗口后重试"));
    }).catch((error) => {
      connection = null;
      throw error;
    });
  return connection;
}
export async function loadPrototype() {
  const controller = new AbortController();
  let timer;
  const timeout = new Promise((_, reject) => {
    timer = setTimeout(() => {
      const error = new Error("任务记录读取超时，请检查本地预览服务后重试");
      controller.abort(error);
      reject(error);
    }, 15000);
  });
  try {
    return await Promise.race([readPrototype(controller.signal), timeout]);
  } finally {
    clearTimeout(timer);
  }
}
async function readPrototype(signal) {
  const db = await open();
  const saved = await new Promise((resolve, reject) => {
    const request = db.transaction(STORE).objectStore(STORE).get(KEY);
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
  // A late storage response must not start a migration after timeout.
  signal.throwIfAborted();
  if (saved?.version === 1) return await loadBatchSnapshot(saved.data, signal);
  if (KEY === "publishing-chg109") return await loadBatchSnapshot(null, signal);
  const legacy = JSON.parse(
    localStorage.getItem("wt-publishing-design-v1") || "null",
  );
  return await loadBatchSnapshot(legacy?.version === 1 ? legacy.data : null, signal);
}
export async function savePrototype(data) {
  const snapshot = { ...data };
  delete snapshot.publishingConfig; // Discard the retired prototype configuration page data.
  await saveBatchSnapshot(snapshot);
  const db = await open();
  await new Promise((resolve, reject) => {
    const tx = db.transaction(STORE, "readwrite");
    tx.objectStore(STORE).put({ version: 1, data: snapshot }, KEY);
    tx.oncomplete = resolve;
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error || new Error("保存已中断"));
  });
}
