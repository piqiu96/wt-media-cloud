<script setup>
// 素材详情抽屉，素材库与我的素材共用一份。行内只放识别信息（封面、ID、标题、文件状态），
// 其余在这里；云端视频地址不随素材 body 返回，就绪时才向详情链接接口要一次。
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
  // 上下文决定动作：素材库给「加入我的素材」，我的素材给下载。
  mode: { type: String, default: 'library' },
  // 只在素材库上下文有意义：用它把主操作换成「去我的素材」。
  mine: { type: Boolean, default: false },
})
defineEmits(['update:visible', 'add', 'download', 'go-mine'])

const client = createMaterialsClient()
const videoUrl = ref('')

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
        <section class="detail-hero">
          <MaterialCover class="detail-cover" :url="material.cover_url" />
          <div class="detail-primary">
            <h3>
              <a v-if="material.source_url" class="wt-primary-link" :href="material.source_url" target="_blank" rel="noopener noreferrer">{{ material.title || '未命名素材' }}</a>
              <span v-else>{{ material.title || '未命名素材' }}</span>
            </h3>
            <!-- 副行只留平台 · 作者：游戏在列表里有列、在基本信息里有行。 -->
            <p class="detail-source-line">
              <template v-if="material.platform">{{ material.platform }}</template>
              <template v-if="material.platform && (material.author_name || material.author_home_url)"> · </template>
              <a v-if="material.author_home_url" class="wt-primary-link" :href="material.author_home_url" target="_blank" rel="noopener noreferrer">{{ material.author_name || '作者主页' }}</a>
              <template v-else>{{ material.author_name }}</template>
            </p>
            <div class="detail-badges">
              <ResourceStatusBadge v-if="material.status" :tone="materialStatusTone(material.status)" :label="materialStatusLabel(material.status)" />
              <ResourceStatusBadge :tone="videoStatusTone(material.video_status)" :label="videoStatusLabel(material.video_status)" />
              <ResourceStatusBadge v-if="mine" tone="info" label="已加入我的素材" />
            </div>
          </div>
        </section>

        <!-- 使用情况服务端尚未返回，这里先立骨架。 -->
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
      </div>
    </t-loading>

    <!-- 必须接管 footer：TDesign 的 footer 属性默认是 true，不给插槽就渲染出一对
         「取消 / 确认」；接管同时让动作条钉在底部，不随正文高度跳动。 -->
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
.detail-usage { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 12px; }
.detail-usage__item { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.detail-usage__item strong { color: var(--wt-text-primary); font-size: 15px; font-weight: 600; }
/* 标签挂类名而不是后代选择器：ResourceStatusBadge 的根元素也是 span，会被后代选择器染色。 */
.detail-usage__label { color: var(--wt-text-tertiary); font-size: 12px; }
.detail-metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.detail-metrics__item { display: flex; flex-direction: column; gap: 6px; }
.detail-metrics__item strong { color: var(--wt-text-primary); font-size: 15px; font-weight: 600; }
.detail-metrics__item span { color: var(--wt-text-tertiary); font-size: 12px; }
.detail-error { margin: 12px 0 0; color: var(--wt-danger); font-size: 13px; line-height: 1.5; }
/* 左「关闭」右主操作：「关闭」是导航动作，与业务动作分开摆。 */
.material-detail__actions { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.material-detail__primary { display: flex; gap: 8px; }
.wt-primary-link { color: var(--wt-primary); font-weight: 600; text-decoration: none; }
.wt-primary-link:hover, .wt-primary-link:focus-visible { text-decoration: underline; }
</style>
