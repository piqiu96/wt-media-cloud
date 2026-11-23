<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { invoke } from '@tauri-apps/api/core'
import { feedback } from '../../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../../shared/ui/feedback/FeedbackAlert.vue'
import ResourceCard from '../../../../shared/ui/resource/ResourceCard.vue'
import { createLocalSettingsService, DEFAULT_TAIL_LINES } from '../local-settings/service.js'
import { formatModified, logKindLabel, formatBytes, describeDiagnostic } from '../local-settings/local-settings-view.js'
import { createLocalLogsPage, NO_LEVEL } from './local-logs-view.js'

const props = defineProps({initialSource:{type:String,default:''}})
const service = createLocalSettingsService({ invoke })
const listing = ref(null), source = ref(''), name = ref(''), tail = ref(null)
const level = ref(NO_LEVEL), lines = ref(DEFAULT_TAIL_LINES)
const listLoading = ref(false), tailLoading = ref(false), exporting = ref(false)
const listError = ref(''), tailError = ref(''), diagnostic = ref(null)
let listEpoch = 0, tailEpoch = 0
const messageOf = error => error?.message || String(error) || '读取失败'
const page = computed(() => createLocalLogsPage({listing:listing.value, source:source.value, name:name.value, tail:tail.value, level:level.value}))
const sourceLabel = value => value === 'desktop' ? '本机控制台日志' : '本机执行服务日志'
const lineOptions = [500,1000,5000].map(value => ({label:`${value.toLocaleString()} 行`,value}))
async function reloadListing() {
  const epoch = ++listEpoch
  listLoading.value = true
  listError.value = ''
  try {
    const next = await service.logFiles()
    if (epoch !== listEpoch) return
    listing.value = next
    const exists = next.trees.some(tree => tree.source === source.value && tree.files.some(file => file.name === name.value))
    if (!exists) {
      const first = next.trees.find(tree => tree.source === props.initialSource && tree.files.length) || next.trees.find(tree => tree.files.length)
      source.value = first?.source || ''
      name.value = first?.files[0]?.name || ''
    }
    await reloadTail()
  } catch (error) { if (epoch === listEpoch) listError.value = messageOf(error) }
  finally { if (epoch === listEpoch) listLoading.value = false }
}
async function reloadTail() {
  const epoch = ++tailEpoch
  tail.value = null
  tailError.value = ''
  if (!source.value || !name.value) { tailLoading.value = false; return }
  tailLoading.value = true
  try {
    const next = await service.logTail({source:source.value,name:name.value,lines:lines.value,minLevel:level.value})
    if (epoch === tailEpoch) tail.value = next
  } catch (error) { if (epoch === tailEpoch) tailError.value = messageOf(error) }
  finally { if (epoch === tailEpoch) tailLoading.value = false }
}
function selectFile(nextSource, nextName) {
  source.value = nextSource
  name.value = nextName
  void reloadTail()
}
watch([level,lines], reloadTail)
watch(() => props.initialSource, value => {
 const tree = listing.value?.trees.find(tree => tree.source === value && tree.files.length)
 if (tree) selectFile(tree.source, tree.files[0].name)
})
async function openFolder() {
  try { await service.openPlace(source.value) }
  catch (error) { feedback.toast.error(messageOf(error)) }
}
async function exportDiagnostic() {
  if (exporting.value) return
  exporting.value = true
  try {
    const report = await service.diagnosticExport()
    diagnostic.value = { ...describeDiagnostic(report), path:report.path }
    if (diagnostic.value.theme === 'success') feedback.toast.success('诊断包已导出')
    else feedback.toast.info('诊断包已生成，部分信息未收录')
  } catch (error) { feedback.toast.error(messageOf(error)) }
  finally { exporting.value = false }
}
onMounted(reloadListing)
onUnmounted(() => { listEpoch++; tailEpoch++ })
</script>
<template>
  <ResourceCard class="logs-page">
    <div class="log-toolbar">
      <label class="log-filter"><span>日志级别</span><t-select v-model="level" :options="page.filterOptions" aria-label="日志级别" placeholder="全部" /></label>
      <label class="log-filter"><span>文件末尾</span><t-select v-model="lines" :options="lineOptions" aria-label="读取行数" /></label>
      <div class="toolbar-actions"><t-button variant="outline" :loading="listLoading" @click="reloadListing">刷新</t-button><t-button variant="outline" :disabled="!source" @click="openFolder">打开日志目录</t-button><t-button variant="outline" :loading="exporting" @click="exportDiagnostic">导出诊断包</t-button></div>
    </div>
    <FeedbackAlert v-if="diagnostic" :theme="diagnostic.theme" :message="diagnostic.text" closable @close="diagnostic = null"><template #description><pre class="diagnostic-detail">{{ diagnostic.detail }}</pre></template></FeedbackAlert>
    <div class="log-panels">
      <aside class="log-file-list" aria-label="日志文件">
        <FeedbackAlert v-if="listError" theme="error" :message="listError" />
        <t-loading :loading="listLoading">
          <div v-for="tree in page.trees" :key="tree.source" class="log-source">
            <h3>{{ sourceLabel(tree.source) }}</h3>
            <button v-for="file in tree.files" :key="file.name" class="log-file" :class="{selected:file.selected}" type="button" :aria-pressed="file.selected" @click="selectFile(tree.source,file.name)"><span class="log-file-name">{{ file.name }}</span><span class="file-meta"><t-tag size="small" :theme="file.kind === 'live' ? 'primary' : 'default'" variant="light">{{ logKindLabel(file.kind) }}</t-tag>{{ formatBytes(file.bytes) }} · {{ formatModified(file.modifiedSeconds) }}</span></button>
            <p v-if="!tree.files.length" class="secondary">暂无日志文件</p>
          </div>
          <t-empty v-if="!listLoading && !page.trees.length && !listError" description="暂无日志文件" />
        </t-loading>
      </aside>
      <section class="log-viewer" aria-label="日志内容">
        <header v-if="page.canView" class="log-viewer-heading"><strong>{{ name }}</strong><span class="log-path">{{ tail?.path || `${page.selectedDirectory}/${name}` }}</span><span class="secondary">{{ page.countsText }}<template v-if="tail"> · {{ page.appliedFilterText }}<template v-if="level">及以上</template></template></span></header>
        <FeedbackAlert v-if="tailError" theme="error" :message="tailError"><template #operation><t-button variant="text" @click="reloadTail">重试</t-button></template></FeedbackAlert>
        <t-loading :loading="tailLoading" class="log-tail-loading">
          <div v-if="page.hasLines" class="log-tail"><div v-for="entry in page.lines" :key="entry.key" class="log-line"><t-tag v-if="entry.levelText" size="small" :theme="entry.levelTheme" variant="light">{{ entry.levelText }}</t-tag><span v-else class="continuation-level" /><code>{{ entry.text }}</code></div></div>
          <t-empty v-else-if="!tailLoading && !tailError" :description="page.canView ? '当前范围内没有日志内容' : '请选择日志文件'" />
        </t-loading>
        <p v-if="page.hiddenNotice || page.truncatedNotice" class="secondary tail-note">{{ page.hiddenNotice }} {{ page.truncatedNotice }}</p>
      </section>
    </div>
  </ResourceCard>
</template>
<style scoped>
.logs-page { margin-top:16px; min-width:0; }.log-toolbar { display:flex; align-items:center; gap:16px; flex-wrap:wrap; padding-bottom:16px; }.log-filter { display:flex; gap:8px; align-items:center; white-space:nowrap; }.log-filter .t-select__wrap { width:130px; }.toolbar-actions { margin-left:auto; display:flex; gap:8px; flex-wrap:wrap; }.log-panels { display:grid; grid-template-columns:280px minmax(0,1fr); border:1px solid var(--wt-border); border-radius:6px; overflow:hidden; }.log-file-list { min-width:0; overflow:auto; max-height:65vh; padding:16px 12px; border-right:1px solid var(--wt-border); }.log-source h3 { font-size:13px; font-weight:500; color:var(--wt-text-secondary); margin:0 0 10px; }.log-source + .log-source { margin-top:20px; }.log-file { display:grid; gap:6px; width:100%; padding:10px; margin-bottom:4px; text-align:left; border:0; border-left:3px solid transparent; border-radius:4px; background:transparent; cursor:pointer; color:var(--wt-text-secondary); }.log-file:hover { background:var(--wt-bg-page); }.log-file.selected { background:var(--wt-primary-light,#eaf2ff); border-left-color:var(--wt-primary); }.log-file-name { font-size:13px; overflow-wrap:anywhere; }.file-meta { display:flex; flex-wrap:wrap; align-items:center; gap:6px; font-size:11px; color:var(--wt-text-tertiary); }.log-viewer { min-width:0; display:flex; flex-direction:column; max-height:65vh; }.log-viewer-heading { display:grid; gap:6px; padding:16px; border-bottom:1px solid var(--wt-border); flex:none; }.log-viewer-heading strong { font-size:14px; font-weight:500; }.log-path,.secondary { color:var(--wt-text-tertiary); font-size:12px; overflow-wrap:anywhere; line-height:20px; }.log-tail-loading { min-height:120px; min-width:0; overflow:auto; }.log-tail { overflow:auto; padding:12px 16px; }.log-line { display:flex; gap:10px; align-items:flex-start; min-width:max-content; line-height:24px; }.log-line .t-tag,.continuation-level { width:44px; flex:none; justify-content:center; margin-top:3px; }.log-line code { white-space:pre; font-family:ui-monospace,SFMono-Regular,Consolas,monospace; font-size:12px; color:var(--wt-text-secondary); }.tail-note { padding:8px 16px; margin:0; border-top:1px solid var(--wt-border); }.diagnostic-detail { white-space:pre-wrap; overflow-wrap:anywhere; font:inherit; margin:0; }
@media(max-width:1100px) { .log-panels { grid-template-columns:230px minmax(0,1fr); }.log-filter .t-select__wrap { width:110px; }.toolbar-actions { margin-left:0; } }
@media(max-width:700px) { .log-panels { grid-template-columns:1fr; }.log-file-list { max-height:220px; border-right:0; border-bottom:1px solid var(--wt-border); } }
</style>
