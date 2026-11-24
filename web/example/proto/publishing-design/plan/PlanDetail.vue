<script setup>
import {formatDateTime} from '../../../../src/shared/utils/datetime.js'
import { Button, DescriptionsItem as Field } from 'tdesign-vue-next'
import ResourceDetailPage from '../../../../src/shared/ui/resource/ResourceDetailPage.vue'
import ResourceCard from '../../../../src/shared/ui/resource/ResourceCard.vue'
import ResourceSection from '../../../../src/shared/ui/resource/ResourceSection.vue'
import ResourceDescriptions from '../../../../src/shared/ui/resource/ResourceDescriptions.vue'
import ResourceNotice from '../../../../src/shared/ui/resource/ResourceNotice.vue'
import ResourceRowActions from '../../../../src/shared/ui/resource/ResourceRowActions.vue'
import ResourceProgress from '../../../../src/shared/ui/resource/ResourceProgress.vue'
import ResourceStatGrid from '../../../../src/shared/ui/resource/ResourceStatGrid.vue'
import ResourceStatusBadge from '../../../../src/shared/ui/resource/ResourceStatusBadge.vue'
import ResourceEmptyState from '../../../../src/shared/ui/resource/ResourceEmptyState.vue'
defineProps({ plan: { type: Object, default: null }, view: { type: Object, default: null } })
defineEmits(['back'])
</script>

<template>
  <ResourceDetailPage v-if="plan" title="发布计划详情" back-label="返回发布计划列表" @back="$emit('back')">
    <ResourceNotice v-if="view.notice" :tone="view.notice.tone" :title="view.notice.title">
      {{ view.notice.text }}
      <template v-if="view.notice.actions?.length" #actions>
        <Button v-for="action in view.notice.actions" :key="action.key" theme="primary" variant="text" :data-act="action.key" :data-id="plan.id" :data-batch="action.batch" :data-pending="action.pending === false ? 'false' : undefined">{{ action.label }}</Button>
      </template>
    </ResourceNotice>
    <ResourceCard class="detail-card"><ResourceSection title="计划信息">
      <ResourceDescriptions>
        <Field label="计划名称">{{ plan.name }}</Field>
        <Field label="计划编号">{{ plan.id }}</Field>
        <Field label="计划状态"><ResourceStatusBadge :label="view.statusLabel" :tone="view.statusTone" /></Field>
        <Field v-if="plan.status === 'ended'" label="结束结果">{{ view.resultLabel }}</Field>
        <Field label="负责人">{{ plan.owner }}</Field>
        <Field label="执行周期">{{ plan.dateStart }} ～ {{ plan.dateEnd }}</Field>
        <Field label="发布平台">{{ plan.platforms.join('、') }}</Field>
        <Field label="发布类目">{{ plan.categories.join('、') }}</Field>
        <Field label="规划批次">{{ view.originalBatches }} 个原始批次</Field>
        <Field label="创建时间">{{ plan.created }}</Field>
        <Field label="最近更新">{{ formatDateTime(plan.updated) }}</Field>
        <Field v-if="plan.note" label="备注">{{ plan.note }}</Field>
      </ResourceDescriptions>
    </ResourceSection></ResourceCard>
    <ResourceCard class="detail-card"><ResourceSection title="目标完成情况" description="仅最终确认发布成功的任务计入达成量">
      <ResourceStatGrid :items="view.stats" />
      <ResourceProgress label="总体达成率" :percentage="view.rate" :value-text="`${view.actual} / ${plan.target} 条 ·`" />
      <div class="plan-detail-subtitle">平台 × 类目完成矩阵</div>
      <div class="plan-detail-table resource-data-table resource-embedded-table is-summary" tabindex="0" role="region" aria-label="平台与类目目标矩阵，可横向滚动">
        <table><thead><tr><th>平台</th><th>类目</th><th class="resource-table-number">目标数</th><th class="resource-table-number">发布成功</th><th class="resource-table-number">目标缺口</th><th class="resource-table-number">完成率</th></tr></thead><tbody>
          <tr v-for="cell in view.cells" :key="cell.platform + cell.category"><td>{{ cell.platform }}</td><td>{{ cell.category }}</td><td class="resource-table-number">{{ cell.target }}</td><td class="resource-table-number">{{ cell.actual }}</td><td class="resource-table-number">{{ cell.gap }}</td><td class="resource-table-number">{{ cell.rate }}%</td></tr>
          <tr class="plan-detail-total"><td colspan="2">合计</td><td class="resource-table-number">{{ view.sumTargets }}</td><td class="resource-table-number">{{ view.actual }}</td><td class="resource-table-number">{{ view.gap }}</td><td class="resource-table-number">{{ view.rate }}%</td></tr>
        </tbody></table>
      </div>
      <ResourceNotice v-if="view.hasMakeup">补量批次是额外发布尝试，独立于原始计划目标分配；原计划目标保持 {{ plan.target }} 条。</ResourceNotice>
    </ResourceSection></ResourceCard>
    <ResourceCard class="detail-card"><ResourceSection id="batch-area" title="批次推进">
      <div class="plan-detail-table resource-data-table resource-embedded-table is-summary" tabindex="0" role="region" aria-label="计划批次推进，可横向滚动">
        <table><thead><tr><th>顺序</th><th>批次名称</th><th>平台 / 类目</th><th class="resource-table-number">目标数</th><th>资源配置</th><th>执行状态</th><th class="resource-table-number">结果</th><th>当前待办</th><th>操作</th></tr></thead><tbody>
          <tr v-for="(row, i) in view.batches" :key="row.batch.id">
            <td>{{ i + 1 }}</td><td class="plan-detail-batch-name">{{ row.batch.name }}<small>{{ row.batch.id }}{{ row.current ? ' · 当前批次' : row.batch.type === 'makeup' ? ' · 补量批次' : '' }}</small></td>
            <td>{{ row.batch.platform }}<small>{{ row.targets }}</small></td><td class="resource-table-number">{{ row.target }}</td>
            <td>{{ row.batch.configured ? (plan.planReview && !row.batch.locked ? '已配置' : '已锁定') : '未配置' }}</td><td><ResourceStatusBadge :label="row.statusLabel" :tone="row.statusTone" /></td>
            <td class="resource-table-number">{{ ['ended', 'running', 'stopping'].includes(row.batch.phase) ? `${row.batch.success} / ${row.target}` : '—' }}</td>
            <td><span :class="{ 'plan-detail-pending': row.count }">{{ row.pendingLabel }}</span></td>
            <td><span v-if="plan.legacyReadOnly">—</span><ResourceRowActions v-else-if="row.actions"><Button v-for="(action, index) in row.actions" :key="action.key" :class="{ 'is-primary': index === 0 }" variant="text" data-act="batch-shortcut" :data-operation="action.key" :data-id="plan.id" :data-batch="row.batch.id">{{ action.label }}</Button></ResourceRowActions><ResourceRowActions v-else><Button v-if="row.count"  class="is-primary" variant="text" data-act="workbench" :data-id="plan.id" :data-batch="row.batch.id">去处理</Button><Button v-else-if="row.batch.phase === 'draft' && plan.status !== 'ended'" class="is-primary" variant="text" data-act="configure-batch" :data-id="plan.id" :data-batch="row.batch.id">去配置</Button><Button variant="text" data-act="view-batch" :data-id="plan.id" :data-batch="row.batch.id">详情</Button></ResourceRowActions></td>
          </tr>
        </tbody></table>
      </div>
    </ResourceSection></ResourceCard>
    <ResourceCard class="detail-card"><ResourceSection title="执行记录">
      <ol v-if="plan.logs?.length" class="plan-detail-history"><li v-for="([date, message, actor], i) in plan.logs" :key="i"><p>{{ message }}</p><span>{{ date }} · {{ actor }}</span></li></ol>
      <ResourceNotice v-else>暂无执行记录</ResourceNotice>
    </ResourceSection></ResourceCard>
    <template v-if="view.footer.length" #footer>
      <Button v-for="action in view.footer" :key="action.key" :disabled="action.disabled" :title="action.reason" :class="{ 'is-danger': action.danger }" :theme="action.primary ? 'primary' : 'default'" :variant="action.danger ? 'text' : action.primary ? 'base' : 'outline'" :data-act="action.key" :data-id="plan.id" :data-batch="action.batch">{{ action.label }}</Button>
    </template>
  </ResourceDetailPage>
  <ResourceDetailPage v-else title="发布计划详情" back-label="返回发布计划列表" @back="$emit('back')"><ResourceCard><ResourceEmptyState title="计划不存在" description="该计划可能已被移除，请返回发布计划列表。" /></ResourceCard></ResourceDetailPage>
</template>

<style scoped>
.plan-detail-subtitle { margin: 24px 0 12px; font-size: 14px; font-weight: 500; }
.plan-detail-table { overflow-x: auto; }
.plan-detail-table:focus-visible { outline: 2px solid var(--wt-primary); outline-offset: 2px; }
.plan-detail-table table { white-space: nowrap; }
.plan-detail-batch-name { min-width: 170px; max-width: 260px; white-space: normal; overflow-wrap: anywhere; }
.plan-detail-total { background: var(--wt-color-bg-subtle); }
.plan-detail-pending { color: var(--wt-color-warning-text); }
.plan-detail-history { list-style: none; margin: 0; padding: 0 0 0 8px; }
.plan-detail-history li { position: relative; padding: 0 0 20px 20px; border-left: 1px solid var(--wt-border); }
.plan-detail-history li:last-child { padding-bottom: 0; }
.plan-detail-history li::before { content: ''; position: absolute; left: -4px; top: 8px; width: 7px; height: 7px; background: var(--wt-primary); border-radius: 50%; }
.plan-detail-history p { margin: 0 0 4px; overflow-wrap: anywhere; }
.plan-detail-history span { color: var(--wt-text-tertiary); font-size: 12px; }
</style>
