<script setup>
// 素材详情抽屉，素材库与我的素材共用一份。
//
// 两个页面都要它，而它显示的是**同一份冻结的素材体**（content-production.yaml 的
// Material）。各写一份的结局是两份慢慢分叉：加一个字段时只改了一处，于是同一个素材
// 在两个页面上显示得不一样，且没有任何东西会报错。
import ResourceStatusBadge from '../../shared/ui/resource/ResourceStatusBadge.vue'
import { formatDateTime } from '../../shared/utils/datetime.js'
import { formatBytes } from '../../shared/utils/units.js'
import { downloadHint, gameName, shortDigest, videoStatusLabel, videoStatusTone } from './labels.js'

defineProps({
  visible: { type: Boolean, default: false },
  material: { type: Object, default: null },
  loading: { type: Boolean, default: false },
  games: { type: Array, default: () => [] },
})
defineEmits(['update:visible', 'add', 'download'])
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
        <h3>{{ material.title || '未命名素材' }}</h3>
        <ResourceStatusBadge :tone="videoStatusTone(material.video_status)" :label="videoStatusLabel(material.video_status)" />
        <dl>
          <div><dt>素材 ID</dt><dd>#{{ material.id }}</dd></div>
          <div><dt>平台</dt><dd>{{ material.platform || '-' }}</dd></div>
          <div><dt>游戏</dt><dd>{{ gameName(games, material.game_id) }}</dd></div>
          <div><dt>作者</dt><dd>{{ material.author_name || '-' }}</dd></div>
          <div><dt>发布时间</dt><dd>{{ formatDateTime(material.published_at) }}</dd></div>
          <div><dt>入库时间</dt><dd>{{ formatDateTime(material.created_at) }}</dd></div>
          <div><dt>体积</dt><dd>{{ formatBytes(material.video_size_bytes) }}</dd></div>
          <div><dt>校验值</dt><dd :title="material.video_sha256 || ''">{{ shortDigest(material.video_sha256) }}</dd></div>
          <div><dt>准备完成于</dt><dd>{{ formatDateTime(material.video_prepared_at) }}</dd></div>
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
.material-detail h3 { margin: 0 0 10px; color: var(--wt-text-primary); font-size: 18px; line-height: 1.4; }
.material-detail dl { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px 16px; margin: 18px 0 0; }
.material-detail dt { color: var(--wt-text-tertiary); font-size: 12px; }
.material-detail dd { margin: 4px 0 0; color: var(--wt-text-primary); font-size: 14px; overflow-wrap: anywhere; }
.material-detail__error { margin: 16px 0 0; color: var(--wt-danger); font-size: 13px; line-height: 1.5; }
/* `align-items: center`：下载那颗按钮下面可能挂着一句提示，于是它比旁边那颗高。
   不居中就会让「加入我的素材」被拉成两行高，看起来像是另一类动作。 */
.material-detail__actions { display: flex; align-items: center; gap: 8px; margin-top: 20px; }
</style>
