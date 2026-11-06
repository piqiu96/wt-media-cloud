<script setup>
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { CloudUploadIcon, VideoIcon } from 'tdesign-icons-vue-next'
import { opColumnWidth } from '../../../shared/ui/resource/opColumnWidth.js'
import { createFilesClient } from '../../../shared/api/files.js'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { feedback } from '../../../shared/ui/feedback/index.js'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import ResourceCard from '../../../shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../shared/ui/resource/ResourcePageHeader.vue'
import ResourceFilterBar from '../../../shared/ui/resource/ResourceFilterBar.vue'
import ResourceOverlay from '../../../shared/ui/resource/ResourceOverlay.vue'
import ResourceForm from '../../../shared/ui/resource/ResourceForm.vue'
import ResourceRowActions from '../../../shared/ui/resource/ResourceRowActions.vue'

const client = createFilesClient()
const items = ref([]), loading = ref(false), error = ref('')
const filters = ref({ search: '', type: '' })
const page = ref(1), pageSize = ref(20), total = ref(0), uploadMaxBytes = ref(0)
const uploadVisible = ref(false), uploadName = ref(''), uploadFile = ref(null), previewURL = ref('')
const busy = ref(false), formError = ref('')
const renameVisible = ref(false), editing = ref(null), renameName = ref('')
const linkVisible = ref(false), linkValue = ref('')
const columns = [
  { colKey: 'id', title: 'ID', width: 70 },
  { colKey: 'filename', title: '文件名', width: 260, ellipsis: true },
  { colKey: 'type', title: '类型', width: 80 },
  { colKey: 'size_bytes', title: '大小', width: 100 },
  { colKey: 'owner_name', title: '上传人', width: 120 },
  { colKey: 'team_name', title: '运营团队', width: 130 },
  { colKey: 'created_at', title: '上传时间', width: 160 },
  { colKey: 'op', title: '操作', width: opColumnWidth(['预览', '下载', '复制链接', '改名', '删除'], { variant: 'text' }), fixed: 'right' },
]
const tableScroll = { x: `${columns.reduce((sum, column) => sum + column.width, 0)}px` }
const acceptedFiles = '.png,.jpg,.jpeg,.webp,.gif,.mp4,.m4v,.mov,.webm,.mkv'
let sequence = 0
function sizeLabel(bytes) { return bytes >= 1048576 ? `${(bytes / 1048576).toFixed(1)} MB` : `${(bytes / 1024).toFixed(1)} KB` }
async function load() {
  const current = ++sequence
  loading.value = true; error.value = ''
  try {
    const data = await client.list({ ...filters.value, page: page.value, page_size: pageSize.value })
    if (current !== sequence) return
    items.value = data.items || []; total.value = data.total || 0; uploadMaxBytes.value = data.upload_max_bytes || 0
  } catch (e) { if (current === sequence) error.value = e.message }
  finally { if (current === sequence) loading.value = false }
}
function query() { page.value = 1; load() }
function reset() { filters.value = { search: '', type: '' }; query() }
function clearSelection() {
  if (previewURL.value) URL.revokeObjectURL(previewURL.value)
  previewURL.value = ''; uploadFile.value = null
}
function openUpload() {
  clearSelection(); uploadName.value = ''; formError.value = ''; uploadVisible.value = true
}
function chooseFile(files) {
  if (busy.value) return
  const file = files?.[0]
  if (!file) return
  const extension = file.name.split('.').pop().toLowerCase()
  if (!file.name.includes('.') || !acceptedFiles.split(',').includes(`.${extension}`)) { formError.value = '请选择支持的图片或视频文件'; return }
  if (!file.size) { formError.value = '不能上传空文件'; return }
  if (uploadMaxBytes.value && file.size > uploadMaxBytes.value) { formError.value = `文件不能超过 ${sizeLabel(uploadMaxBytes.value)}`; return }
  clearSelection()
  uploadFile.value = file; uploadName.value = file.name; formError.value = ''
  if (['png', 'jpg', 'jpeg', 'webp', 'gif'].includes(extension)) previewURL.value = URL.createObjectURL(file)
}
watch(uploadVisible, visible => { if (!visible) clearSelection() })
onUnmounted(clearSelection)
async function upload() {
  if (busy.value) return
  if (!uploadName.value.trim() || !uploadFile.value) { formError.value = '请选择文件并填写文件名'; return }
  if (uploadMaxBytes.value && uploadFile.value.size > uploadMaxBytes.value) { formError.value = `文件不能超过 ${sizeLabel(uploadMaxBytes.value)}`; return }
  busy.value = true; formError.value = ''
  try {
    await client.upload(uploadName.value.trim(), uploadFile.value)
    uploadVisible.value = false; feedback.toast.success('文件上传成功'); page.value = 1; await load()
  } catch (e) { formError.value = e.message }
  finally { busy.value = false }
}
function openRename(file) { editing.value = file; renameName.value = file.name; formError.value = ''; renameVisible.value = true }
async function rename() {
  if (busy.value) return
  if (!renameName.value.trim()) { formError.value = '请填写文件名'; return }
  busy.value = true; formError.value = ''
  try { await client.rename(editing.value.id, renameName.value.trim()); renameVisible.value = false; feedback.toast.success('文件已改名'); await load() }
  catch (e) { formError.value = e.message }
  finally { busy.value = false }
}
async function copy(file) {
  try { linkValue.value = client.publicURL(file) }
  catch (e) { feedback.toast.error(e.message); return }
  try { await navigator.clipboard.writeText(linkValue.value); feedback.toast.success('文件链接已复制') }
  catch { linkVisible.value = true }
}
async function remove(file) {
  if (!await feedback.confirmWarning({ title: '删除文件', message: `删除「${file.filename}」后，其链接将无法继续使用。`, confirmText: '删除', danger: true })) return
  try { await client.delete(file.id); feedback.toast.success('文件已删除'); if (items.value.length === 1 && page.value > 1) page.value--; await load() }
  catch (e) { feedback.toast.error(e.message) }
}
onMounted(load)
</script>

<template>
  <div class="wt-resource-page resource-visual-approved files-page">
    <ResourcePageHeader title="文件管理" description="上传图片和视频，获取可复用的文件链接">
      <template #actions><t-button theme="primary" @click="openUpload">上传文件</t-button></template>
    </ResourcePageHeader>
    <FeedbackAlert v-if="error" theme="error" :message="error" closable @close="error = ''" />
    <ResourceCard>
      <ResourceFilterBar>
        <label class="wt-filter-field wt-filter-field--wide"><span class="wt-filter-field__label">文件搜索</span><t-input v-model="filters.search" clearable placeholder="搜索文件名或 ID" @enter="query" /></label>
        <label class="wt-filter-field"><span class="wt-filter-field__label">文件类型</span><t-select v-model="filters.type" :options="[{label:'图片',value:'image'},{label:'视频',value:'video'}]" clearable placeholder="全部" /></label>
        <div class="wt-filter-actions"><t-button theme="primary" @click="query">查询</t-button><t-button variant="outline" @click="reset">重置</t-button></div>
      </ResourceFilterBar>
      <div class="table-scroll-wrap"><t-table class="wt-resource-table" :data="items" :columns="columns" row-key="id" :loading="loading" :scroll="tableScroll" size="small" hover empty="还没有上传文件">
        <template #type="{row}"><t-tag variant="light">{{ row.type === 'video' ? '视频' : '图片' }}</t-tag></template>
        <template #size_bytes="{row}">{{ sizeLabel(row.size_bytes) }}</template>
        <template #owner_name="{row}">{{ row.owner_name || `用户 ${row.owner_id}` }}</template>
        <template #team_name="{row}">{{ row.team_name || '—' }}</template>
        <template #created_at="{row}">{{ formatDateTime(row.created_at) }}</template>
        <template #op="{row}"><ResourceRowActions>
          <t-button tag="a" size="small" variant="text" :href="client.resourceURL(row)" target="_blank" rel="noopener noreferrer">预览</t-button>
          <t-button tag="a" size="small" variant="text" :href="`${client.resourceURL(row)}?download=1`" target="_blank" rel="noopener noreferrer">下载</t-button>
          <t-button size="small" variant="text" @click="copy(row)">复制链接</t-button>
          <t-button size="small" variant="text" @click="openRename(row)">改名</t-button>
          <t-button size="small" variant="text" class="is-danger" @click="remove(row)">删除</t-button>
        </ResourceRowActions></template>
      </t-table></div>
      <div class="pagination-bar"><t-pagination v-model:current="page" v-model:page-size="pageSize" :total="total" :show-jumper="false" :page-size-options="[10,20,50]" @current-change="load" @page-size-change="query" /></div>
    </ResourceCard>
    <ResourceOverlay v-model:visible="uploadVisible" title="上传文件" :busy="busy">
      <FeedbackAlert v-if="formError" theme="error" :message="formError" />
      <ResourceForm>
        <t-form-item label="选择文件" required>
          <div class="file-choice">
            <t-upload class="file-picker" theme="custom" draggable :auto-upload="false" :files="[]" :upload-button="null" :show-upload-progress="false" :use-mock-progress="false" :multiple="false" :disabled="busy" :accept="acceptedFiles" :input-attributes="{'aria-label':'选择图片或视频'}" @select-change="chooseFile">
              <template #dragContent>
                <div class="file-picker__content">
                  <CloudUploadIcon class="file-picker__icon" />
                  <t-button variant="text" theme="primary" :disabled="busy">{{ uploadFile ? '重新选择文件' : '点击选择文件' }}</t-button>
                  <span class="file-help">或拖拽图片、视频到这里</span>
                </div>
              </template>
            </t-upload>
            <div v-if="uploadFile" class="selected-file">
              <img v-if="previewURL" :src="previewURL" alt="所选图片预览" class="selected-file__preview" />
              <div v-else class="selected-file__preview selected-file__video"><VideoIcon /></div>
              <div class="selected-file__info"><strong :title="uploadFile.name">{{ uploadFile.name }}</strong><span>{{ sizeLabel(uploadFile.size) }}</span></div>
              <t-button variant="text" :disabled="busy" @click="clearSelection">移除</t-button>
            </div>
            <span class="file-help">支持 PNG、JPG、WEBP、GIF、MP4、MOV、WEBM、MKV<span v-if="uploadMaxBytes">，单个文件不超过 {{ sizeLabel(uploadMaxBytes) }}</span>。</span>
          </div>
        </t-form-item>
        <t-form-item label="文件名" required><t-input v-model="uploadName" :maxlength="80" :disabled="busy" placeholder="请填写文件名，可修改预填名称" /></t-form-item>
      </ResourceForm>
      <p v-if="busy" class="file-help" role="status">正在上传并校验文件，请稍候…</p>
      <template #footer><t-button variant="outline" :disabled="busy" @click="uploadVisible = false">取消</t-button><t-button theme="primary" :loading="busy" :disabled="!uploadFile || !uploadName.trim()" @click="upload">上传</t-button></template>
    </ResourceOverlay>
    <ResourceOverlay v-model:visible="renameVisible" title="修改文件名" :busy="busy">
      <FeedbackAlert v-if="formError" theme="error" :message="formError" />
      <ResourceForm><t-form-item label="文件名" required><t-input v-model="renameName" :maxlength="80" :disabled="busy" placeholder="请填写文件名" /></t-form-item></ResourceForm>
      <template #footer><t-button variant="outline" :disabled="busy" @click="renameVisible = false">取消</t-button><t-button theme="primary" :loading="busy" @click="rename">保存</t-button></template>
    </ResourceOverlay>
    <ResourceOverlay v-model:visible="linkVisible" title="文件链接" :footer="false"><p>请选择并复制以下链接：</p><t-textarea :value="linkValue" readonly /></ResourceOverlay>
  </div>
</template>

<style scoped>
.file-choice{display:flex;flex-direction:column;gap:12px;min-width:0;width:100%}
.file-help{color:var(--wt-text-tertiary);font-size:13px;line-height:1.6}
.file-picker{width:100%}
.file-picker__content{display:flex;flex-direction:column;align-items:center;gap:4px;padding:20px 12px}
.file-picker__icon{font-size:28px;color:var(--wt-primary);margin-bottom:4px}
.selected-file{display:flex;align-items:center;gap:12px;padding:12px;border:1px solid var(--wt-border);border-radius:var(--wt-radius-sm);min-width:0}
.selected-file__preview{width:56px;height:56px;flex:none;object-fit:contain;border-radius:var(--wt-radius-sm);background:var(--wt-bg-page)}
.selected-file__video{display:grid;place-items:center;font-size:28px;color:var(--wt-primary)}
.selected-file__info{display:flex;flex:1;min-width:0;flex-direction:column;gap:4px;font-size:13px}
.selected-file__info strong{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-weight:500}
.selected-file__info span{color:var(--wt-text-tertiary)}
</style>
