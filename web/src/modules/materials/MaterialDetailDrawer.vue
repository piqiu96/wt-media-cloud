<script setup>
// 素材详情抽屉，素材库与我的素材共用一份。行内只放识别信息（封面、ID、标题、文件状态），
// 其余在这里；云端视频地址不随素材 body 返回，就绪时才向详情链接接口要一次。
import { computed, ref, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { createMaterialsClient } from '../../shared/api/materials.js'
import { createFileTransferClient } from '../../shared/api/fileTransfer.js'
import { downloadedFileName } from '../transfer/downloadFacts.js'
import { revealSavedFile, savedFileStates } from '../transfer/desktopBridge.js'
import ResourceStatusBadge from '../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../shared/utils/datetime.js'
import { formatBytes } from '../../shared/utils/units.js'
import MaterialCover from './components/MaterialCover.vue'
import {
  downloadActionLabel,
  downloadStatusLabel,
  downloadStatusTone,
  gameName,
  materialStatusLabel,
  materialStatusTone,
  nextStepHint,
  shortDigest,
  usageFacts,
  usageStatusLabel,
  usageStatusTone,
  videoStatusLabel,
  videoStatusTone,
} from './labels.js'

const props = defineProps({
  visible: { type: Boolean, default: false },
  material: { type: Object, default: null },
  loading: { type: Boolean, default: false },
  games: { type: Array, default: () => [] },
  // 上下文决定动作：素材库给「加入我的素材」，我的素材这一组按关系与文件状态分支。
  mode: { type: String, default: 'library' },
  // 只在素材库上下文有意义：用它把主操作换成「去我的素材」。
  mine: { type: Boolean, default: false },
})
defineEmits(['update:visible', 'add', 'download', 'redownload', 'go-mine', 'give-up', 'restore', 'compose'])

const client = createMaterialsClient()
const transfer = createFileTransferClient()
const videoUrl = ref('')
// 本机那一份：扫描结果里的一条（`{ name, directory, presence, bytes }`），没量到就是
// `null`。名字也在里面 —— 「打开目录」按下去那一刻要把它送出去。
const localFile = ref(null)
// 这个运营在这条素材上的下载生命周期（downloading/downloaded/failed；空串=没下过）。
// 详情从它已拉的这张用户任务表里就地派生，不为此加接口。
const downloadStatus = ref('')

/**
 * 从这条用户的下载任务表里挑该素材**最新一次** user_download 的状态。
 *
 * `ListTasks` 是 created_at DESC, id DESC，第一条匹配即最新；与下载中心同一套过滤
 * （只看本机 + 这个动作），并映射成素材页同款三档。
 */
function deriveDownloadStatus(tasks, assetId) {
  const wanted = Number(assetId)
  if (!Number.isFinite(wanted)) return ''
  for (const task of Array.isArray(tasks) ? tasks : []) {
    if (task?.asset_type !== 'material') continue
    if (Number(task?.asset_id) !== wanted) continue
    if (task?.execution_scope !== 'local_agent') continue
    if (task?.purpose !== 'user_download') continue
    if (task?.status === 'pending' || task?.status === 'running') return 'downloading'
    if (task?.status === 'success') return 'downloaded'
    if (task?.status === 'failed' || task?.status === 'cancelled') return 'failed'
    return ''
  }
  return ''
}

/**
 * 文件状态：下载状态优先（有记录时），素材云侧 `video_status` 兜底。兜底里
 * `failed`（云准备失败）也画「下载失败」——用户裁定「准备失败和本地下载失败都
 * 属于下载失败，不需要分那么细」。与我的素材页同语义。
 */
const fileStatus = computed(() => {
  if (downloadStatus.value) {
    return { label: downloadStatusLabel(downloadStatus.value), tone: downloadStatusTone(downloadStatus.value) }
  }
  if (props.material?.video_status === 'failed') {
    return { label: '下载失败', tone: 'danger' }
  }
  return { label: videoStatusLabel(props.material?.video_status), tone: videoStatusTone(props.material?.video_status) }
})

// 与内容池页同一个量法：千分位。统计键恒在（库列 NOT NULL），0 是「采集时就是 0」。
function countLabel(value) { return Number(value || 0).toLocaleString() }

// 云端视频地址只在就绪时才去要：未就绪的素材没有地址，一次必然 409 的请求不该发出去。
watch(() => [props.visible, props.material?.id, props.material?.video_status], async ([open, id, status]) => {
  videoUrl.value = ''
  if (!open || !id || status !== 'ready') return
  try {
    const data = await client.getVideoUrl(id)
    videoUrl.value = data?.url || ''
  } catch {
    // 拿不到地址只少一个链接，不打断详情本身。
  }
})

/**
 * 这个素材在本机的那一份在哪个文件夹里。
 *
 * 两步，都不由前端拼：文件名从这张用户的下载任务表里挑（执行器报的那一个），目录由
 * Desktop 在所有已知保存位置里量出来。**「已下载」的判据在本机事实里，不在
 * `video_status` 上** —— 文件状态说的是云端那份源文件，一块本机从没下过的就绪素材
 * 也有它，而一份下好但云端还没就绪的文件照样躺在磁盘上。
 */
watch(() => [props.visible, props.material?.id, props.mode], async ([open, id, mode]) => {
  localFile.value = null
  downloadStatus.value = ''
  if (!open || !id || mode !== 'mine') return
  try {
    const tasks = await transfer.listTasks()
    downloadStatus.value = deriveDownloadStatus(tasks, id)
    const name = downloadedFileName(tasks, id)
    if (!name) return
    localFile.value = (await savedFileStates([name]))[name] ?? null
  } catch {
    // 读不到本机（浏览器、Agent 没起来）只少这一行：界面此时与没有这项功能时一样，
    // 不弹提示，也不把「读不到」写成「文件不在」。
  }
})

/**
 * 在文件管理器里打开那个文件夹。
 *
 * 发给 Rust 的是**名字**：目录由它在已知保存位置里找出来，前端从头到尾不知道路径。
 */
async function openDirectory() {
  try {
    await revealSavedFile(localFile.value.name)
  } catch (e) {
    // Tauri 的 invoke 拒绝时给的是字符串而不是 Error，两条路都要接住，
    // 否则界面上会出现一个空的错误提示。
    MessagePlugin.error(e?.message || String(e))
  }
}
</script>

<template>
  <t-drawer
    :visible="visible"
    class="material-detail-drawer"
    header="素材详情"
    size="min(62vw, 880px)"
    destroy-on-close
    :close-btn="true"
    @update:visible="$emit('update:visible', $event)"
  >
    <t-loading :loading="loading" :show-overlay="true">
      <div v-if="material" class="detail-workspace">
        <section class="detail-hero">
          <MaterialCover class="detail-cover" :url="material.cover_url" />
          <div class="detail-primary">
            <h3>
              <a v-if="material.source_url" class="wt-primary-link" :href="material.source_url" target="_blank" rel="noopener noreferrer">{{ material.title || '未命名素材' }}</a>
              <span v-else>{{ material.title || '未命名素材' }}</span>
            </h3>
            <!-- 副行只留平台 · 作者：游戏在列表里有列、在详情最后一段有行。 -->
            <p class="detail-source-line">
              <template v-if="material.platform">{{ material.platform }}</template>
              <template v-if="material.platform && (material.author_name || material.author_home_url)"> · </template>
              <a v-if="material.author_home_url" class="wt-primary-link" :href="material.author_home_url" target="_blank" rel="noopener noreferrer">{{ material.author_name || '作者主页' }}</a>
              <template v-else>{{ material.author_name }}</template>
            </p>
            <!-- 走查七轮：小型 Badge 组，不是横跨整页的状态色块（§7.1）。使用状态只在
                 读得到时出——素材库上下文打开的这个抽屉没有关系那一维，画一颗「使用中」
                 就是替服务端宣布一条它没说过的关系。 -->
            <div class="detail-badges">
              <ResourceStatusBadge v-if="material.usage_status" :tone="usageStatusTone(material.usage_status)" :label="usageStatusLabel(material.usage_status)" />
              <ResourceStatusBadge v-if="material.status" :tone="materialStatusTone(material.status)" :label="materialStatusLabel(material.status)" />
              <ResourceStatusBadge :tone="fileStatus.tone" :label="fileStatus.label" />
              <ResourceStatusBadge v-if="mine" tone="info" label="已加入我的素材" />
            </div>
          </div>
        </section>

        <!-- 首段回答「我现在处理到哪一步，下一步是什么」（走查七轮用户提示词）：两个状态
             维度 + 加入时间 + 一句下一步，使用情况的五个事实跟在下面（协议见 labels.js）。 -->
        <section class="detail-card">
          <h4 class="detail-card__title">当前进度 / 使用情况</h4>
          <dl class="detail-card__grid detail-card__grid--4">
            <div><dt>文件状态</dt><dd><ResourceStatusBadge :tone="fileStatus.tone" :label="fileStatus.label" /></dd></div>
            <div><dt>使用状态</dt><dd>
              <ResourceStatusBadge v-if="material.usage_status" :tone="usageStatusTone(material.usage_status)" :label="usageStatusLabel(material.usage_status)" />
              <template v-else>—</template>
            </dd></div>
            <div><dt>加入时间</dt><dd>{{ material.added_at ? formatDateTime(material.added_at) : '—' }}</dd></div>
            <div><dt>下一步</dt><dd>{{ nextStepHint(material) }}</dd></div>
          </dl>
          <div class="detail-usage">
            <div v-for="fact in usageFacts(material)" :key="fact.key" class="detail-usage__item">
              <ResourceStatusBadge v-if="fact.tone" :tone="fact.tone" :label="fact.value" />
              <strong v-else>{{ fact.value }}</strong>
              <span class="detail-usage__label">{{ fact.label }}</span>
            </div>
          </div>
        </section>

        <section class="detail-card">
          <h4 class="detail-card__title">
            文件信息
            <ResourceStatusBadge :tone="fileStatus.tone" :label="fileStatus.label" />
          </h4>
          <dl class="detail-card__grid detail-card__grid--4">
            <div><dt>文件大小</dt><dd>{{ formatBytes(material.video_size_bytes) }}</dd></div>
            <div><dt>准备完成于</dt><dd>{{ formatDateTime(material.video_prepared_at) }}</dd></div>
            <div><dt>校验值</dt><dd :title="material.video_sha256 || ''">{{ shortDigest(material.video_sha256) }}</dd></div>
            <div><dt>云端视频</dt><dd>
              <a v-if="videoUrl" class="wt-primary-link" :href="videoUrl" target="_blank" rel="noopener noreferrer">打开云端视频</a>
              <template v-else>{{ material.video_status === 'ready' ? '地址获取中' : '视频未就绪' }}</template>
            </dd></div>
          </dl>
          <!-- 本机那一份在哪个文件夹里。这一行说的是磁盘上的文件，与上面那条云端地址
               是两份东西：路径可能很长，所以不塞进四列网格，整行铺开、换行不截断。
               量不到（浏览器读不了本机、Desktop 还没扫到）时整行不出现 —— 与下载中心
               同一条约定：读不到本机不等于文件不在。 -->
          <div v-if="localFile?.directory" class="detail-local-file">
            <div class="detail-local-file__where">
              <span class="detail-local-file__label">下载目录</span>
              <span class="detail-local-file__path">{{ localFile.directory }}</span>
            </div>
            <t-button size="small" class="wt-secondary-button" variant="outline" @click="openDirectory">打开目录</t-button>
          </div>
        </section>
        <p v-if="material.last_error" class="detail-error">最近一次准备失败：{{ material.last_error }}</p>

        <section class="detail-card">
          <h4 class="detail-card__title">来源信息</h4>
          <dl class="detail-card__grid detail-card__grid--4">
            <div><dt>平台</dt><dd>{{ material.platform || '-' }}</dd></div>
            <div><dt>作者</dt><dd>
              <a v-if="material.author_home_url" class="wt-primary-link" :href="material.author_home_url" target="_blank" rel="noopener noreferrer">{{ material.author_name || '作者主页' }}</a>
              <template v-else>{{ material.author_name || '-' }}</template>
            </dd></div>
            <div><dt>发布时间</dt><dd>{{ formatDateTime(material.published_at) }}</dd></div>
            <div><dt>平台原视频</dt><dd>
              <a v-if="material.source_url" class="wt-primary-link" :href="material.source_url" target="_blank" rel="noopener noreferrer">打开原视频页面</a>
              <template v-else>-</template>
            </dd></div>
          </dl>
        </section>

        <!-- 来源行采集时的快照；抖音不给 play_count，只展示拿得到数的四项。 -->
        <section class="detail-card">
          <h4 class="detail-card__title">来源内容池统计</h4>
          <div class="detail-metrics">
            <div class="detail-metrics__item"><strong>{{ countLabel(material.like_count) }}</strong><span>点赞</span></div>
            <div class="detail-metrics__item"><strong>{{ countLabel(material.favorite_count) }}</strong><span>收藏</span></div>
            <div class="detail-metrics__item"><strong>{{ countLabel(material.comment_count) }}</strong><span>评论</span></div>
            <div class="detail-metrics__item"><strong>{{ countLabel(material.share_count) }}</strong><span>分享</span></div>
          </div>
        </section>

        <!-- 素材自身的三个字段放最后：走查七轮那份提示词按「先进度、后字段」排序，没有安排
             它们的位置；直接删掉会丢掉运营要复制去别处查的素材 ID。 -->
        <section class="detail-card">
          <h4 class="detail-card__title">基本信息</h4>
          <dl class="detail-card__grid detail-card__grid--3">
            <div><dt>素材 ID</dt><dd>{{ material.id }}</dd></div>
            <div><dt>游戏</dt><dd>{{ gameName(games, material.game_id) }}</dd></div>
            <div><dt>入库时间</dt><dd>{{ formatDateTime(material.created_at) }}</dd></div>
          </dl>
        </section>
      </div>
    </t-loading>

    <!-- 必须接管 footer：TDesign 的 footer 属性默认是 true，不给插槽就渲染出一对
         「取消 / 确认」；接管同时让动作条钉在底部，不随正文高度跳动。 -->
    <template #footer>
      <div v-if="material" class="material-detail__actions">
        <t-button v-if="mode === 'library' && !mine" theme="primary" @click="$emit('add', material)">加入我的素材</t-button>
        <t-button v-if="mode === 'library' && mine" theme="primary" @click="$emit('go-mine')">去我的素材</t-button>
        <!-- 我的素材这一组按两个维度分支（走查七轮用户提示词第十一节）：已放弃只给恢复，
             其余按文件状态给一步主操作；「重新下载」是就绪行的次级入口。 -->
        <template v-if="mode === 'mine'">
          <template v-if="material.usage_status === 'removed'">
            <t-button theme="primary" @click="$emit('restore', material)">恢复使用</t-button>
          </template>
          <template v-else>
            <t-button v-if="material.video_status === 'ready'" theme="primary" @click="$emit('compose', material)">加入合成</t-button>
            <t-button v-else-if="material.video_status !== 'downloading'" theme="primary" @click="$emit('download', material)">{{ downloadActionLabel(material.video_status) }}</t-button>
            <t-button v-if="material.video_status === 'ready'" class="wt-secondary-button" variant="outline" @click="$emit('redownload', material)">重新下载</t-button>
            <t-button class="wt-secondary-button wt-danger-button" variant="outline" @click="$emit('give-up', material)">放弃使用</t-button>
          </template>
        </template>
      </div>
    </template>
  </t-drawer>
</template>

<style scoped>
.detail-workspace { display: flex; flex-direction: column; gap: 14px; }
.detail-hero { display: grid; grid-template-columns: 160px minmax(0, 1fr); gap: 16px; align-items: flex-start; }
.detail-cover { width: 160px; height: 100px; border-radius: 10px; }
.detail-primary { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.detail-primary h3 { margin: 0; color: var(--wt-text-primary); font-size: 18px; font-weight: 650; line-height: 1.35; overflow-wrap: anywhere; }
.detail-source-line { margin: 0; color: var(--wt-text-tertiary); font-size: 12px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.detail-badges { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
/* 卡片间距由 .detail-workspace 的 gap 给；这里不要再写 + 选择器的 margin。 */
.detail-card { padding: 14px 16px; border: 1px solid var(--wt-border); border-radius: 10px; background: var(--wt-bg-card); }
.detail-card__title { display: flex; align-items: center; gap: 8px; margin: 0 0 12px; color: var(--wt-text-primary); font-size: 15px; font-weight: 650; }
.detail-card__grid { display: grid; gap: 12px 16px; margin: 0; }
.detail-card__grid--3 { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.detail-card__grid--4 { grid-template-columns: repeat(4, minmax(0, 1fr)); }
.detail-card__grid dt { color: var(--wt-text-tertiary); font-size: 12px; }
.detail-card__grid dd { margin: 5px 0 0; color: var(--wt-text-primary); font-size: 14px; line-height: 1.5; overflow-wrap: anywhere; }
/* 使用情况的五个事实与上面那行进度事实同卡不同组：一条虚线分开，不另起一张卡。 */
.detail-usage { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 12px; margin-top: 14px; padding-top: 14px; border-top: 1px dashed var(--wt-border); }
.detail-usage__item { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.detail-usage__item strong { color: var(--wt-text-primary); font-size: 15px; font-weight: 600; }
/* 标签挂类名而不是后代选择器：ResourceStatusBadge 的根元素也是 span，会被后代选择器染色。 */
.detail-usage__label { color: var(--wt-text-tertiary); font-size: 12px; }
.detail-metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.detail-metrics__item { display: flex; flex-direction: column; gap: 6px; }
.detail-metrics__item strong { color: var(--wt-text-primary); font-size: 15px; font-weight: 600; }
.detail-metrics__item span { color: var(--wt-text-tertiary); font-size: 12px; }
.detail-error { margin: 12px 0 0; color: var(--wt-danger); font-size: 13px; line-height: 1.5; }
/* 整行铺开而不是塞进四列网格：目录可能很长，截断掉的恰好是「在哪个盘的哪个文件夹」。
   与上面那行网格用一条虚线分开，同 `.detail-usage` 的做法。 */
.detail-local-file { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 14px; padding-top: 14px; border-top: 1px dashed var(--wt-border); }
.detail-local-file__where { display: flex; align-items: baseline; gap: 10px; min-width: 0; }
.detail-local-file__label { flex: 0 0 auto; color: var(--wt-text-tertiary); font-size: 12px; }
.detail-local-file__path { color: var(--wt-text-primary); font-size: 13px; line-height: 1.5; word-break: break-all; }
/* 页脚只放业务动作、靠右收：关闭走抽屉右上角的 ×（规范 §6.3），不再占页脚一格。 */
.material-detail__actions { display: flex; justify-content: flex-end; align-items: center; gap: 8px; }
.wt-primary-link { color: var(--wt-primary); font-weight: 600; text-decoration: none; }
.wt-primary-link:hover, .wt-primary-link:focus-visible { text-decoration: underline; }
</style>
