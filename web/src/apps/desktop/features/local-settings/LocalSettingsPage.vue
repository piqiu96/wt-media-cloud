<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { invoke } from '@tauri-apps/api/core'
import { feedback } from '../../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../../shared/ui/feedback/FeedbackAlert.vue'
import ResourceCard from '../../../../shared/ui/resource/ResourceCard.vue'
import ResourceSection from '../../../../shared/ui/resource/ResourceSection.vue'
import ResourcePageHeader from '../../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceOverlay from '../../../../shared/ui/resource/ResourceOverlay.vue'
import ResourceStatusBadge from '../../../../shared/ui/resource/ResourceStatusBadge.vue'
import LocalLogsPage from '../local-logs/LocalLogsPage.vue'
import { createLocalSettingsService } from './service.js'
import { FILE_CHOICES, choiceLabel, chosenNames, describeCleanup, describeMigration, describeMigrationPlan, describePush, describeSearchDirectory, formatBytes, migrationRows } from './local-settings-view.js'
import { loadCandidateNames } from './migrationCandidates.js'

const service = createLocalSettingsService({ invoke })
const route = useRoute(), router = useRouter()
const tab = computed({ get: () => route.query.tab === 'log' ? 'log' : 'storage', set: value => router.replace({ query: { ...route.query, tab: value === 'log' ? 'log' : undefined } }) })
const settings = ref(null), usage = ref(null)
const loading = ref(true), usageLoading = ref(false), busy = ref('')
const settingsError = ref(''), usageError = ref(''), pushNote = ref(null), cleanupResult = ref(null)
const history = computed(() => (settings.value?.knownSaveDirs || []).filter(dir => dir !== settings.value?.saveDir))
const messageOf = error => error?.message || String(error) || '操作失败'
const logSource = [ { source: 'desktop', label: '本机控制台日志' }, { source: 'agent', label: '本机执行服务日志' } ]
async function readSettings() {
  try { settings.value = await service.getSettings(); settingsError.value = '' }
  catch (error) { settingsError.value = messageOf(error) }
}
async function readUsage() {
  usageLoading.value = true
  try { usage.value = await service.storageUsage(); usageError.value = '' }
  catch (error) { usage.value = null; usageError.value = messageOf(error) }
  finally { usageLoading.value = false }
}
async function reload() {
  loading.value = true
  await Promise.all([readSettings(), readUsage()])
  loading.value = false
}
async function run(key, work) {
  if (busy.value) return
  busy.value = key
  try { return await work() }
  catch (error) { feedback.toast.error(messageOf(error)) }
  finally { busy.value = '' }
}
async function pushSaveDir() {
  try {
    const facts = await service.pushSaveDir()
    const note = describePush(facts)
    const matches = facts.saveDir === settings.value?.saveDir
    pushNote.value = matches && facts.writable ? null : matches ? note : {theme:'warning',text:`目录已保存，但执行服务当前目录为 ${facts.saveDir || '未设置'}，尚未应用所选目录。请重试应用。`}
    if (matches && facts.writable) feedback.toast.success('下载目录已保存并应用')
  } catch (error) {
    pushNote.value = { theme:'warning', text:`目录已保存，本机执行服务尚未应用：${messageOf(error)}。可重试应用，或在服务重启后读取。` }
  }
}
async function saveSaveDir() {
  await run('save', async () => {
    const written = await service.pickSaveDirectory()
    if (!written) return
    const dropped = (settings.value?.knownSaveDirs || []).filter(dir => !written.knownSaveDirs.includes(dir))
    settings.value = written
    pushNote.value = null
    await pushSaveDir()
    await readUsage()
    if (dropped.length) await feedback.notice({title:'历史查找目录已更新',message:`登记达到上限，以下目录不再参与查找，文件保留：${dropped.join('、')}`})
  })
}
async function clearSaveDir() {
  if (!await feedback.confirmWarning({ title:'取消默认下载目录？', message:'仅取消默认位置，已下载文件和历史查找登记保留。', confirmText:'取消默认目录' })) return
  await run('clear', async () => {
    settings.value = await service.setSaveDir(null)
    pushNote.value = { theme:'warning', text:'已取消默认下载目录。已运行的执行服务可能仍使用原位置；重启后按任务规则处理。' }
    feedback.toast.success('默认下载目录已取消')
  })
}
async function addSearchDirectory() {
  await run('add', async () => {
    const answer = await service.pickSearchDirectory()
    if (!answer) return
    const result = describeSearchDirectory(answer)
    await readSettings()
    if (answer.dropped) await feedback.notice({title:'历史查找目录已更新', message:`${result.text} ${result.detail || ''}`})
    else feedback.toast.info(result.text)
  })
}
async function removeDirectory(directory) {
  if (!await feedback.confirmWarning({ title:'移除历史查找目录？', message:`${directory}\n移除后不再从此处查找文件。目录、文件和下载记录保留，重新添加即可恢复查找。`, confirmText:'移除' })) return
  await run('remove', async () => {
    settings.value = await service.removeSearchDirectory(directory)
    feedback.toast.success('已移除查找登记，文件保留')
  })
}
async function openDirectory(directory) {
  await run('open', () => service.openSaveDirectory(directory))
}
async function clean(target) {
  const label = target === 'cache' ? '缓存' : logSource.find(item => item.source === target).label
  if (!await feedback.confirmWarning({ title:`清理${label}？`, message: target === 'cache' ? '仅清理可再生缓存，保留素材和成片。' : '仅清理已轮转日志，保留正在写入的日志。', confirmText:'清理' })) return
  await run(`clean-${target}`, async () => {
    const report = target === 'cache' ? await service.cacheCleanup() : await service.logCleanup(target)
    const result = describeCleanup(report)
    cleanupResult.value = report.failures.length ? result : null
    if (!report.failures.length) feedback.toast.success(result.text)
    await readUsage()
  })
}
function viewLogs(source) { router.replace({query:{...route.query,tab:'log',source}}) }
const migrationVisible = ref(false), migrationLoading = ref(false), migrationBusy = ref('')
const migrationPlan = ref(null), migrationSummary = ref(null), migrationChoices = ref({}), migrationError = ref(''), migrationResult = ref(null)
let candidateNames = []
const migrationRowsFor = computed(() => migrationPlan.value ? migrationRows(migrationPlan.value, migrationChoices.value) : [])
const movableNames = computed(() => chosenNames(migrationRowsFor.value, FILE_CHOICES.move))
const deletableNames = computed(() => chosenNames(migrationRowsFor.value, FILE_CHOICES.delete))
const CHOICE_OPTIONS = [FILE_CHOICES.keep, FILE_CHOICES.move, FILE_CHOICES.delete]
const MIGRATION_COLUMNS = [{colKey:'name',title:'文件',ellipsis:true},{colKey:'bytesText',title:'大小',width:100},{colKey:'statusLabel',title:'状态',width:110},{colKey:'choice',title:'处理方式',width:230}]
async function openMigration() {
  migrationVisible.value = true
  migrationLoading.value = true
  migrationError.value = ''
  migrationPlan.value = null
  migrationResult.value = null
  migrationChoices.value = {}
  try {
    candidateNames = await loadCandidateNames()
    if (!candidateNames.length) return
    migrationPlan.value = await service.migrationPlan(candidateNames)
    migrationSummary.value = describeMigrationPlan(migrationPlan.value)
  } catch (error) { migrationError.value = messageOf(error) }
  finally { migrationLoading.value = false }
}
function choose(name, choice) { migrationChoices.value = {...migrationChoices.value, [name]:choice} }
async function runMigration(action) {
  const names = action === FILE_CHOICES.delete ? deletableNames.value : movableNames.value
  if (migrationBusy.value || !names.length) return
  if (action === FILE_CHOICES.delete && !await feedback.confirmWarning({title:`删除 ${names.length} 个本地文件？`,message:'删除后无法撤销，云端业务记录保留。请确认已选文件。',confirmText:'删除文件',danger:true})) return
  migrationBusy.value = action
  migrationError.value = ''
  try {
    const report = action === FILE_CHOICES.delete ? await service.deleteSavedFiles(names) : await service.moveSavedFiles(names)
    migrationResult.value = describeMigration(report, action)
    // 已完成项不重复执行；部分失败的选择保留，可核对后重试。
    const completed = new Set([...report.moved, ...report.deleted, ...report.missing])
    migrationChoices.value = Object.fromEntries(Object.entries(migrationChoices.value).filter(([name]) => !completed.has(name)))
    migrationPlan.value = await service.migrationPlan(candidateNames)
    migrationSummary.value = describeMigrationPlan(migrationPlan.value)
    await readUsage()
  } catch (error) { migrationError.value = messageOf(error) }
  finally { migrationBusy.value = '' }
}
onMounted(reload)
</script>

<template>
  <main class="settings-page">
    <ResourcePageHeader title="桌面设置" description="管理这台电脑的下载、存储和日志。" />
    <t-tabs v-model="tab" class="settings-tabs">
      <t-tab-panel value="storage" label="下载与存储">
        <div class="settings-content">
          <ResourceCard>
            <ResourceSection title="下载目录">
              <t-loading :loading="loading">
                <FeedbackAlert v-if="settingsError" theme="error" :message="settingsError"><template #operation><t-button variant="text" @click="readSettings">重试</t-button></template></FeedbackAlert>
                <template v-else>
                  <dl class="path-details"><div><dt>系统配置文件</dt><dd>{{ settings?.file || '未读取到' }}</dd></div></dl>
                  <div class="setting-row"><div class="setting-value"><span class="directory"><t-icon name="folder" />{{ settings?.saveDir || '请先选择下载目录' }}</span><span class="secondary">新下载的文件保存到此处，更改目录不会搬运已有文件。</span></div><div class="actions"><t-button variant="outline" :loading="busy === 'save'" :disabled="!!busy" @click="saveSaveDir">{{ settings?.saveDir ? '更改目录' : '选择目录' }}</t-button><t-button v-if="settings?.saveDir" variant="outline" :disabled="!!busy" @click="openDirectory(settings.saveDir)">打开目录</t-button><t-button v-if="settings?.saveDir" variant="text" :disabled="!!busy" @click="clearSaveDir">取消默认目录</t-button></div></div>
                  <FeedbackAlert v-if="pushNote" :theme="pushNote.theme" :message="pushNote.text"><template #operation><t-button v-if="settings?.saveDir" variant="text" :disabled="!!busy" @click="run('push', pushSaveDir)">重试应用</t-button></template></FeedbackAlert>
                </template>
              </t-loading>
            </ResourceSection>
          </ResourceCard>
          <ResourceCard>
            <ResourceSection title="历史文件查找目录" description="用于查找本机已有文件，不改变新下载位置。最多登记8个位置（含当前下载目录）。">
              <template #title-extra><t-button class="section-action" variant="outline" :disabled="!!busy || !!settingsError" @click="addSearchDirectory"><t-icon name="add" />添加目录</t-button></template>
              <div v-for="directory in history" :key="directory" class="setting-row history-row"><span class="directory"><t-icon name="folder" />{{ directory }}</span><div class="actions"><t-button variant="text" :disabled="!!busy" @click="openDirectory(directory)">打开目录</t-button><t-button variant="text" :disabled="!!busy" @click="removeDirectory(directory)">移除</t-button></div></div>
              <t-empty v-if="!history.length" description="暂无历史查找目录" />
            </ResourceSection>
          </ResourceCard>
          <ResourceCard><ResourceSection title="整理已下载文件" description="逐个选择保留、移动到当前下载目录或删除，默认保留。"><template #title-extra><t-button class="section-action" variant="outline" :disabled="!!busy || !settings?.saveDir || !!settingsError" @click="openMigration">整理文件</t-button></template><p class="secondary">按下载记录查找文件，仅处理你选中的文件。</p></ResourceSection></ResourceCard>
          <ResourceCard>
            <ResourceSection title="存储与清理">
              <template #title-extra><div class="section-action actions"><span class="secondary">磁盘可用 {{ formatBytes(usage?.availableBytes) }}</span><t-button variant="text" :loading="usageLoading" :disabled="!!busy" @click="readUsage">刷新</t-button></div></template>
              <FeedbackAlert v-if="usageError" theme="error" :message="usageError" />
              <div class="storage-head"><span>内容</span><span>占用空间</span><span>操作</span></div>
              <div class="storage-row"><span>缓存</span><span>{{ formatBytes(usage?.cacheBytes) }}</span><div class="actions"><t-button variant="text" :disabled="!!busy || !usage" @click="clean('cache')">清理</t-button></div></div>
              <div v-for="tree in logSource" :key="tree.source" class="storage-row"><span>{{ tree.label }}</span><span>{{ formatBytes(usage?.logs.find(item => item.source === tree.source)?.bytes) }}</span><div class="actions"><t-button variant="text" @click="viewLogs(tree.source)">查看日志</t-button><t-button variant="text" :disabled="!!busy || !usage" @click="clean(tree.source)">清理</t-button></div></div>
              <p class="secondary storage-note">仅清理可再生缓存和已轮转日志，保留素材、成片与正在写入的日志。</p>
              <FeedbackAlert v-if="cleanupResult" :theme="cleanupResult.theme" :message="cleanupResult.text"><template #description><pre class="result-detail">{{ cleanupResult.detail }}</pre></template></FeedbackAlert>
            </ResourceSection>
          </ResourceCard>
        </div>
      </t-tab-panel>
      <t-tab-panel value="log" label="本地日志"><LocalLogsPage v-if="tab === 'log'" :initial-source="typeof route.query.source === 'string' ? route.query.source : ''" /></t-tab-panel>
    </t-tabs>
    <ResourceOverlay v-model:visible="migrationVisible" title="整理已下载文件" :width="960" :busy="!!migrationBusy || migrationLoading">
      <t-loading :loading="migrationLoading">
        <FeedbackAlert v-if="migrationError" theme="error" :message="migrationError" />
        <FeedbackAlert v-if="migrationResult" :theme="migrationResult.theme" :message="migrationResult.text"><template #description><pre class="result-detail">{{ migrationResult.detail }}</pre></template></FeedbackAlert>
        <template v-if="migrationPlan">
          <p class="secondary">目标目录：{{ migrationPlan.to }}</p>
          <FeedbackAlert v-if="migrationSummary?.theme === 'warning'" theme="warning" :message="migrationSummary.text"><template #description><pre class="result-detail">{{ migrationSummary.detail }}</pre></template></FeedbackAlert>
          <p v-else class="secondary">{{ migrationSummary?.text }}</p>
          <t-table :data="migrationRowsFor" :columns="MIGRATION_COLUMNS" row-key="name" size="small" max-height="360">
            <template #name="{row}"><span>{{ row.name }}</span><div class="secondary">{{ row.directory }}</div></template>
            <template #statusLabel="{row}"><ResourceStatusBadge :tone="row.statusTone" :label="row.statusLabel" /></template>
            <template #choice="{row}"><t-space v-if="row.selectable" size="small"><t-button v-for="option in CHOICE_OPTIONS" :key="option" size="small" :theme="row.choice === option ? 'primary' : 'default'" :variant="row.choice === option ? 'base' : 'outline'" :disabled="!!migrationBusy" @click="choose(row.name,option)">{{ choiceLabel(option) }}</t-button></t-space><span v-else>—</span></template>
          </t-table>
        </template>
        <t-empty v-else-if="!migrationLoading && !migrationError" description="暂无可整理的下载记录" />
      </t-loading>
      <template #footer><t-button variant="outline" :disabled="!!migrationBusy" @click="migrationVisible = false">关闭</t-button><t-button theme="danger" variant="outline" :disabled="!!migrationBusy || !deletableNames.length" :loading="migrationBusy === FILE_CHOICES.delete" @click="runMigration(FILE_CHOICES.delete)">删除所选（{{ deletableNames.length }}）</t-button><t-button theme="primary" :disabled="!!migrationBusy || !movableNames.length" :loading="migrationBusy === FILE_CHOICES.move" @click="runMigration(FILE_CHOICES.move)">移动所选（{{ movableNames.length }}）</t-button></template>
    </ResourceOverlay>
  </main>
</template>
<style scoped>
.settings-page { min-width:0; }.settings-tabs { background:transparent; }.settings-content { display:grid; gap:16px; padding-top:16px; }.setting-row { display:flex; align-items:center; justify-content:space-between; gap:16px; padding:12px 0; }.setting-value { display:grid; gap:6px; min-width:0; }.directory { display:flex; align-items:flex-start; gap:8px; overflow-wrap:anywhere; min-width:0; }.directory .t-icon { flex:none; margin-top:3px; color:var(--wt-primary); }.actions { display:flex; align-items:center; gap:8px; flex-wrap:wrap; flex-shrink:0; }.secondary { color:var(--wt-text-tertiary); font-size:13px; line-height:22px; overflow-wrap:anywhere; margin:0; }.section-action { margin-left:auto; font-weight:400; }.path-details { margin:0 0 12px; padding-bottom:16px; border-bottom:1px solid var(--wt-border); }.path-details > div { display:grid; grid-template-columns:112px minmax(0,1fr); gap:16px; }.path-details dt { color:var(--wt-text-tertiary); }.path-details dd { margin:0; overflow-wrap:anywhere; }.history-row + .history-row { border-top:1px solid var(--wt-border); }.storage-head,.storage-row { display:grid; grid-template-columns:minmax(160px,1fr) 140px 200px; gap:16px; align-items:center; padding:14px 16px; }.storage-head { background:var(--wt-bg-table-header,#f2f5fa); font-weight:500; }.storage-row { border-bottom:1px solid var(--wt-border); }.storage-note { margin-top:16px; }.result-detail { white-space:pre-wrap; overflow-wrap:anywhere; margin:4px 0 0; font-family:inherit; font-size:13px; }
@media(max-width:1100px) { .setting-row { align-items:flex-start; flex-wrap:wrap; }.storage-head,.storage-row { grid-template-columns:minmax(130px,1fr) 90px 155px; padding:12px 8px; gap:8px; }.storage-row .actions { gap:0; } }
</style>
