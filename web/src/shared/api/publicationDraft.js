import { invoke } from "@tauri-apps/api/core";

const inputCache = new Map();
let sessionGeneration = 0;
const nativeDesktop = () =>
  typeof window !== "undefined" && !!window.__TAURI_INTERNALS__;
const clone = (value) =>
  value == null ? value : JSON.parse(JSON.stringify(value));
const identityChanged = () =>
  Object.assign(new Error("登录身份已变化，请重新打开发布配置"), {
    code: "draft_identity_required",
  });

export async function resetPublicationDraftSession({
  tauri = nativeDesktop(),
  invokeImpl = invoke,
} = {}) {
  sessionGeneration++;
  inputCache.clear();
  if (tauri) await invokeImpl("local_publication_draft_deactivate");
}

export function createPublicationDraftClient({
  userId,
  draftType = "task",
  tauri = nativeDesktop(),
  invokeImpl = invoke,
}) {
  const user = String(userId ?? "");
  if (!/^[1-9][0-9]*$/.test(user)) throw identityChanged();
  if (!["task", "batch", "plan"].includes(draftType)) throw new Error("不支持的续编类型");
  const typeArgs = draftType === "task" ? {} : { draft_type: draftType };
  const generation = sessionGeneration;
  const key = `${tauri ? "desktop" : "web"}:${user}:${draftType}`;
  if (!inputCache.has(key))
    inputCache.set(key, {
      loaded: false,
      value: {
        payload: null,
        revision: "0",
        saved_at: null,
        schema_version: 1,
      },
    });
  const entry = inputCache.get(key);
  function guard() {
    if (generation !== sessionGeneration) throw identityChanged();
  }
  function accept(value) {
    guard();
    if (
      !value ||
      !/^(0|[1-9][0-9]*)$/.test(value.revision) ||
      value.schema_version !== 1 ||
      (value.payload !== null &&
        (typeof value.payload !== "object" || Array.isArray(value.payload)))
    ) {
      throw new Error("续编数据无法读取，请保留当前填写并联系管理员");
    }
    entry.value = clone(value);
    entry.loaded = true;
    return clone(entry.value);
  }
  return {
    persistent: tauri,
    async get({ reload = false } = {}) {
      guard();
      if (entry.loaded && !reload) return clone(entry.value);
      return accept(
        tauri
          ? await invokeImpl("local_publication_draft_get", {
              args: { user_id: user, ...typeArgs },
            })
          : entry.value,
      );
    },
    remember(payload) {
      guard();
      entry.value.payload = clone(payload);
    },
    async save(payload) {
      guard();
      if (!entry.loaded) throw new Error("请先读取当前续编，再保存");
      // A failed write retains human input without guessing a new revision.
      entry.value.payload = clone(payload);
      return accept(
        tauri
          ? await invokeImpl("local_publication_draft_save", {
              args: {
                user_id: user,
                ...typeArgs,
                expected_revision: entry.value.revision,
                payload: clone(payload),
              },
            })
          : {
              ...entry.value,
              revision: String(BigInt(entry.value.revision) + 1n),
              saved_at: new Date().toISOString(),
            },
      );
    },
    async clear() {
      guard();
      if (!entry.loaded) throw new Error("请先读取当前续编，再清除");
      return accept(
        tauri
          ? await invokeImpl("local_publication_draft_clear", {
              args: { user_id: user, ...typeArgs, expected_revision: entry.value.revision },
            })
          : {
              ...entry.value,
              payload: null,
              revision: String(BigInt(entry.value.revision) + 1n),
              saved_at: new Date().toISOString(),
            },
      );
    },
  };
}
