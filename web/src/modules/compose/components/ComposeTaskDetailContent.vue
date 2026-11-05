<script setup>
import ResourceStatusBadge from '../../../shared/ui/resource/ResourceStatusBadge.vue'
import FeedbackAlert from '../../../shared/ui/feedback/FeedbackAlert.vue'
import { formatDateTime } from '../../../shared/utils/datetime.js'
import { formatDurationMs } from '../../../shared/utils/units.js'
import { attemptSteps, stepMetricsText } from '../attemptSteps.js'
import { platformText } from '../strategySettings.js'
import { schemaFields } from '../schemaFields.js'
import { triggerLabel } from '../labels.js'
import { UNCERTAIN_CODE, retryHint } from '../retryPolicy.js'
import { safeComposeError } from '../uncertainReview.js'
const props=defineProps({detail:Object,templates:{type:Array,default:()=>[]},attempts:{type:Array,default:()=>[]},attemptsError:String,outputSummary:Object,canRetry:Boolean,tab:{type:String,default:'overview'}})
const emit=defineEmits(['update:tab','material','strategy','output','batch'])
const taskName = row => row.display_name?.trim() || `合成任务 ${row.id}`
const outputID = row => row.status === 'success' ? row.composite_output_id : null
const strategyName = row => row.strategy_name || `策略 ${row.compose_strategy_id}`
const outcomeLabels = { pending:'等待执行',running:'处理中',success:'成功',failed:'失败',cancelled:'已取消' }
const outcomeTones = { pending:'warning',running:'info',success:'success',failed:'danger',cancelled:'neutral' }
const statusLabels = { pending: '排队中', running: '处理中', success: '成功', failed: '失败', cancelled: '已取消' }
const statusTones = { pending: 'warning', running: 'info', success: 'success', failed: 'danger', cancelled: 'neutral' }
const stageLabels = {
  pending: '等待源素材',
  validating_input: '校验素材',
  waiting_input: '等待素材准备',
  cutting: '视频合成中',
  uploading: '上传成片',
  committing: '保存成片',
}

function statusText(row) {
  if (row.status === 'running' && row.cancel_requested) return '停止请求已受理'
  if (row.status === 'running' && row.stage === 'waiting_input') return stageLabels.waiting_input
  return statusLabels[row.status] || row.status
}

/** 当前阶段只说阶段，不带进度百分比；失败时也不拿错误码的文字冒充阶段。 */
function stageText(row) {
  if (row.status === 'success') return '已完成'
  // 阶段名里带「中」的说法（视频合成中）不能直接接「失败」，去掉尾字再拼。
  if (row.status === 'failed') return row.stage ? `${(stageLabels[row.stage] || '执行').replace(/中$/, '')}失败` : '执行失败'
  if (row.status === 'cancelled') return '已取消'
  if (row.status === 'pending') return row.stage === 'waiting_input' ? stageLabels.waiting_input : '等待执行'
  return stageLabels[row.stage] || '处理中'
}

function snapshotFields(row) {
  const template = props.templates.find((item) => item.code === row.template_code && item.version === row.template_version)
  return schemaFields(template?.schema).filter((field) => Object.hasOwn(row.config_snapshot || {}, field.key) && field.key !== 'guide_video_url')
}
function snapshotValue(row, field) {
  const value = row.config_snapshot?.[field.key]
  if (typeof value === 'boolean') return value ? '开启' : '关闭'
  if (typeof value === 'number' && field.key.endsWith('_seconds')) return `${value} 秒`
  return typeof value === 'number' ? String(value) : typeof value === 'string' && !value.includes('://') ? value : '—'
}

</script>
<template>
          <div v-if="detail" class="compose-detail">
            <t-tabs :value="tab" @change="emit('update:tab', $event)" class="compose-detail__tabs">
              <t-tab-panel value="overview" label="任务信息">
                <div class="compose-detail__section">
                  <h4>任务信息</h4>
                  <div class="compose-detail__grid">
                    <div class="compose-detail__wide"><span class="compose-detail__label">任务名称</span><span>{{ taskName(detail) }}</span></div>
                    <div><span class="compose-detail__label">任务 ID</span><span>{{ detail.id }}</span></div>
                    <div><span class="compose-detail__label">触发方式</span><span>{{ triggerLabel(detail.trigger_type) }}</span></div>
                    <div><span class="compose-detail__label">任务批次</span><t-link v-if="detail.batch_id" theme="primary" @click="emit('batch', detail.batch_id)">批次 {{ detail.batch_id }}</t-link><span v-else>未记录</span></div>
                    <div><span class="compose-detail__label">负责人</span><span>{{ detail.operator_name || '未记录' }}</span></div>
                    <div><span class="compose-detail__label">创建时间</span><span>{{ formatDateTime(detail.created_at) }}</span></div>
                    <div><span class="compose-detail__label">完成时间</span><span>{{ formatDateTime(detail.finished_at) }}</span></div>
                    <div><span class="compose-detail__label">状态</span><ResourceStatusBadge :tone="statusTones[detail.status] || 'neutral'" :label="statusText(detail)" /></div>
                    <!-- 失败原因就写在任务信息里：找「为什么失败」的人先看的就是这一段，不必
                         再翻到执行记录去对错误码。列表页不加列（详情才需要这个上下文）。 -->
                    <div v-if="detail.status === 'failed'"><span class="compose-detail__label">失败原因</span><span class="compose-detail__failure-reason">{{ safeComposeError(detail.error_code) }}</span></div>
                  </div>
                  <!-- 重试按钮为什么不在，就写在这里：这一格给的是「不是所有失败都能重试」的
                       那个理由（错误类型 / 账号权限），两列网格里放不下，也不属于任务信息。 -->
                  <p v-if="retryHint(detail, canRetry)" class="compose-detail__hint">{{ retryHint(detail, canRetry) }}</p>
                </div>
                <div class="compose-detail__section">
                  <h4>合成配置</h4>

                  <div class="compose-detail__grid compose-detail__snapshot-fields">
                    <div><span class="compose-detail__label">模板</span><span>{{ templates.find(row => row.code === detail.template_code && row.version === detail.template_version)?.name || detail.template_code }}</span></div>
                    <div><span class="compose-detail__label">成片设置</span><span>{{ detail.setting_name || '未记录' }} · 第 {{ detail.copy_index || 1 }} 份</span></div>
                    <div><span class="compose-detail__label">适用平台</span><span>{{ platformText(detail.platforms) }}</span></div>
                    <div><span class="compose-detail__label">模板版本</span><span>v{{ detail.template_version }}</span></div>
                    <div v-for="field in snapshotFields(detail)" :key="field.key"><span class="compose-detail__label">{{ field.label }}</span><span>{{ snapshotValue(detail, field) }}</span></div>
                  </div>
                  <p v-if="!snapshotFields(detail).length" class="compose-detail__hint">当前没有可安全展示的模板参数定义。</p>
                </div>
                <div class="compose-detail__section">
                  <h4>关联信息</h4>
                  <div class="compose-detail__grid">
                    <div><span class="compose-detail__label">关联素材</span><t-link theme="primary" @click="emit('material', detail.material_id)">素材 {{ detail.material_id }}</t-link></div>
                    <div><span class="compose-detail__label">合成策略</span><t-link theme="primary" @click="emit('strategy', detail.compose_strategy_id)">{{ strategyName(detail) }}</t-link></div>
                  </div>
                </div>
                <div class="compose-detail__section">
                  <h4>执行信息</h4>
                  <div class="compose-detail__grid">
                    <div><span class="compose-detail__label">当前阶段</span><span>{{ stageText(detail) }}</span></div>
                    <div><span class="compose-detail__label">执行次数</span><span>{{ detail.attempt_count }}</span></div>
                    <div><span class="compose-detail__label">开始时间</span><span>{{ formatDateTime(detail.started_at) }}</span></div>
                    <div><span class="compose-detail__label">更新时间</span><span>{{ formatDateTime(detail.updated_at) }}</span></div>
                  </div>
                </div>
                <div class="compose-detail__section">
                  <h4>生成结果</h4>
                  <!-- 成片只在「生成结果」里出现一次，生成时间自己占一格：这两样是并排的两条
                       事实，拼成一句话反而让人在一串文字里找时间。 -->
                  <div v-if="outputID(detail)" class="compose-detail__grid">
                    <div><span class="compose-detail__label">生成成片</span><t-link theme="primary" @click="emit('output', outputID(detail))">成片 {{ outputID(detail) }}</t-link></div>
                    <div><span class="compose-detail__label">生成时间</span><span>{{ outputSummary ? formatDateTime(outputSummary.created_at) : '—' }}</span></div>
                  </div>
                  <template v-else-if="detail.status === 'failed'">
                    <p class="compose-detail__empty">未生成成片</p>

                  </template>
                  <p v-else-if="detail.status === 'cancelled'" class="compose-detail__empty">未生成成片</p>
                  <p v-else class="compose-detail__empty">成片尚未生成</p>
                </div>
              </t-tab-panel>
              <t-tab-panel value="attempts" label="执行记录">
                <!-- 已成功的任务可能留着上一次失败的 error_code：只有当前仍是失败时才谈失败原因。 -->
                <div v-if="detail.status === 'failed' && (detail.error_code || detail.error_message)" class="compose-detail__section">
                  <h4>失败原因</h4>
                  <FeedbackAlert theme="error" :message="detail.error_code === 'execution_uncertain' ? '上次执行结果不明确，等待人工核对。系统尚未确认旧进程已经停止。' : safeComposeError(detail.error_code)" />
                  <!-- 与概览里那一句同源（retryPolicy），同一件事不并存两种说法。 -->
                  <p v-if="retryHint(detail, canRetry)" class="compose-detail__hint">{{ retryHint(detail, canRetry) }}</p>
                  <p v-else-if="detail.status === 'failed' && detail.error_code === 'execution_failed'" class="compose-detail__hint">请根据失败原因判断是否重试。</p>
                </div>
                <div class="compose-detail__section">
                  <FeedbackAlert v-if="attemptsError" theme="error" :message="attemptsError" />
                  <template v-else>
                    <p v-if="!attempts.length" class="compose-detail__empty">还没有执行记录。</p>
                    <details v-for="(attempt, attemptIndex) in attempts" :key="attempt.attempt" :open="attemptIndex === 0" class="compose-detail__attempt">
                      <summary class="compose-detail__attempt-head">
                        <strong>第 {{ attempt.attempt }} 次执行</strong>
                        <ResourceStatusBadge :tone="outcomeTones[attempt.outcome] || 'neutral'" :label="outcomeLabels[attempt.outcome] || '状态未知'" />
                      </summary>
                      <div class="compose-detail__attempt-meta">开始：{{ formatDateTime(attempt.started_at) }} · 结束：{{ formatDateTime(attempt.finished_at) }}</div>
                      <!-- 过程：云端每走完一步追加一条，谁在哪一步倒下、每步花了多久都在这。
                           没有步骤的用户是更早的执行记录（那一列当时还不存在），如实说没有。 -->
                      <ul v-if="attemptSteps(attempt).length > 0" class="compose-detail__steps">
                        <li v-for="step in attemptSteps(attempt)" :key="step.step">
                          <span class="compose-detail__step-name">{{ step.label || '未记录' }}</span>
                          <span>{{ formatDateTime(step.started_at) }} → {{ formatDateTime(step.finished_at) }}</span>
                          <span v-if="step.duration_ms != null">用时 {{ formatDurationMs(step.duration_ms) }}</span>
                          <span v-if="stepMetricsText(step)" class="compose-detail__step-metrics">{{ stepMetricsText(step) }}</span>
                          <span v-if="step.outcome === 'failed'" class="compose-detail__step-error">失败：{{ step.error_label || '原因未记录' }}</span>
                        </li>
                      </ul>
                      <p v-else class="compose-detail__attempt-meta">该次执行没有步骤记录。</p>
                      <p v-if="attempt.outcome === 'success'" class="compose-detail__attempt-meta">本次执行成功，输出校验完成<span v-if="outputID(detail)">，生成成片 {{ outputID(detail) }}</span>。</p>
                      <template v-if="attempt.outcome === 'failed'">
                        <p class="compose-detail__attempt-meta">失败阶段：{{ stageLabels[attempt.stage] || '阶段未记录' }}</p>
                        <p class="compose-detail__attempt-err">失败原因：{{ attempt.error_code === 'execution_uncertain' ? '执行结果不明确，需人工核对' : safeComposeError(attempt.error_code) }}</p>
                        <p class="compose-detail__attempt-meta">可重试：{{ attempt.error_code === UNCERTAIN_CODE ? '不可直接重试' : '可由高级运营提交重试' }}</p>
                      </template>
                      <div v-if="attempt.uncertain_review" class="compose-detail__review">
                        <strong>人工核对</strong><p class="compose-detail__attempt-meta">核对人 {{ attempt.uncertain_review.actor_user_id }} · {{ formatDateTime(attempt.uncertain_review.created_at) }}</p>
                        <p v-if="attempt.uncertain_review.evidence" class="compose-detail__review-evidence">{{ attempt.uncertain_review.evidence }}</p>
                      </div>
                    </details>
                  </template>
                </div>
                <slot name="uncertain-review" />
              </t-tab-panel>

            </t-tabs>
          </div>
</template>
<style scoped>
/* 策略名长起来会把行撑成两行；与成片页同一处理：一行截断，全名挂在 title 上。 */
.compose-detail { min-height: 320px; }
.compose-detail__tabs { margin-top: 4px; }
.compose-detail__section { margin-top: 24px; min-width:0; }
.compose-detail__section h4 { padding-left:10px; border-left:3px solid var(--wt-primary); margin: 0 0 16px; color: var(--wt-text-primary); font-size: 15px; font-weight: 600; }
.compose-detail__grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px 24px; font-size: 14px; line-height: 1.5; }
.compose-detail__grid > div { display: grid; grid-template-columns:112px minmax(0,1fr); gap:16px; min-width:0; color:var(--wt-text-primary);overflow-wrap:anywhere;align-items:start; }
.compose-detail__label {color:var(--wt-text-tertiary);font-size:14px;}
.compose-detail__wide{grid-column:1/-1;}
.compose-detail__snapshot-fields { margin-top: 16px; }
/* 失败态用 --wt-danger：它定义在入口加载的 design-token.css 里，
   且与紧邻「当前状态」的失败徽标（statusTones.failed=danger）同色。 */
.compose-detail__failure-reason { color: var(--wt-danger); font-weight: 500; }
.compose-detail__hint { margin: 0; color: var(--wt-text-secondary); font-size: 12px; line-height: 1.5; }
.compose-detail__grid + .compose-detail__hint, .compose-detail__empty + .compose-detail__hint, .compose-detail__section > .compose-detail__hint + .compose-detail__grid { margin-top: 12px; }
.compose-detail__empty { margin: 0; color: var(--wt-text-secondary); font-size: 14px; }
.compose-detail__attempt { padding: 0 0 20px;border:0;border-bottom:1px solid var(--wt-border); }
.compose-detail__attempt + .compose-detail__attempt { margin-top: 12px; }
.compose-detail__attempt-head { display: flex; justify-content: space-between; gap: 12px; align-items: center; }
.compose-detail__attempt-head strong { color: var(--wt-text-primary); font-size: 14px; }
.compose-detail__attempt-meta { margin: 8px 0 0; color: var(--wt-text-secondary); font-size: 12px; line-height: 1.5; }
.compose-detail__attempt-err { margin: 8px 0 0; color: var(--wt-danger); font-size: 12px; }
.compose-detail__steps { margin: 20px 0 0 6px; padding: 0 0 0 20px;border-left:1px solid var(--wt-border); list-style: none; }
.compose-detail__steps li { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px 12px; position:relative;padding:0 0 20px;border:0; color: var(--wt-text-secondary); font-size: 12px; line-height: 1.5; }
.compose-detail__steps li::before{content:"";position:absolute;left:-25px;top:5px;width:8px;height:8px;border-radius:50%;background:var(--wt-primary);box-shadow:0 0 0 4px var(--wt-bg-card);}
.compose-detail__step-name { min-width: 108px; color: var(--wt-text-primary); font-weight: 500; }
.compose-detail__step-metrics { color: var(--wt-text-tertiary); }
.compose-detail__step-error { color: var(--wt-danger); }
.compose-detail__review { display: flex; flex-direction: column; align-items: flex-start; gap: 12px; }
.compose-detail__review h4 { margin-bottom: 0; }
.compose-detail__review :deep(.t-textarea) { width: 100%; }
.compose-detail__review-evidence { margin: 8px 0 0; color: var(--wt-text-primary); font-size: 13px; white-space: pre-wrap; overflow-wrap: anywhere; }
.compose-detail__footer { display: flex; justify-content: flex-end; align-items: center; gap: 8px; }
@media (max-width: 760px) { .compose-detail__grid { grid-template-columns: 1fr; } }
</style>
