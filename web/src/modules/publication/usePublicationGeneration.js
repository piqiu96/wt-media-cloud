import { onScopeDispose, ref, watch } from 'vue';
import { parseBilibiliTags, platformGenerationFields } from './platformContent.js';
const limits = { title: 80, description: 1000, cover_title: 100, topics: 200, tags: 209 };
// Suggestions are transient. They never enter a draft until a separate adoption.
export function usePublicationGeneration({ context, values, fields, disabled, generate, adopt, getIdentity }) {
  const visible = ref(false), selected = ref([]), instruction = ref(''), busy = ref(false), error = ref(''), suggestion = ref(null), stage = ref('');
  const revisions = Object.fromEntries(Object.keys(limits).map(field => [field, 0])), latest = {};
  let inputRevision = 1, sequence = 0, controller, disposed = false, knownIdentity = '';
  function cancel() { sequence++; controller?.abort(); controller = null; busy.value = false; stage.value = ''; }
  watch(() => JSON.stringify(context.value), () => { inputRevision++; cancel(); suggestion.value = null; error.value = ''; }, { flush: 'sync' });
  for (const field of Object.keys(limits)) watch(() => values.value[field] || '', () => { revisions[field]++; }, { flush: 'sync' });
  watch(visible, show => { if (!show) { cancel(); suggestion.value = null; error.value = ''; } }, { flush: 'sync' });
  onScopeDispose(() => { disposed = true; cancel(); });
  async function start() {
    if (disposed || disabled.value || busy.value || !visible.value) return;
    const chosen = [...new Set(selected.value)].filter(field => fields.value.includes(field));
    if (!chosen.length) { error.value = '请选择需要生成的字段'; return; }
    const basis = context.value;
    if (!basis.rowKey || !basis.categoryId || !basis.outputId || !['baidu', 'bilibili'].includes(basis.platform)) { error.value = '请先选择平台、类目和成片'; return; }
    error.value = ''; busy.value = true; stage.value = '正在生成建议';
    const request = ++sequence, revision = inputRevision;
    controller = new AbortController();
    const fieldRevisions = Object.fromEntries(chosen.map(field => [field, String(revisions[field])]));
    for (const field of chosen) latest[field] = request;
    const platformKeys = Object.keys(platformGenerationFields(basis.platform));
    const input = { row_key: basis.rowKey, input_revision: String(revision), platform: basis.platform, category_id: String(basis.categoryId), composite_output_id: String(basis.outputId), fields: chosen, field_revisions: fieldRevisions, content: Object.fromEntries(platformKeys.map(field => [field, values.value[field] || ''])), instruction: instruction.value };
    try {
      const identityId = String(await getIdentity());
      if (disposed || request !== sequence || !visible.value) return;
      if (knownIdentity && knownIdentity !== identityId) suggestion.value = null;
      knownIdentity = identityId;
      const result = await generate(input, { signal: controller.signal, onEvent: ({ event, data }) => {
        if (['start', 'item_result', 'item_error'].includes(event) && (data.row_key !== input.row_key || data.input_revision !== input.input_revision)) throw new Error('生成结果与当前内容不一致，请重新生成');
      } });
      if (disposed || request !== sequence || revision !== inputRevision || !visible.value) return;
      const currentIdentity = String(await getIdentity());
      if (disposed || request !== sequence || revision !== inputRevision || !visible.value) return;
      if (currentIdentity !== identityId) {
        suggestion.value = null;
        knownIdentity = currentIdentity;
        throw new Error('登录账号已变化，请重新打开发布内容');
      }
      const length = (field,value) => basis.platform === 'bilibili' && field !== 'cover_title' ? value.length : [...value].length;
      const limit = field => basis.platform === 'bilibili' && field === 'description' ? 2000 : limits[field];
      if (!result?.content || !result.field_revisions || Object.keys(result.content).length !== chosen.length || Object.keys(result.field_revisions).length !== chosen.length || chosen.some(field => typeof result.content[field] !== 'string' || length(field,result.content[field]) > limit(field) || field !== 'topics' && !(basis.platform === 'bilibili' && field === 'description') && !result.content[field].trim() || result.field_revisions[field] !== fieldRevisions[field])) throw new Error('生成结果不完整，请重新生成');
      if (basis.platform === 'bilibili' && chosen.includes('tags')) parseBilibiliTags(result.content.tags);
      const next = { ...suggestion.value };
      for (const field of chosen) next[field] = { value: result.content[field], fieldRevision: fieldRevisions[field], request, revision, identityId, adopted: false };
      suggestion.value = next; stage.value = '请审阅后逐项采用';
    } catch (failure) {
      if (!disposed && request === sequence && visible.value) error.value = failure.name === 'AbortError' ? '已停止生成，填写内容已保留' : failure.message || '生成未完成，请重试';
    } finally { if (request === sequence) { busy.value = false; controller = null; } }
  }
  function canAdopt(field) {
    const item = suggestion.value?.[field];
    return !!item && !disposed && visible.value && !disabled.value && fields.value.includes(field) && !item.adopted && item.identityId === knownIdentity && item.revision === inputRevision && item.request === latest[field] && item.fieldRevision === String(revisions[field]);
  }
  async function apply(field) {
    if (!canAdopt(field)) return;
    const item = suggestion.value[field];
    try {
      const identityId = String(await getIdentity());
      if (disposed || !visible.value) return;
      if (identityId !== item.identityId) {
        suggestion.value = null;
        knownIdentity = identityId;
        throw new Error('登录账号已变化，请重新生成建议');
      }
      if (suggestion.value?.[field] !== item || !canAdopt(field)) return;
      item.adopted = true;
      adopt(field, item.value);
    } catch (failure) {
      if (!disposed && visible.value) error.value = failure.message || '暂时无法核对登录账号，请重试';
    }
  }

  return { visible, selected, instruction, busy, error, suggestion, stage, start, cancel, canAdopt, apply };
}
