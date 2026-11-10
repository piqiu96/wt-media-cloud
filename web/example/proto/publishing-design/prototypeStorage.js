// Isolated browser storage for this offline prototype; never used by runtime APIs.
const DATABASE = "wt-media-publishing-prototype",
  STORE = "snapshots";
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
  const db = await open();
  const saved = await new Promise((resolve, reject) => {
    const request = db.transaction(STORE).objectStore(STORE).get("publishing");
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
  if (saved?.version === 1) return saved.data;
  const legacy = JSON.parse(
    localStorage.getItem("wt-publishing-design-v1") || "null",
  );
  return legacy?.version === 1 ? legacy.data : null;
}
export async function savePrototype(data) {
  const db = await open();
  await new Promise((resolve, reject) => {
    const tx = db.transaction(STORE, "readwrite");
    tx.objectStore(STORE).put({ version: 1, data }, "publishing");
    tx.oncomplete = resolve;
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error || new Error("保存已中断"));
  });
}
