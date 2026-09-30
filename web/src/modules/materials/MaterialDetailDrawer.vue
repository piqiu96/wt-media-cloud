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
              <ResourceStatusBadge :tone="videoStatusTone(material.video_status)" :label="videoStatusLabel(material.video_status)" />
              <ResourceStatusBadge v-if="mine" tone="info" label="已加入我的素材" />
            </div>
          </div>
        </section>

        <!-- 首段回答「我现在处理到哪一步，下一步是什么」（走查七轮用户提示词）：两个状态
             维度 + 加入时间 + 一句下一步，使用情况的五个事实跟在下面（协议见 labels.js）。 -->
        <section class="detail-card">
          <h4 class="detail-card__title">当前进度 / 使用情况</h4>
          <dl class="detail-card__grid detail-card__grid--4">
            <div><dt>文件状态</dt><dd><ResourceStatusBadge :tone="videoStatusTone(material.video_status)" :label="videoStatusLabel(material.video_status)" /></dd></div>
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
        <t-button class="wt-secondary-button" variant="outline" @click="$emit('update:visible', false)">关闭</t-button>
        <div class="material-detail__primary">
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
/* 左「关闭」右主操作：「关闭」是导航动作，与业务动作分开摆。 */
.material-detail__actions { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.material-detail__primary { display: flex; gap: 8px; }
.wt-primary-link { color: var(--wt-primary); font-weight: 600; text-decoration: none; }
.wt-primary-link:hover, .wt-primary-link:focus-visible { text-decoration: underline; }
</style>
