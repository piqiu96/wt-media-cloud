<script setup>
import { onMounted, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { createDiscoveryClient } from '../../../shared/api/discovery.js'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'

const client = createDiscoveryClient()
const rows = ref([]); const loading = ref(false); const error = ref(''); const visible = ref(false); const saving = ref(false); const editingID = ref(null)
const form = ref({ name: '', strategy_type: 'keyword', platform: 'douyin', config: { keywords: [], author: '' }, schedule: 'manual', timezone: 'Asia/Shanghai', status: 'disabled' })
const keywordText = ref('')
const columns = [
  { colKey: 'name', title: '策略名称', minWidth: 200 }, { colKey: 'strategy_type', title: '策略类型', width: 120 }, { colKey: 'platform', title: '平台', width: 110 }, { colKey: 'schedule', title: '执行周期', width: 150 }, { colKey: 'status', title: '状态', width: 110 }, { colKey: 'op', title: '操作', width: 280, fixed: 'right' },
]
onMounted(load)
async function load() { loading.value = true; try { const data = await client.listStrategies(); rows.value = Array.isArray(data) ? data : [] } catch (e) { error.value = e.message || '策略加载失败' } finally { loading.value = false } }
function openCreate() { editingID.value = null; form.value = { name: '', strategy_type: 'keyword', platform: 'douyin', config: { keywords: [], author: '' }, schedule: 'manual', timezone: 'Asia/Shanghai', status: 'disabled' }; keywordText.value = ''; visible.value = true }
function openEdit(row) { editingID.value = row.id; form.value = { name: row.name, strategy_type: row.strategy_type, platform: row.platform, config: { ...(row.config || {}) }, schedule: row.schedule || 'manual', timezone: row.timezone || 'Asia/Shanghai', status: row.status || 'disabled' }; keywordText.value = (row.config?.keywords || []).join('\n'); visible.value = true }
async function save() { saving.value = true; try { const config = { ...form.value.config }; if (form.value.strategy_type === 'keyword') config.keywords = keywordText.value.split(/[,\n]/).map((item) => item.trim()).filter(Boolean); if (editingID.value) await client.updateStrategy(editingID.value, { ...form.value, config }); else await client.createStrategy({ ...form.value, config }); visible.value = false; await load(); MessagePlugin.success(editingID.value ? '策略已更新' : '策略已保存') } catch (e) { error.value = e.message || '策略保存失败' } finally { saving.value = false } }
async function toggle(row) { try { await client.setStrategyStatus(row.id, row.status === 'enabled' ? 'disabled' : 'enabled'); await load() } catch (e) { error.value = e.message || '状态更新失败' } }
async function run(row) { try { await client.runStrategy(row.id); MessagePlugin.success('已创建挖掘任务') } catch (e) { error.value = e.message || '任务创建失败' } }
function typeLabel(type) { return type === 'author' ? '博主策略' : '关键词策略' }
</script>

<template>
  <t-loading :loading="loading" :show-overlay="true"><div class="wt-resource-page strategy-page">
    <t-alert v-if="error" theme="error" :message="error" closable style="margin-bottom:16px" @close="error=''" />
    <ResourcePageHeader title="挖掘策略" description="定义关键词自动发现规则，执行由 Cloud Scheduler 与 Crawler 负责">
      <template #actions><t-button theme="primary" @click="openCreate">新增策略</t-button><t-button class="wt-secondary-button" variant="outline" @click="load">刷新</t-button></template>
    </ResourcePageHeader>
    <ResourceCard class="strategy-card"><div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="rows" :columns="columns" row-key="id" hover :scroll="{ x: '900px' }" empty="暂无挖掘策略">
      <template #strategy_type="{ row }">{{ typeLabel(row.strategy_type) }}</template><template #status="{ row }"><ResourceStatusBadge :tone="row.status === 'enabled' ? 'success' : 'neutral'" :label="row.status === 'enabled' ? '启用' : '停用'" /></template>
      <template #op="{ row }"><t-space class="wt-resource-actions"><t-button size="small" theme="primary" :disabled="row.status !== 'enabled' || row.strategy_type === 'author'" @click="run(row)">立即执行</t-button><t-button size="small" class="wt-secondary-button" variant="outline" @click="openEdit(row)">编辑</t-button><t-button size="small" class="wt-secondary-button" variant="outline" @click="toggle(row)">{{ row.status === 'enabled' ? '停用' : '启用' }}</t-button><t-button size="small" class="wt-secondary-button" variant="outline" @click="$router.push(`/crawl-tasks?strategy_id=${row.id}`)">执行记录</t-button></t-space></template>
    </t-table></div></ResourceCard>
    <t-dialog v-model:visible="visible" :header="editingID ? '编辑挖掘策略' : '新增挖掘策略'" :confirm-btn="{ loading: saving, theme: 'primary', content: '保存' }" @confirm="save">
      <t-form label-width="92px"><t-form-item label="策略名称"><t-input v-model="form.name" placeholder="例如：王者荣耀热点" /></t-form-item><t-form-item label="策略类型"><t-radio-group v-model="form.strategy_type"><t-radio value="keyword">关键词</t-radio><t-radio value="author" disabled>博主（维护中）</t-radio></t-radio-group></t-form-item><t-form-item label="平台"><t-select v-model="form.platform"><t-option value="douyin" label="抖音" /><t-option value="bilibili" label="B站（待接入）" disabled /></t-select></t-form-item><t-form-item v-if="form.strategy_type === 'keyword'" label="关键词"><t-textarea v-model="keywordText" placeholder="每行一个关键词" /></t-form-item><t-form-item v-else label="博主账号"><t-input v-model="form.config.author" disabled placeholder="接口维护中，暂不可配置" /></t-form-item><t-form-item label="执行周期"><t-select v-model="form.schedule"><t-option value="manual" label="手动执行" /><t-option value="daily 09:00" label="每天 09:00" /><t-option value="interval:60" label="每 60 分钟" /></t-select></t-form-item><t-form-item label="状态"><t-switch v-model="form.status" :custom-value="['enabled', 'disabled']" /></t-form-item></t-form>
    </t-dialog>
  </div></t-loading>
</template>

<style scoped>.strategy-card { padding: 18px 20px; }</style>
