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
import { downloadHint, gameName, shortDigest, videoStatusLabel, videoStatusTone } from './labels.js'

const props = defineProps({
  visible: { type: Boolean, default: false },
  material: { type: Object, default: null },
  loading: { type: Boolean, default: false },
  games: { type: Array, default: () => [] },
})
defineEmits(['update:visible', 'add', 'download'])

const client = createMaterialsClient()
const videoUrl = ref('')

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
        <p v-if="material.last_error" class="material-detail__error">最近一次准备失败：{{ material.last_error }}</p>
        <div class="material-detail__actions">
          <t-button class="wt-secondary-button" variant="outline" @click="$emit('add', material)">加入我的素材</t-button>
          <div class="wt-row-action">
            <t-button theme="primary" @click="$emit('download', material)">下载到本机</t-button>
            <small v-if="downloadHint(material)" class="wt-row-hint">{{ downloadHint(material) }}</small>
          </div>
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
/* `align-items: center`：下载那颗按钮下面可能挂着一句提示，于是它比旁边那颗高。
   不居中就会让「加入我的素材」被拉成两行高，看起来像是另一类动作。 */
.material-detail__actions { display: flex; align-items: center; gap: 8px; margin-top: 20px; }
</style>
