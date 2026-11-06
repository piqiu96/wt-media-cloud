import { ref } from 'vue'

// Keep confirmations inside the lock so rapid clicks cannot open duplicate dialogs.
export function usePendingActions() {
  const pending = ref(new Set())
  async function run(id, action) {
    if (pending.value.has(id)) return
    pending.value = new Set([...pending.value, id])
    try { return await action() }
    finally { const next = new Set(pending.value); next.delete(id); pending.value = next }
  }
  return { pending, run }
}

export function useEditorClose({ visible, busy, draft, confirm }) {
  let baseline = '', confirming = false
  function capture() { baseline = JSON.stringify(draft()) }
  async function close() {
    if (busy.value || confirming) return
    if (JSON.stringify(draft()) === baseline) { visible.value = false; return }
    confirming = true
    try {
      if (await confirm({ title: '放弃未保存的修改？', message: '关闭后，修改不会保留。', confirmText: '不保存', danger: true })) visible.value = false
    } finally { confirming = false }
  }
  return { capture, close }
}

export function useReferenceDetails(load) {
  const visible = ref(false), category = ref(null), data = ref(null), loading = ref(false), error = ref('')
  let generation = 0
  async function reload() {
    const id = ++generation, target = category.value
    if (!target) return
    data.value = null; error.value = ''; loading.value = true
    try {
      const result = await load(target.id)
      if (generation === id) data.value = result
    } catch (e) {
      if (generation === id) error.value = e.message || '加载关联信息失败'
    } finally { if (generation === id) loading.value = false }
  }
  function open(target) { category.value = target; visible.value = true; return reload() }
  return { visible, category, data, loading, error, open, reload }
}
