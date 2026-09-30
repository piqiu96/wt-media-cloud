<script setup>
// 素材详情抽屉，素材库与我的素材共用一份。
//
// 行内只放识别信息（封面、ID、标题、状态）；作者主页、平台原视频页、体积和云端视频
// 地址都在这里（CHG-20260930-069）。云端地址不随素材 body 返回，就绪时才向详情
// 链接接口要一次。
import { ref, watch } from 'vue'
import { createMaterialsClient } from '../../shared/api/materials.js'
import ResourceStatusBadge from '../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../shared/utils/datetime.js'
import { formatBytes } from '../../shared/utils/units.js'
import MaterialCover from './components/MaterialCover.vue'
import { gameName, shortDigest, videoStatusLabel, videoStatusTone } from './labels.js'

const props = defineProps({
  visible: { type: Boolean, default: false },
  material: { type: Object, default: null },
  loading: { type: Boolean, default: false },
  games: { type: Array, default: () => [] },
})
defineEmits(['update:visible', 'add', 'download'])

const client = createMaterialsClient()
const videoUrl = ref('')

// 与内容池页同一个量法：千分位。统计键恒在（库列 NOT NULL），0 是「采集时就是 0」。
function countLabel(value) { return Number(value || 0).toLocaleString() }

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
            <ResourceStatusBadge :tone="videoStatusTone(material.video_status)" :label="videoStatusLabel(material.video_status)" />
          </div>
        </div>
        <dl>
          <div><dt>素材 ID</dt><dd>#{{ material.id }}</dd></div>
          <div><dt>平台</dt><dd>{{ material.platform || '-' }}</dd></div>
          <div><dt>游戏</dt><dd>{{ gameName(games, material.game_id) }}</dd></div>
          <div><dt>作者</dt><dd>
            <a v-if="material.author_home_url" class="wt-primary-link" :href="material.author_home_url" target="_blank" rel="noopener noreferrer">{{ material.author_name || '作者主页' }}</a>
            <template v-else>{{ material.author_name || '-' }}</template>
          </dd></div>
          <div><dt>发布时间</dt><dd>{{ formatDateTime(material.published_at) }}</dd></div>
          <div><dt>入库时间</dt><dd>{{ formatDateTime(material.created_at) }}</dd></div>
          <div><dt>体积</dt><dd>{{ formatBytes(material.video_size_bytes) }}</dd></div>
          <div><dt>校验值</dt><dd :title="material.video_sha256 || ''">{{ shortDigest(material.video_sha256) }}</dd></div>
          <div><dt>准备完成于</dt><dd>{{ formatDateTime(material.video_prepared_at) }}</dd></div>
          <div><dt>平台原视频</dt><dd>
            <a v-if="material.source_url" class="wt-primary-link" :href="material.source_url" target="_blank" rel="noopener noreferrer">打开原视频页面</a>
            <template v-else>-</template>
          </dd></div>
          <div><dt>云端视频</dt><dd>
            <a v-if="videoUrl" class="wt-primary-link" :href="videoUrl" target="_blank" rel="noopener noreferrer">打开云端视频</a>
            <template v-else>{{ material.video_status === 'ready' ? '地址获取中' : '视频未就绪' }}</template>
          </dd></div>
        </dl>
        <!-- 走查反馈（CHG-20260930-069）：来源行采集时的内容池统计以独立区块展现。
             它们是采集时刻的快照，不是素材自己的属性，混进上面的 dl 会把「入库时间」
             和「72313 个赞」读成同一类事实。 -->
        <section class="material-detail__stats">
          <h4>来源内容池统计</h4>
          <div class="material-detail__stats-grid">
            <div><span>播放</span><strong>{{ countLabel(material.view_count) }}</strong></div>
            <div><span>点赞</span><strong>{{ countLabel(material.like_count) }}</strong></div>
            <div><span>收藏</span><strong>{{ countLabel(material.favorite_count) }}</strong></div>
            <div><span>评论</span><strong>{{ countLabel(material.comment_count) }}</strong></div>
            <div><span>分享</span><strong>{{ countLabel(material.share_count) }}</strong></div>
          </div>
        </section>
        <p v-if="material.last_error" class="material-detail__error">最近一次准备失败：{{ material.last_error }}</p>
        <div class="material-detail__actions">
          <t-button class="wt-secondary-button" variant="outline" @click="$emit('add', material)">加入我的素材</t-button>
          <t-button theme="primary" @click="$emit('download', material)">下载</t-button>
        </div>
      </div>
    </t-loading>
  </t-drawer>
</template>

<style scoped>
.material-detail__head { display: flex; gap: 14px; align-items: flex-start; }
.material-detail__cover { width: 120px; height: 78px; }
.material-detail__heading { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.material-detail h3 { margin: 0; color: var(--wt-text-primary); font-size: 18px; line-height: 1.4; overflow-wrap: anywhere; }
.material-detail dl { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px 16px; margin: 18px 0 0; }
.material-detail dt { color: var(--wt-text-tertiary); font-size: 12px; }
.material-detail dd { margin: 4px 0 0; color: var(--wt-text-primary); font-size: 14px; overflow-wrap: anywhere; }
.material-detail__error { margin: 16px 0 0; color: var(--wt-danger); font-size: 13px; line-height: 1.5; }
/* `align-items: center`：与列表行同一句话——两颗按钮等高并排，谁也不像另一类动作。 */
.material-detail__actions { display: flex; align-items: center; gap: 8px; margin-top: 20px; }
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
