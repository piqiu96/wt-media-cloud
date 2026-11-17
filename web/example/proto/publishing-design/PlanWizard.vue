<script setup>
import { reactive, ref, computed } from "vue";
import ResourceOverlay from "./PrototypeOverlay.vue";
import ResourceWizardSteps from "../../../src/shared/ui/resource/ResourceWizardSteps.vue";
import { platforms, categories } from "./model.js";
import { clone, confirmPlan, planImpact, itemRecord } from "./relationships.js";
import { platformInventory, businessDate } from "./inventory.js";
import { feedback } from "../../../src/shared/ui/feedback/index.js";
const props = defineProps({ db: Object, persist: Function });
const emit = defineEmits(["saved"]);
const confirmed = ref(false);
const visible = ref(false),
  step = ref(0),
  error = ref(""),
  busy = ref(false),
  now = ref(new Date().toISOString());
const draft = reactive({
  name: "",
  targets: [],
  splits: [],
  platformOrder: [...platforms],
  confirmKey: "",
});
const total = computed(() =>
  draft.targets.reduce((n, t) => n + Number(t.count), 0),
);
const targetCategories = computed(() => [
  ...new Set(draft.targets.map((t) => t.category)),
]);
const activePlatformOrder = computed(() =>
  draft.platformOrder.filter((platform) =>
    draft.targets.some((t) => t.platform === platform && Number(t.count) > 0),
  ),
);
const inventory = computed(() => platformInventory(props.db, now.value));
const categoryCandidateLimit = (row) =>
  row?.eligibleAccounts ? row.free : 0;
const platformCandidateLimit = (row) =>
  row.byCategory.reduce((sum, category) => sum + categoryCandidateLimit(category), 0);
function open(p) {
  if (p?.lockedAt) {
    error.value = "计划结构已锁定";
    return;
  }
  Object.assign(draft, {
    id: p?.id,
    name: p?.name || "",
    targets: clone(
      p?.targets ||
        platforms.flatMap((platform) =>
          categories.map((category) => ({ platform, category, count: 0 })),
        ),
    ),
    splits: p?.confirmed
      ? (p.items || []).map((i) => {
          const b = itemRecord(props.db, i);
          return {
            id: b.id,
            name: b.name,
            platform: b.platform,
            targets: clone(b.targets),
          };
        })
      : clone(p?.splits || []),
    platformOrder: clone(p?.platformOrder || platforms),
    confirmKey: p?.confirmKey || crypto.randomUUID(),
    revision: p?.revision,
  });
  confirmed.value = !!p?.confirmed;
  error.value = "";
  step.value = 0;
  visible.value = true;
}
function add(platform) {
  draft.splits.push({
    name:
      platform +
      " · 批次 " +
      String.fromCharCode(
        65 + draft.splits.filter((b) => b.platform === platform).length,
      ),
    platform,
    targets: draft.targets
      .filter((t) => t.platform === platform && t.count > 0)
      .map((t) => ({
        category: t.category,
        count: Math.max(
          0,
          t.count -
            draft.splits
              .filter((b) => b.platform === platform)
              .reduce(
                (n, b) =>
                  n +
                  Number(
                    b.targets.find((x) => x.category === t.category)?.count ||
                      0,
                  ),
                0,
              ),
        ),
      })),
  });
}
function move(b, delta) {
  const group = draft.splits.filter((x) => x.platform === b.platform),
    i = group.indexOf(b),
    other = group[i + delta];
  if (!other) return;
  const from = draft.splits.indexOf(b),
    to = draft.splits.indexOf(other);
  [draft.splits[from], draft.splits[to]] = [
    draft.splits[to],
    draft.splits[from],
  ];
}
function movePlatform(platform, delta) {
  const other = activePlatformOrder.value[activePlatformOrder.value.indexOf(platform) + delta];
  if (!other) return;
  const from = draft.platformOrder.indexOf(platform);
  const to = draft.platformOrder.indexOf(other);
  [draft.platformOrder[from], draft.platformOrder[to]] = [draft.platformOrder[to], draft.platformOrder[from]];
}
function next() {
  step.value++;
  if (step.value === 2) {
    for (const b of draft.splits) {
      b.targets = draft.targets
        .filter((t) => t.platform === b.platform && t.count > 0)
        .map((t) => ({
          category: t.category,
          count: b.targets.find((x) => x.category === t.category)?.count || 0,
        }));
    }
    if (!draft.splits.length)
      draft.platformOrder
        .filter((p) =>
          draft.targets.some((t) => t.platform === p && t.count > 0),
        )
        .forEach(add);
  }
}
function ordered() {
  draft.splits.sort(
    (a, b) =>
      draft.platformOrder.indexOf(a.platform) -
      draft.platformOrder.indexOf(b.platform),
  );
}
async function save(draftOnly = false) {
  busy.value = true;
  error.value = "";
  try {
    ordered();
    const p = props.db.plans.find((p) => p.id === draft.id);
    if (p?.confirmed) {
      const changes = planImpact(props.db, p, draft);
      if (
        !(await feedback.confirmWarning({
          title: "确认逐批影响",
          message: changes
            .map(
              (c) =>
                `${c.name}：${c.changed ? "目标变化，保留兼容候选并重新预检" : "目标不变，保留候选"}（${c.candidates} 条人工配置）`,
            )
            .join("；"),
          confirmText: "确认修改",
        }))
      )
        return;
    }
    const result = confirmPlan(props.db, draft, { draftOnly });
    draft.id = result.id;
    draft.revision = result.revision;
    confirmed.value = !!result.confirmed;
    if (result.confirmed)
      draft.splits = result.items.map((i) => {
        const b = itemRecord(props.db, i);
        return {
          id: b.id,
          name: b.name,
          platform: b.platform,
          targets: clone(b.targets),
        };
      });
    if (props.persist && !(await props.persist()))
      throw Error("保存失败，输入已保留，请重试");
    visible.value = false;
    emit("saved", result);
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
async function close() {
  if (
    !(await feedback.confirmWarning({
      title: "保留还是离开？",
      message: "请先保存计划草稿以便刷新恢复；离开将放弃本次未保存编辑。",
      confirmText: "放弃本次编辑",
    }))
  )
    return;
  visible.value = false;
}
defineExpose({ open });
</script>
<template>
  <ResourceOverlay
    :visible="visible"
    @update:visible="close"
    title="新建 / 编辑发布计划"
    workspace
    kind="drawer"
    :width="1280"
    :busy="busy"
  >
    <ResourceWizardSteps
      :current="step"
      :steps="
        ['资源盘点', '平台 × 类目目标', '按平台拆分批次', '确认轻量骨架'].map(
          (title, value) => ({ title, value }),
        )
      "
    />
    <t-alert v-if="error" theme="warning" :message="error" />
    <div class="plan-workflow-grid">
      <div class="plan-workflow-main">
    <section class="plan-section" v-if="step === 0">
      <div class="plan-section-heading">
        <div><span class="section-kicker">01 / RESOURCE INVENTORY</span><h3>资源盘点</h3><p>{{ businessDate(now) }} · Asia/Shanghai；共享账号去重，按确认公开时间统计。</p></div>
        <t-button variant="outline" @click="now = new Date().toISOString()">刷新资源</t-button>
      </div>
      <div class="plan-metrics">
        <div><small>可用平台账号</small><strong>{{ inventory.reduce((n, r) => n + r.available, 0) }}</strong><span>按平台分别统计</span></div>
        <div><small>今日已公开</small><strong>{{ inventory.reduce((n, r) => n + r.published, 0) }}</strong><span>确认公开时间落在今日</span></div>
        <div><small>今日已安排</small><strong>{{ inventory.reduce((n, r) => n + r.scheduled, 0) }}</strong><span>与已公开分开统计</span></div>
        <div><small>可配候选上限</small><strong>{{ inventory.reduce((n, r) => n + platformCandidateLimit(r), 0) }}</strong><span>跨平台候选，不等于成片去重总量</span></div>
      </div>
      <t-alert v-if="inventory.some((r) => r.estimate === null)" class="plan-capacity-note" theme="info" message="部分账号没有已核实的每日额度。下方“预计可安排候选”是满足平台、类目和成片唯一性的上限，不代表今天可以实际发布的数量；有真实额度的平台按共享账号联合估算。" />
      <h4>平台账号与今日发布</h4>
      <div class="plan-platform-cards">
        <article v-for="r in inventory" :key="r.platform" class="plan-platform-card">
          <header><strong>{{ r.platform }}</strong><span>可用账号 {{ r.available }} / {{ r.total }}</span></header>
          <div class="plan-platform-numbers"><div><small>今日已公开</small><strong>{{ r.published }}</strong><span>{{ r.publishAccounts }} 个去重账号</span></div><div><small>今日已安排</small><strong>{{ r.scheduled }}</strong><span>未并入已公开</span></div><div><small>预计可安排候选</small><strong>{{ r.estimate ?? platformCandidateLimit(r) }}</strong><span>{{ r.estimate === null ? '资源上限 · 日额度未知' : '已按共享额度联合估算' }}</span></div></div>
          <p>账号分层 {{ r.quality }}（优质 / 普通 / 新号）<span v-if="r.invalid"> · {{ r.invalid }} 个待登录或不可用</span></p>
        </article>
      </div>
      <h4 id="plan-category-inventory">各类目成片与平台可配量</h4>
      <div class="plan-table-scroll resource-data-table resource-embedded-table is-editable"><table class="closure-table plan-category-matrix"><thead><tr><th>内容类目 / 成片池</th><th v-for="r in inventory" :key="r.platform">{{ r.platform }}</th></tr></thead><tbody><tr v-for="category in categories" :key="category"><th><strong>{{ category }}</strong><small>总计 {{ db.assets.filter((a) => a.category === category).length }} · 优质 {{ db.assets.filter((a) => a.category === category && a.quality === '优质').length }} · 普通 {{ db.assets.filter((a) => a.category === category && a.quality !== '优质').length }}</small></th><td v-for="r in inventory" :key="r.platform"><strong>{{ categoryCandidateLimit(r.byCategory.find((c) => c.category === category)) }}</strong> 条候选<small>可用成片 {{ r.byCategory.find((c) => c.category === category)?.free || 0 }} · 已占用 {{ r.byCategory.find((c) => c.category === category)?.occupied || 0 }} · 合格账号 {{ r.byCategory.find((c) => c.category === category)?.eligibleAccounts || 0 }}</small></td></tr></tbody></table></div>
      <p class="plan-footnote">预计可安排候选按平台分别计算；同一成片跨平台可复用。共享账号额度未知时不把各类目候选相加当成今日可执行量。资源盘点时间：{{ now }}。</p>
    </section>
    <section class="plan-section" v-if="step === 1">
      <div class="plan-section-heading"><div><span class="section-kicker">02 / TARGET MATRIX</span><h3>设定平台与类目目标</h3><p>输入计划目标；资源缺口将在批次配置时继续核验。</p></div></div>
      <label class="plan-name-field">计划名称 *<t-input v-model="draft.name" aria-label="计划名称" placeholder="例如：本周游戏内容发布计划" /></label>
      <h4>平台 × 类目目标 <span class="plan-total">总目标 {{ total }} 条</span></h4>
      <div class="plan-table-scroll resource-data-table resource-embedded-table is-editable"><table class="closure-table plan-target-matrix"><thead><tr><th>发布平台</th><th v-for="c in targetCategories" :key="c">{{ c }}</th><th>平台合计</th></tr></thead><tbody><tr v-for="p in platforms" :key="p"><th>{{ p }}</th><td v-for="c in targetCategories" :key="c"><t-input-number :value="draft.targets.find((t) => t.platform === p && t.category === c)?.count || 0" :min="0" :decimal-places="0" :aria-label="p + c + '目标'" @change="(v) => { const target = draft.targets.find((t) => t.platform === p && t.category === c); if (target) target.count = v; else draft.targets.push({ platform: p, category: c, count: v }); }" /></td><td><strong>{{ draft.targets.filter((t) => t.platform === p).reduce((n, t) => n + Number(t.count), 0) }}</strong> 条</td></tr></tbody></table></div>
      <p class="plan-footnote">目标表示规划愿望；超出资源的缺口必须在批次配置时修复，不能按目标直接启动。</p>
    </section>
    <section class="plan-section" v-if="step >= 2">
      <label v-if="step === 3"
        >计划名称：{{ draft.name }} · 总目标 {{ total }}</label
      >
      <section
        v-for="(p, pi) in activePlatformOrder"
        :key="p"
        class="platform-group"
      >
        <h3>
          {{ p }} · 目标
          {{
            draft.targets
              .filter((t) => t.platform === p)
              .reduce((n, t) => n + Number(t.count), 0)
          }}
          <t-button v-if="step === 2" variant="text" @click="add(p)">新增批次</t-button
          ><t-button
            v-if="step === 2"
            variant="text"
            :disabled="pi === 0"
            @click="movePlatform(p, -1)"
            >平台组上移</t-button
          >
        </h3>
        <div
          class="plan-split"
          v-for="b in draft.splits.filter((b) => b.platform === p)"
          :key="b.id || b.name"
        >
          <t-input v-if="step === 2" v-model="b.name" aria-label="批次名称" /><strong v-else class="plan-confirm-name">{{ b.name }}</strong><label
            v-for="t in b.targets"
            :key="t.category"
            >{{ t.category
            }}<t-input-number v-if="step === 2"
              v-model="t.count"
              :min="0"
              :decimal-places="0"
              :aria-label="b.name + t.category + '分目标'" /><strong v-else>{{ t.count }} 条</strong></label
          ><t-button v-if="step === 2" variant="text" :disabled="draft.splits.filter((x) => x.platform === p).indexOf(b) === 0" @click="move(b, -1)">上移</t-button
          ><t-button v-if="step === 2" variant="text" :disabled="draft.splits.filter((x) => x.platform === p).indexOf(b) === draft.splits.filter((x) => x.platform === p).length - 1" @click="move(b, 1)">下移</t-button
          ><t-button
            v-if="step === 2"
            variant="text"
            @click="
              draft.splits.push({
                ...clone(b),
                id: null,
                name: b.name + ' 副本',
              })
            "
            >复制</t-button
          ><t-button
            v-if="step === 2"
            variant="text"
            @click="draft.splits = draft.splits.filter((x) => x !== b)"
            >移除</t-button
          >
        </div>
      </section>
      <t-alert
        theme="info"
        message="确认仅生成轻量批次骨架，不创建正式任务、不占用成片、不执行。每个矩阵目标必须严格等于所属批次分目标之和。"
      />
    </section>
      </div>
      <aside class="plan-workflow-rail">
        <section><span class="section-kicker">PLAN SUMMARY</span><h3>计划摘要</h3><dl><div><dt>计划名称</dt><dd>{{ draft.name || '待填写' }}</dd></div><div><dt>目标总数</dt><dd>{{ total }} 条</dd></div><div><dt>平台组</dt><dd>{{ draft.targets.filter((t) => Number(t.count) > 0).map((t) => t.platform).filter((p, i, a) => a.indexOf(p) === i).length }} 个</dd></div><div><dt>批次骨架</dt><dd>{{ draft.splits.length }} 个</dd></div></dl></section>
        <section><span class="section-kicker">CURRENT STEP</span><h3>{{ ['资源盘点', '目标矩阵', '拆分批次', '确认计划'][step] }}</h3><p v-if="step === 0">先确认账号与成片是否满足计划方向，容量待核实时请人工复核。</p><p v-else-if="step === 1">目标可调整；下一步按平台拆成独立批次。</p><p v-else>每个批次只对应一个平台。分目标之和需要与矩阵一致。</p></section>
        <section class="rail-info"><strong>创建后的状态</strong><p>确认计划仅生成轻量批次骨架，不创建正式任务、不占用成片，也不会自动执行。</p></section>
      </aside>
    </div>
    <template #footer
      ><t-button v-if="!confirmed" variant="outline" @click="save(true)"
        >保存计划草稿</t-button
      ><t-button v-if="step" variant="outline" @click="step--">上一步</t-button
      ><t-button v-if="step < 3" @click="next">下一步</t-button
      ><t-button v-else :loading="busy" @click="save(false)"
        >确认计划并生成 {{ draft.splits.length }} 个批次</t-button
      ></template
    >
  </ResourceOverlay>
</template>
<style scoped>
.plan-workflow-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 295px;
  gap: 18px;
  align-items: start;
  margin-top: 22px;
}
.plan-workflow-main { min-width: 0; }
.plan-section {
  margin: 0;
  padding: 24px;
  border: 1px solid #e2e9f3;
  border-radius: 10px;
  background: #fff;
  box-shadow: 0 3px 14px #24446b0a;
}
.plan-section-heading { display:flex; justify-content:space-between; align-items:start; gap:16px; margin-bottom:21px; }
.plan-section-heading h3 { margin:4px 0 5px; font-size:19px; }
.plan-section-heading p { margin:0; color:#72829a; font-size:12px; }
.section-kicker { font-size:10px; letter-spacing:.1em; color:#3976d6; font-weight:700; }
.plan-section h4 { display:flex; align-items:center; justify-content:space-between; margin:22px 0 9px; font-size:14px; color:#263d60; }
.plan-metrics { display:grid; grid-template-columns:repeat(4, minmax(0,1fr)); gap:10px; }
.plan-metrics > div { display:flex; flex-direction:column; gap:2px; padding:14px 16px; border:1px solid #e4ebf5; border-radius:8px; background:#f8faff; }
.plan-metrics small,.plan-metrics span { color:#7888a0; font-size:11px; }
.plan-metrics strong { color:#1d65ce; font-size:24px; line-height:1.2; }
.plan-capacity-note { margin:16px 0 20px; }
.plan-platform-cards { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:10px; }
.plan-platform-card { min-width:0; border:1px solid #e2e9f3; border-radius:8px; background:#fff; overflow:hidden; }
.plan-platform-card header { display:flex; justify-content:space-between; align-items:center; gap:8px; padding:12px 14px; background:#f7faff; border-bottom:1px solid #e9eef6; }
.plan-platform-card header strong { color:#2a4f84; font-size:14px; }
.plan-platform-card header span { color:#7185a2; font-size:11px; }
.plan-platform-numbers { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:3px; padding:12px 10px 8px; }
.plan-platform-numbers > div { display:flex; flex-direction:column; gap:2px; min-width:0; padding:0 5px; }
.plan-platform-numbers small,.plan-platform-numbers span { color:#8291a6; font-size:10px; line-height:1.4; }
.plan-platform-numbers strong { color:#2469d3; font-size:22px; line-height:1.2; }
.plan-platform-card p { margin:0; padding:10px 14px 12px; border-top:1px solid #edf1f7; color:#6a7c96; font-size:11px; line-height:1.5; }
.plan-category-matrix { min-width:650px; }
.plan-category-matrix tbody th { width:205px; background:#fbfcff; }
.plan-category-matrix tbody th strong,.plan-category-matrix tbody th small { display:block; }
.plan-category-matrix tbody th small,.plan-category-matrix td small { display:block; margin-top:5px; color:#8493a8; font-size:11px; line-height:1.5; }
.plan-category-matrix td { min-width:145px; }
.plan-category-matrix td strong { color:#2369d5; font-size:17px; }
.plan-table-scroll { overflow-x:auto; }
.plan-category-pool { display:grid; grid-template-columns:repeat(3, minmax(0,1fr)); gap:10px; }
.plan-category-pool > div { display:flex; flex-direction:column; gap:3px; border:1px solid #e5ebf4; border-radius:8px; padding:11px 13px; }
.plan-category-pool strong { font-size:13px; }
.plan-category-pool span,.plan-category-pool small { color:#74849d; font-size:11px; }
.plan-footnote { padding:10px 12px; background:#eff5ff; color:#59739d; border-radius:6px; font-size:12px; line-height:1.6; }
.plan-workflow-rail { display:flex; flex-direction:column; gap:12px; }
.plan-workflow-rail section { padding:18px; background:#fff; border:1px solid #e2e9f3; border-radius:10px; box-shadow:0 3px 14px #24446b0a; }
.plan-workflow-rail h3 { margin:4px 0 13px; font-size:15px; }
.plan-workflow-rail p { margin:0; font-size:12px; line-height:1.7; color:#647895; }
.plan-workflow-rail dl { margin:0; }
.plan-workflow-rail dl div { display:flex; justify-content:space-between; gap:8px; padding:9px 0; border-top:1px solid #edf1f7; font-size:12px; }
.plan-workflow-rail dt { color:#8290a5; }
.plan-workflow-rail dd { margin:0; text-align:right; font-weight:600; }
.plan-workflow-rail .rail-info { background:#fff9ed; border-color:#f2e3bf; }
.rail-info strong { display:block; margin-bottom:8px; color:#915a08; font-size:13px; }
.plan-name-field { display:flex; flex-direction:column; gap:8px; font-size:12px; font-weight:600; max-width:520px; }
.plan-total { color:#2369d5; font-size:13px; }
.plan-target-matrix { min-width:580px; }
.plan-target-matrix :deep(.t-input-number) { max-width:150px; }
.plan-target-matrix tbody th { font-weight:650; color:#2d4264; background:#fbfcff; }
.plan-target-matrix tbody td:last-child { color:#2465cd; background:#f7faff; }
.plan-target-matrix tbody tr:hover td { background:#f3f8ff; }
.plan-section :deep(.t-alert) { margin-top:18px; }
.plan-section :deep(.t-input__wrap) { max-width:520px; }
.plan-section :deep(.t-input-number) .t-input__wrap { max-width:none; }
.plan-section :deep(.t-button--variant-text) { white-space:nowrap; }
.plan-section > label { display:flex; flex-direction:column; gap:8px; }
.plan-section > label:not(.plan-name-field) { font-weight:600; }
.plan-section > p { color:#647895; }
.plan-section > h3 { margin:3px 0 18px; }
.plan-section .platform-group { border:1px solid #e3eaf4; border-radius:8px; }
.plan-section .platform-group h3 { display:flex; align-items:center; gap:10px; margin:0 0 12px; font-size:15px; }
.plan-section .platform-group .plan-split { border:1px solid #e4ebf5; border-radius:7px; }
.plan-section .platform-group .plan-split label { display:flex; gap:8px; align-items:center; }
.plan-section .platform-group .plan-split :deep(.t-input-number) { width:132px; }
.plan-section .platform-group .plan-split :deep(.t-input__wrap) { min-width:170px; }
.plan-section .platform-group .plan-split :deep(.t-input-number .t-input__wrap) { min-width:0; }
.closure-table { min-width:950px; }
.closure-table th { color:#657892; background:#f7f9fc; font-weight:600; white-space:nowrap; }
.closure-table td { color:#344866; }
.closure-table small { color:#8592a6; line-height:1.5; }
@media (max-width: 1180px) {
  .plan-workflow-grid { grid-template-columns:minmax(0,1fr); }
  .plan-workflow-rail { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); }
  .plan-workflow-rail .rail-info { grid-column:1/-1; }
}
@media (max-width: 1000px) { .plan-platform-cards { grid-template-columns:1fr; } }
@media (max-width: 700px) {
  .plan-section { padding:16px; }
  .plan-metrics { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .plan-category-pool { grid-template-columns:1fr; }
  .plan-workflow-rail { display:flex; }
}
.platform-group {
  background: var(--wt-bg-page, #f5f7fb);
  padding: 16px;
  margin: 16px 0;
}
.plan-split {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  padding: 16px;
  background: white;
  margin: 8px 0;
}
.plan-split > .t-input__wrap {
  max-width: 240px;
}
.plan-confirm-name { min-width:220px; color:#28446b; }
.target-row {
  display: flex;
  gap: 20px;
  margin: 12px 0;
  align-items: center;
}
.closure-table {
  width: 100%;
  border-collapse: collapse;
  margin: 20px 0;
  font-size: 13px;
}
.closure-table td,
.closure-table th {
  padding: 12px 8px;
  text-align: left;
  border-bottom: 1px solid #e5e9f0;
}
@media (max-width: 700px) {
  .plan-section {
    overflow: auto;
  }
  .target-row {
    flex-wrap: wrap;
  }
}
</style>
