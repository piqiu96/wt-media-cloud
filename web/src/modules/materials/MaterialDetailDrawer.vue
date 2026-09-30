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
import { ref, watch } from 'vue'
import { createMaterialsClient } from '../../shared/api/materials.js'
import ResourceStatusBadge from '../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../shared/utils/datetime.js'
import { formatBytes } from '../../shared/utils/units.js'
import MaterialCover from './components/MaterialCover.vue'
import { downloadActionLabel, gameName, shortDigest, videoStatusLabel, videoStatusTone } from './labels.js'

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
    size="min(52vw, 720px)"
    destroy-on-close
    @update:visible="$emit('update:visible', $event)"
  >
    <t-loading :loading="loading" :show-overlay="true">
      <div v-if="material" class="material-detail">
        <div class="material-detail__head">
          <MaterialCover class="material-detail__cover" :url="material.cover_url" />
          <div class="material-detail__heading">
            <h3>{{ material.title || '未命名素材' }}</h3>
            <!-- 副行先说清「这是什么」：游戏 · 平台 · 作者。正文负责解释细节。 -->
            <p class="material-detail__source-line">{{ [gameName(games, material.game_id), material.platform, material.author_name].filter(Boolean).join(' · ') }}</p>
            <div class="material-detail__badges">
              <ResourceStatusBadge :tone="videoStatusTone(material.video_status)" :label="videoStatusLabel(material.video_status)" />
              <!-- 使用状态维度：加入状态与文件状态是两件事，可以同时是「已加入 + 准备失败」。 -->
              <ResourceStatusBadge v-if="mine" tone="info" label="已加入我的素材" />
            </div>
          </div>
        </div>

        <t-tabs v-model="activeTab" class="material-detail__tabs">
          <t-tab-panel value="overview" label="概览">
            <section class="material-detail__section">
              <h4>基本信息</h4>
              <dl>
                <div><dt>素材 ID</dt><dd>{{ material.id }}</dd></div>
                <div><dt>游戏</dt><dd>{{ gameName(games, material.game_id) }}</dd></div>
                <div><dt>平台</dt><dd>{{ material.platform || '-' }}</dd></div>
                <div><dt>发布时间</dt><dd>{{ formatDateTime(material.published_at) }}</dd></div>
                <div><dt>入库时间</dt><dd>{{ formatDateTime(material.created_at) }}</dd></div>
              </dl>
            </section>
            <!-- 走查反馈（CHG-20260930-069）：来源行采集时的内容池统计以独立区块展现。
                 它们是采集时刻的快照，不是素材自己的属性，混进上面的 dl 会把「入库时间」
                 和「72313 个赞」读成同一类事实。抖音接口不给 play_count（恒为 0，全部来源
                 行核对过），区块只展示拿得到数的四项，不为一个永远的 0 留位置。 -->
            <section class="material-detail__stats">
              <h4>来源内容池统计</h4>
              <div class="material-detail__stats-grid">
                <div><span>点赞</span><strong>{{ countLabel(material.like_count) }}</strong></div>
                <div><span>收藏</span><strong>{{ countLabel(material.favorite_count) }}</strong></div>
                <div><span>评论</span><strong>{{ countLabel(material.comment_count) }}</strong></div>
                <div><span>分享</span><strong>{{ countLabel(material.share_count) }}</strong></div>
              </div>
            </section>
          </t-tab-panel>

          <t-tab-panel value="file" label="文件信息">
            <section class="material-detail__section">
              <dl>
                <div><dt>文件状态</dt><dd><ResourceStatusBadge :tone="videoStatusTone(material.video_status)" :label="videoStatusLabel(material.video_status)" /></dd></div>
                <div><dt>文件大小</dt><dd>{{ formatBytes(material.video_size_bytes) }}</dd></div>
                <div><dt>准备完成于</dt><dd>{{ formatDateTime(material.video_prepared_at) }}</dd></div>
                <div><dt>校验值</dt><dd :title="material.video_sha256 || ''">{{ shortDigest(material.video_sha256) }}</dd></div>
                <div><dt>云端视频</dt><dd>
                  <a v-if="videoUrl" class="wt-primary-link" :href="videoUrl" target="_blank" rel="noopener noreferrer">打开云端视频</a>
                  <template v-else>{{ material.video_status === 'ready' ? '地址获取中' : '视频未就绪' }}</template>
                </dd></div>
              </dl>
            </section>
            <p v-if="material.last_error" class="material-detail__error">最近一次准备失败：{{ material.last_error }}</p>
          </t-tab-panel>

          <t-tab-panel value="source" label="来源信息">
            <section class="material-detail__section">
              <dl>
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
          </t-tab-panel>
        </t-tabs>

        <!-- 规范 §6.3：详情不出现无意义的「确认 / 取消」，关闭走统一的关闭动作；
             当前状态允许的业务动作保留在它旁边。 -->
        <div class="material-detail__actions">
          <t-button class="wt-secondary-button" variant="outline" @click="$emit('update:visible', false)">关闭</t-button>
          <t-button v-if="mode === 'library' && !mine" theme="primary" @click="$emit('add', material)">加入我的素材</t-button>
          <t-button v-if="mode === 'library' && mine" theme="primary" @click="$emit('go-mine')">去我的素材</t-button>
          <t-button v-if="mode === 'mine'" theme="primary" @click="$emit('download', material)">{{ downloadActionLabel(material.video_status) }}</t-button>
        </div>
      </div>
    </t-loading>
  </t-drawer>
</template>

<style scoped>
.material-detail__head { display: flex; gap: 14px; align-items: flex-start; }
.material-detail__cover { width: 120px; height: 78px; }
.material-detail__heading { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.material-detail h3 { margin: 0; color: var(--wt-text-primary); font-size: 18px; line-height: 1.4; overflow-wrap: anywhere; }
/* 副行是识别信息，不是重点：一行、次要色、放不下就省略。 */
.material-detail__source-line { margin: 0; color: var(--wt-text-tertiary); font-size: 12px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.material-detail__badges { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.material-detail__tabs { margin-top: 18px; }
.material-detail__section { padding-top: 4px; }
.material-detail__section h4 { margin: 0 0 10px; color: var(--wt-text-secondary); font-size: 13px; font-weight: 500; }
.material-detail dl { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px 16px; margin: 0; }
.material-detail dt { color: var(--wt-text-tertiary); font-size: 12px; }
.material-detail dd { margin: 4px 0 0; color: var(--wt-text-primary); font-size: 14px; overflow-wrap: anywhere; }
.material-detail__error { margin: 16px 0 0; color: var(--wt-danger); font-size: 13px; line-height: 1.5; }
/* 单一主操作（交互对齐 §2.3）：上下文给哪颗就渲染哪颗，没有并排的第二个业务动作。
   「关闭」是导航动作，不算业务动作。 */
.material-detail__actions { display: flex; align-items: center; gap: 8px; margin-top: 24px; }
.material-detail__stats { margin-top: 20px; padding: 14px 16px; border: 1px solid var(--wt-border); border-radius: 8px; background: var(--wt-bg-page); }
.material-detail__stats h4 { margin: 0 0 10px; color: var(--wt-text-secondary); font-size: 13px; font-weight: 500; }
.material-detail__stats-grid { display: flex; flex-wrap: wrap; gap: 8px 28px; }
.material-detail__stats-grid div { display: flex; align-items: baseline; gap: 6px; }
.material-detail__stats-grid span { color: var(--wt-text-tertiary); font-size: 12px; }
.material-detail__stats-grid strong { color: var(--wt-text-primary); font-size: 14px; }
/* 抽屉里的链接此前借用 ContentPoolPage 的类名却没有那份样式；这里补上同一份定义，
   「打开作者主页／原视频／云端视频」与标题链接才是同一个蓝色。 */
.wt-primary-link { color: var(--wt-primary); font-weight: 600; text-decoration: none; }
.wt-primary-link:hover, .wt-primary-link:focus-visible { text-decoration: underline; }
</style>
