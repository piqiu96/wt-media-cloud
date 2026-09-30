<script setup>
// 素材详情抽屉，素材库与我的素材共用一份。
//
// 行内只放识别信息（封面、ID、标题、文件状态）；作者主页、平台原视频页、体积和云端视频
// 地址都在这里（CHG-20260930-069）。云端地址不随素材 body 返回，就绪时才向详情
// 链接接口要一次。
//
// 走查四轮（2026-09-30）：正文按「概览 / 文件信息 / 来源信息」分标签。此前十几个字段
// 平铺在同一张 dl 里，结果是「入库时间」和「72313 个赞」读成同一类事实、文件事实与
// 来源事实混在一起 —— 分标签让每一屏只回答一个问题。
//
// 走查五轮：标签里的每一段信息再各自成为一张有边界的卡（.detail-card），照内容池详情的
// 框来做；顶部合成 hero（封面 + 标题 + 来源副行 + 两个状态维度）。互动数据从概览挪到
// 来源信息——它是来源行的快照，跟着来源走。
import { ref, watch } from 'vue'
import { createMaterialsClient } from '../../shared/api/materials.js'
import ResourceStatusBadge from '../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../shared/utils/datetime.js'
import { formatBytes } from '../../shared/utils/units.js'
import MaterialCover from './components/MaterialCover.vue'
import {
  downloadActionLabel,
  gameName,
  materialStatusLabel,
  materialStatusTone,
  shortDigest,
  usageFacts,
  videoStatusLabel,
  videoStatusTone,
} from './labels.js'

const props = defineProps({
  visible: { type: Boolean, default: false },
  material: { type: Object, default: null },
  loading: { type: Boolean, default: false },
  games: { type: Array, default: () => [] },
  // 上下文决定动作（走查三轮，交互对齐）：素材库上下文的主操作是「加入我的素材」，
  // 不提供下载——下载归「我的素材」；我的素材上下文相反。
  mode: { type: String, default: 'library' },
  // 这条素材是否已经在「我的素材」里。只在素材库上下文有意义：那里要拿它把主操作
  // 换成「去我的素材」，并说明为什么不再给一次「加入」。
  mine: { type: Boolean, default: false },
})
defineEmits(['update:visible', 'add', 'download', 'go-mine'])

const client = createMaterialsClient()
const videoUrl = ref('')
const activeTab = ref('overview')

// 与内容池页同一个量法：千分位。统计键恒在（库列 NOT NULL），0 是「采集时就是 0」。
function countLabel(value) { return Number(value || 0).toLocaleString() }

// 顶部副行（游戏 · 平台 · 作者）在模板里由 filter(Boolean).join 拼出：缺哪项就少哪项，
// 不留孤零零的分隔符。它不值得一个 ref —— 每次 render 重算一遍比缓存一份更不容易腐坏。
watch(() => [props.visible, props.material?.id, props.material?.video_status], async ([open, id, status]) => {
  videoUrl.value = ''
  activeTab.value = 'overview'
  if (!open || !id || status !== 'ready') return
  try {
    const data = await client.getVideoUrl(id)
    videoUrl.value = data?.url || ''
  } catch {
    // 拿不到地址只少一个链接，不打断详情本身。
  }
})
</script>

<template>
  <t-drawer
    :visible="visible"
    class="material-detail-drawer"
    header="素材详情"
    size="min(62vw, 880px)"
    destroy-on-close
    @update:visible="$emit('update:visible', $event)"
  >
    <t-loading :loading="loading" :show-overlay="true">
      <div v-if="material" class="detail-workspace">
        <!-- 顶部一次说清「这是什么」：封面、标题、来源副行、两个状态维度。
             副行按设计图是「游戏 · 平台 · 作者」，缺哪项少哪项。 -->
        <section class="detail-hero">
          <MaterialCover class="detail-cover" :url="material.cover_url" />
          <div class="detail-primary">
            <h3>{{ material.title || '未命名素材' }}</h3>
            <p class="detail-source-line">{{ [gameName(games, material.game_id), material.platform, material.author_name].filter(Boolean).join(' · ') }}</p>
            <div class="detail-badges">
              <!-- 规范 §7.2：两个状态维度并排，不合并。素材状态服务端还没返回，
                   读不到就不画 —— 画一个默认的「可用」等于替服务端做了判断。 -->
              <ResourceStatusBadge v-if="material.status" :tone="materialStatusTone(material.status)" :label="materialStatusLabel(material.status)" />
              <ResourceStatusBadge :tone="videoStatusTone(material.video_status)" :label="videoStatusLabel(material.video_status)" />
              <!-- 使用状态维度：加入状态与文件状态是两件事，可以同时是「已加入 + 准备失败」。 -->
              <ResourceStatusBadge v-if="mine" tone="info" label="已加入我的素材" />
            </div>
          </div>
        </section>

        <t-tabs v-model="activeTab" class="detail-tabs">
          <t-tab-panel value="overview" label="概览">
            <!-- 使用情况的数据服务端尚未返回（change.md §3），这里先立设计图里的
                 五格骨架：数据打通后直接填，不用再动布局。 -->
            <section class="detail-card">
              <h4 class="detail-card__title">使用情况</h4>
              <div class="detail-usage">
                <div v-for="fact in usageFacts(material)" :key="fact.key" class="detail-usage__item">
                  <ResourceStatusBadge v-if="fact.tone" :tone="fact.tone" :label="fact.value" />
                  <strong v-else>{{ fact.value }}</strong>
                  <span class="detail-usage__label">{{ fact.label }}</span>
                </div>
              </div>
            </section>

            <section class="detail-card">
              <h4 class="detail-card__title">基本信息</h4>
              <dl class="detail-card__grid detail-card__grid--3">
                <div><dt>素材 ID</dt><dd>{{ material.id }}</dd></div>
                <div><dt>游戏</dt><dd>{{ gameName(games, material.game_id) }}</dd></div>
                <div><dt>入库时间</dt><dd>{{ formatDateTime(material.created_at) }}</dd></div>
              </dl>
            </section>
          </t-tab-panel>

          <t-tab-panel value="file" label="文件信息">
            <!-- 文件状态从字段挪进了卡片标题：它说的是这一整块信息成不成，不是其中一行。 -->
            <section class="detail-card">
              <h4 class="detail-card__title">
                文件信息
                <ResourceStatusBadge :tone="videoStatusTone(material.video_status)" :label="videoStatusLabel(material.video_status)" />
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
            </section>
            <p v-if="material.last_error" class="detail-error">最近一次准备失败：{{ material.last_error }}</p>
          </t-tab-panel>

          <t-tab-panel value="source" label="来源信息">
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

            <!-- 互动数据是来源行采集时的快照，跟着「来源」走：走查五轮按设计图从概览
                 挪到这里。混进基本信息会把「入库时间」和「72313 个赞」读成同一类事实。
                 抖音接口不给 play_count（恒为 0，全部来源行核对过），只展示拿得到数的四项。 -->
            <section class="detail-card">
              <h4 class="detail-card__title">来源内容池统计</h4>
              <div class="detail-metrics">
                <div class="detail-metrics__item"><strong>{{ countLabel(material.like_count) }}</strong><span>点赞</span></div>
                <div class="detail-metrics__item"><strong>{{ countLabel(material.favorite_count) }}</strong><span>收藏</span></div>
                <div class="detail-metrics__item"><strong>{{ countLabel(material.comment_count) }}</strong><span>评论</span></div>
                <div class="detail-metrics__item"><strong>{{ countLabel(material.share_count) }}</strong><span>分享</span></div>
              </div>
            </section>
          </t-tab-panel>
        </t-tabs>
      </div>
    </t-loading>

    <!-- 走查五轮：动作条接管 footer 插槽，做两件事。
         1）消掉 TDesign 的默认页脚。Drawer 的 footer 属性默认是 true，不给插槽就渲染
            getDefaultFooter() 的「取消 / 确认」——规范 §6.3 明令详情不该有它。全站另
            8 个抽屉都显式关掉了（`:footer="false"` 或给自己的 #footer），只有本抽屉漏了。
         2）让它钉在抽屉底部。原先它在正文末尾，切标签时正文高度一变，按钮就跟着上下跳。
         上下文只给一颗主操作（§2.3），「关闭」是导航动作、不算业务动作，随它留在同一行。 -->
    <template #footer>
      <div v-if="material" class="material-detail__actions">
        <t-button class="wt-secondary-button" variant="outline" @click="$emit('update:visible', false)">关闭</t-button>
        <div class="material-detail__primary">
          <t-button v-if="mode === 'library' && !mine" theme="primary" @click="$emit('add', material)">加入我的素材</t-button>
          <t-button v-if="mode === 'library' && mine" theme="primary" @click="$emit('go-mine')">去我的素材</t-button>
          <t-button v-if="mode === 'mine'" theme="primary" @click="$emit('download', material)">{{ downloadActionLabel(material.video_status) }}</t-button>
        </div>
      </div>
    </template>
  </t-drawer>
</template>

<style scoped>
/* 走查五轮（2026-09-30 用户走查）：「详情里的展示框效果远不如预期」，用户给的参照是
   内容池详情的框。于是每一段信息都是一张有边界的卡（.detail-card），不再是十几行
   dt/dd 平铺下来的长条——平铺的结局是「入库时间」和「72313 个赞」读成同一类事实。 */
.detail-workspace { display: flex; flex-direction: column; gap: 14px; }
.detail-hero { display: grid; grid-template-columns: 160px minmax(0, 1fr); gap: 16px; align-items: flex-start; }
.detail-cover { width: 160px; height: 100px; border-radius: 10px; }
.detail-primary { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.detail-primary h3 { margin: 0; color: var(--wt-text-primary); font-size: 18px; font-weight: 650; line-height: 1.35; overflow-wrap: anywhere; }
/* 副行是识别信息，不是重点：一行、次要色、放不下就省略。 */
.detail-source-line { margin: 0; color: var(--wt-text-tertiary); font-size: 12px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.detail-badges { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.detail-tabs { margin-top: 16px; }
.detail-card { padding: 14px 16px; border: 1px solid var(--wt-border); border-radius: 10px; background: var(--wt-bg-card); }
.detail-card + .detail-card { margin-top: 12px; }
/* 标题与状态徽章同行：文件状态说的是这一整块成不成，不是其中一行。 */
.detail-card__title { display: flex; align-items: center; gap: 8px; margin: 0 0 12px; color: var(--wt-text-primary); font-size: 15px; font-weight: 650; }
.detail-card__grid { display: grid; gap: 12px 16px; margin: 0; }
.detail-card__grid--3 { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.detail-card__grid--4 { grid-template-columns: repeat(4, minmax(0, 1fr)); }
.detail-card__grid dt { color: var(--wt-text-tertiary); font-size: 12px; }
.detail-card__grid dd { margin: 5px 0 0; color: var(--wt-text-primary); font-size: 14px; line-height: 1.5; overflow-wrap: anywhere; }
.detail-usage { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 12px; }
.detail-usage__item { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.detail-usage__item strong { color: var(--wt-text-primary); font-size: 15px; font-weight: 600; }
/* 标签挂类名而不是 `.detail-usage__item > span`：重复风险那格的第一行是
   ResourceStatusBadge，它的根元素也是 span，且会带上父组件的 scoped 属性——
   用后代选择器写会把徽章一起染成次要色，绿色「正常」变成灰的。 */
.detail-usage__label { color: var(--wt-text-tertiary); font-size: 12px; }
.detail-metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.detail-metrics__item { display: flex; flex-direction: column; gap: 6px; }
.detail-metrics__item strong { color: var(--wt-text-primary); font-size: 15px; font-weight: 600; }
.detail-metrics__item span { color: var(--wt-text-tertiary); font-size: 12px; }
.detail-error { margin: 12px 0 0; color: var(--wt-danger); font-size: 13px; line-height: 1.5; }
/* 设计图：页脚左边「关闭」、右边上下文主操作。单一主操作（交互对齐 §2.3）说的是
   同一时刻只有一颗业务动作，不是「所有按钮都挤右边」——「关闭」是导航动作，
   把它与主操作分开是让两者一眼可辨。页脚自带内边距，这里不再补 margin。 */
.material-detail__actions { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.material-detail__primary { display: flex; gap: 8px; }
/* 抽屉里的链接此前借用 ContentPoolPage 的类名却没有那份样式；这里补上同一份定义，
   「打开作者主页／原视频／云端视频」与标题链接才是同一个蓝色。 */
.wt-primary-link { color: var(--wt-primary); font-weight: 600; text-decoration: none; }
.wt-primary-link:hover, .wt-primary-link:focus-visible { text-decoration: underline; }
</style>
