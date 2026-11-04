import { computed, ref } from 'vue'

// Keep a bounded, explicitly requested directory window; selection resolves independently by ID.
export function useStrategyDirectory(client) {
  const rows = ref([]), total = ref(0), page = ref(0), loading = ref(false), error = ref(''), selected = ref(null)
  let sequence = 0, selectionSequence = 0, context = {}, search = ''
  const hasMore = computed(() => rows.value.length < total.value)
  async function load(filters = {}, term = '', append = false) {
    const request = ++sequence
    context = { ...filters }; search = term.trim()
    const nextPage = append ? page.value + 1 : 1
    if (!append) { rows.value = []; total.value = 0; page.value = 0 }
    loading.value = true; error.value = ''
    try {
      const data = await client.listStrategies({ ...context, search, page: nextPage, page_size: 20 })
      if (request !== sequence) return
      rows.value = append ? [...new Map([...rows.value, ...(data.items || [])].map(row => [row.id, row])).values()] : data.items || []
      total.value = data.total || 0; page.value = nextPage
    } catch (e) { if (request === sequence) error.value = e?.message || '策略读取失败' }
    finally { if (request === sequence) loading.value = false }
  }
  async function resolve(id) {
    const request = ++selectionSequence
    selected.value = null
    if (!id) return null
    try {
      const detail = await client.getStrategy(id)
      if (request !== selectionSequence) return null
      selected.value = detail
      return detail
    } catch (e) { if (request === selectionSequence) error.value = e?.message || '所选策略读取失败'; return null }
  }
  function invalidate() { sequence++; selectionSequence++; rows.value = []; total.value = 0; page.value = 0; selected.value = null; loading.value = false; error.value = '' }
  return { rows, total, loading, error, selected, hasMore, load, resolve, invalidate, more: () => load(context, search, true) }
}
