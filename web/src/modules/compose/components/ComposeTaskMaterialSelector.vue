<script setup>
import { computed, ref, watch } from 'vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import MaterialCover from '../../materials/components/MaterialCover.vue'
import { createMaterialsClient } from '../../../shared/api/materials.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { materialRow, mergeSelection, acceptSelectedMaterials } from '../materialSelection.js'
const props = defineProps({ visible: Boolean, context: { type: Object, required: true }, selected: { type: Array, default: () => [] } })
const emit = defineEmits(['update:visible', 'selected'])
const client = createMaterialsClient()
const source = ref('mine'), rows = ref([]), total = ref(0), loading = ref(false), busy = ref(false), error = ref(''), selectedMap = ref({}), pageKeys = ref([]), advanced = ref(false), onlySelected = ref(false), failures = ref([])
const pagination = ref({ current: 1, pageSize: 10 })
const defaults = () => ({ search: '', published: [], my_usage: '', team_usage: '', view_count_min: null, view_count_max: null, like_count_min: null, like_count_max: null, favorite_count_min: null, favorite_count_max: null })
const filters = ref(defaults()), sort = ref({ sortBy: 'published_at', descending: true })
let sequence = 0
const tableScroll = computed(() => ({x:`${columns.reduce((sum,column) => sum + (column.width || 0),0)}px`}))
const selectedRows = computed(() => Object.values(selectedMap.value))
const shownRows = computed(() => onlySelected.value ? selectedRows.value.slice((pagination.value.current - 1) * pagination.value.pageSize, pagination.value.current * pagination.value.pageSize) : rows.value)
const usageOptions = [{value:'used',label:'已使用'},{value:'unused',label:'未使用'}]
const columns = [ { colKey:'row-select', type:'multiple', width:44, checkProps: ({row}) => ({disabled:row.selectable !== true || busy.value}) }, {colKey:'id',title:'ID',width:80}, {colKey:'material',title:'素材',width:290}, {colKey:'published_at',title:'发布时间',width:160,sorter:true}, {colKey:'view_count',title:'浏览量',width:100,sorter:true}, {colKey:'like_count',title:'点赞量',width:100,sorter:true}, {colKey:'favorite_count',title:'收藏量',width:100,sorter:true}, {colKey:'my_usage',title:'我的使用',width:110}, {colKey:'team_usage',title:'团队使用',width:110}, {colKey:'reason',title:'选择说明',width:180} ]
const selectionReason = (value) => ({source_unavailable:'来源失效，且没有可用视频',material_unavailable:'暂无可用输入'}[value] || '暂不可选择')
const metric = (value) => value === undefined || value === null ? '—' : Number(value).toLocaleString('zh-CN')
function syncKeys() { pageKeys.value = shownRows.value.filter((row) => selectedMap.value[row.id]).map((row) => row.id) }
watch(shownRows, syncKeys)
watch(() => props.visible, (visible) => {
  if (!visible) { ++sequence; return }
  selectedMap.value = Object.fromEntries(props.selected.map((row) => [row.id,row])); source.value = 'mine'; filters.value = defaults(); pagination.value.current = 1; onlySelected.value = false; failures.value = []; error.value = ''; load()
})
async function load() {
  if (!props.visible || onlySelected.value) { syncKeys(); return }
  const request = ++sequence
  loading.value = true; error.value = ''
  const params = { strategy_id: props.context.strategy_id, operator_id: props.context.operator_id, page: pagination.value.current, page_size: pagination.value.pageSize, search: filters.value.search, my_usage: filters.value.my_usage, team_usage: filters.value.team_usage, sort_by: sort.value.sortBy, sort_order: sort.value.descending ? 'desc':'asc' }
  const [from,through] = filters.value.published || []
  if (from) params.published_from = new Date(`${from}T00:00:00`).toISOString()
  if (through) { const end = new Date(`${through}T00:00:00`); end.setDate(end.getDate()+1); params.published_before = end.toISOString() }
  for (const key of ['view_count_min','view_count_max','like_count_min','like_count_max','favorite_count_min','favorite_count_max']) if (filters.value[key] !== null && filters.value[key] !== '') params[key] = filters.value[key]
  try {
    const data = await (source.value === 'mine' ? client.listMyMaterials(params) : client.list(params))
    if (request !== sequence) return
    rows.value = (data.items || []).map(materialRow); total.value = data.total || 0; syncKeys()
  } catch(e) { if (request === sequence) error.value = e?.message || '素材读取失败' }
  finally { if (request === sequence) loading.value = false }
}
function query() { pagination.value.current=1; load() }
function reset() { filters.value = defaults(); sort.value = {sortBy:'published_at',descending:true}; query() }
function select(keys) { selectedMap.value = mergeSelection(selectedMap.value, shownRows.value, keys); syncKeys() }
function changeSort(value) { sort.value = value || {sortBy:'published_at',descending:true}; query() }
function changeSelectedOnly() { pagination.value.current=1; if (onlySelected.value) syncKeys(); else load() }
function close() { if (!busy.value) emit('update:visible',false) }
async function confirm() {
  if (!selectedRows.value.length || busy.value) return
  busy.value=true; error.value=''
  const result = await acceptSelectedMaterials(selectedRows.value, (id) => client.addUsage(id, { strategy_id: props.context.strategy_id, operator_id: props.context.operator_id }))
  failures.value = result.failed
  if (result.accepted.length) emit('selected', result.accepted)
  if (result.failed.length) {
    error.value = `已使用 ${result.accepted.length} 条，${result.failed.length} 条未受理，请查看原因`
    selectedMap.value = Object.fromEntries(result.failed.map((row) => [row.id,row])); syncKeys()
  } else emit('update:visible',false)
  busy.value=false
}
</script>
<template>
  <ResourceOverlay :visible="visible" title="选择素材" :width="1240" :busy="busy" @close="close">
    <div class="material-selector">
      <p class="material-selector__context">负责人：{{ context.operator_name || context.operator_id }} · 团队 {{ context.team_name || context.team_id }} · 类目 {{ context.category_name || context.category_id }}</p>
      <t-tabs v-model="source" @change="query"><t-tab-panel value="mine" label="我的素材" /><t-tab-panel value="library" label="素材库" /></t-tabs>
      <p class="material-selector__hint">“我的素材”为负责人工作集；“我的使用”按当前登录人统计。</p>
      <FeedbackAlert v-if="error" theme="error" :message="error" />
      <div v-if="failures.length" class="material-selector__failures"><p v-for="row in failures" :key="row.id">{{ row.title || `素材 ${row.id}` }} · {{ row.selection_error }}</p></div>
      <div class="filter-bar"><label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">素材搜索</span><t-input v-model="filters.search" clearable placeholder="搜索素材或作者" @enter="query" /></label><label class="wt-filter-field material-selector__date"><span class="wt-filter-field__label">发布时间</span><t-date-range-picker v-model="filters.published" value-type="YYYY-MM-DD" clearable /></label><div class="wt-filter-actions"><t-button theme="primary" :disabled="onlySelected" @click="query">查询</t-button><t-button variant="outline" class="wt-secondary-button" :disabled="onlySelected" @click="reset">重置</t-button><t-button variant="text" @click="advanced = !advanced">{{ advanced ? '收起高级筛选' : '高级筛选' }}</t-button></div></div>
      <div v-if="advanced" class="wt-advanced-filters material-selector__advanced">
        <label class="wt-filter-field"><span class="wt-filter-field__label">我的使用</span><t-select v-model="filters.my_usage" :options="usageOptions" clearable placeholder="全部" /></label><label class="wt-filter-field"><span class="wt-filter-field__label">团队使用</span><t-select v-model="filters.team_usage" :options="usageOptions" clearable placeholder="全部" /></label>
        <label v-for="metricField in [{key:'view_count',label:'浏览量'},{key:'like_count',label:'点赞量'},{key:'favorite_count',label:'收藏量'}]" :key="metricField.key" class="material-selector__metric"><span class="wt-filter-field__label">{{ metricField.label }}</span><div><t-input-number theme="normal" v-model="filters[`${metricField.key}_min`]" :min="0" placeholder="最小" /><span>至</span><t-input-number theme="normal" v-model="filters[`${metricField.key}_max`]" :min="0" placeholder="最大" /></div></label>
      </div>
      <div class="wt-selection-toolbar material-selector__selection"><span>已选 {{ selectedRows.length }} 条素材</span><t-checkbox v-model="onlySelected" @change="changeSelectedOnly">只看已选</t-checkbox><t-link theme="primary" :disabled="busy || !selectedRows.length" @click="selectedMap = {}; syncKeys()">清空选择</t-link></div>
      <t-loading :loading="loading" :show-overlay="true"><div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="shownRows" :columns="columns" row-key="id" :selected-row-keys="pageKeys" :sort="sort" :scroll="tableScroll" size="small" hover @select-change="select" @sort-change="changeSort" empty="暂无符合条件的素材"><template #material="{row}"><div class="material-selector__material"><MaterialCover :url="row.cover_url" /><span :title="row.title">{{ row.title || '未命名素材' }}</span></div></template><template #published_at="{row}">{{ formatDateTime(row.published_at) }}</template><template #view_count="{row}">{{ metric(row.metric_presence?.view_count === false ? null : row.view_count) }}</template><template #like_count="{row}">{{ metric(row.metric_presence?.like_count === false ? null : row.like_count) }}</template><template #favorite_count="{row}">{{ metric(row.metric_presence?.favorite_count === false ? null : row.favorite_count) }}</template><template #my_usage="{row}">{{ row.my_usage === 'used' ? '已使用' : row.my_usage === 'unused' ? '未使用' : '—' }}</template><template #team_usage="{row}">{{ metric(row.usage?.output_count) }} 个成片</template><template #reason="{row}"><span>{{ row.selectable ? '' : selectionReason(row.selection_reason) }}</span><template v-if="row.strategy_history?.active_count">本策略有 {{ row.strategy_history.active_count }} 个进行中任务</template></template></t-table></div></t-loading>
      <div class="pagination-bar"><t-pagination v-model:current="pagination.current" v-model:pageSize="pagination.pageSize" :total="onlySelected ? selectedRows.length : total" :page-size-options="[10,20,50]" @current-change="load" @page-size-change="query" /></div>
    </div>
    <template #footer><t-button class="wt-secondary-button" variant="outline" :disabled="busy" @click="close">取消</t-button><t-button theme="primary" :disabled="!selectedRows.length" :loading="busy" @click="confirm">使用所选（{{ selectedRows.length }}）</t-button></template>
  </ResourceOverlay>
</template>
<style scoped>
.material-selector { display:grid; gap:14px; min-width:0; }
.material-selector > * { min-width:0; }
.material-selector .table-scroll-wrap { min-width:0; overflow-x:auto; width:100%; }
.material-selector__context { margin:0; color:var(--wt-text-primary); font-size:13px; }
.material-selector__hint { margin:0; color:var(--wt-text-tertiary); font-size:12px; }
.material-selector__advanced > .wt-filter-field { flex:0 1 180px; width:180px; min-width:0; }
.material-selector__advanced > .material-selector__metric { flex:0 0 276px; width:276px; }
.material-selector__advanced { display:flex; flex-wrap:wrap; gap:12px; padding-bottom:14px; border-bottom:1px solid var(--wt-border); }
.material-selector__metric { display:grid; gap:6px; }
.material-selector__metric > div,.material-selector__selection { display:flex; align-items:center; gap:12px; }
.material-selector__metric :deep(.t-input-number) { width:120px; }
.material-selector__selection { margin-bottom:0; }
.material-selector__material { display:flex; gap:10px; align-items:center; }
.material-selector__material > span { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.material-selector .filter-bar > .material-selector__date { flex:0 0 320px; min-width:280px; max-width:100%; }
.material-selector__failures { max-height:130px; overflow:auto; font-size:12px; color:var(--wt-text-secondary); }
</style>
