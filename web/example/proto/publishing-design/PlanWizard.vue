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
const inventory = computed(() => platformInventory(props.db, now.value));
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
          categories
            .slice(0, 2)
            .map((category) => ({ platform, category, count: 0 })),
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
    <section class="plan-section" v-if="step === 0">
      <h3>资源盘点 · {{ businessDate(now) }} · Asia/Shanghai</h3>
      <p>
        按确认公开时间统计；共享账号只计一次。无真实额度显示容量待核实。刷新：{{
          now
        }}
      </p>
      <t-button variant="outline" @click="now = new Date().toISOString()"
        >刷新资源</t-button
      >
      <table class="closure-table">
        <thead>
          <tr>
            <th>平台</th>
            <th>账号总 / 可用</th>
            <th>优质 / 普通 / 新号</th>
            <th>待登录</th>
            <th>今日公开 / 去重账号</th>
            <th>今日已安排</th>
            <th>今日剩余额度 / 联合可安排估计</th>
            <th>可用成片</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in inventory" :key="r.platform">
            <td>{{ r.platform }}</td>
            <td>{{ r.total }} / {{ r.available }}</td>
            <td>{{ r.quality }}</td>
            <td>{{ r.invalid }}</td>
            <td>{{ r.published }} / {{ r.publishAccounts }}</td>
            <td>{{ r.scheduled }}</td>
            <td>{{ r.capacity ?? "待核实" }} / {{ r.estimate ?? "待核实" }}</td>
            <td>
              {{ r.free
              }}<small
                v-for="c in r.byCategory"
                :key="c.category"
                style="display: block"
                >{{ c.category }}：可用 {{ c.free }} / 占用
                {{ c.occupied }}；合格账号 {{ c.eligibleAccounts }}，估计
                {{ c.estimate ?? "待核实" }}</small
              >
            </td>
          </tr>
        </tbody>
      </table>
      <h3>类目成片池 · 去重</h3>
      <p v-for="c in categories" :key="c">
        {{ c }}：{{ db.assets.filter((a) => a.category === c).length }} 条 /
        优质
        {{
          db.assets.filter((a) => a.category === c && a.quality === "优质")
            .length
        }}
        条
      </p>
      <p>
        各类目成片数不能直接相加为可执行额度；账号每日额度、平台接收限制未核实时不承诺容量。
      </p>
    </section>
    <section class="plan-section" v-if="step === 1">
      <label
        >计划名称 *<t-input v-model="draft.name" aria-label="计划名称"
      /></label>
      <h3>平台 × 类目目标 · 总目标 {{ total }}</h3>
      <div
        class="target-row"
        v-for="t in draft.targets"
        :key="t.platform + t.category"
      >
        <span>{{ t.platform }} / {{ t.category }}</span
        ><t-input-number
          v-model="t.count"
          :min="0"
          :decimal-places="0"
          :aria-label="t.platform + t.category + '目标'"
        /><small>目标为规划愿望，超过资源的缺口需在批次配置修复</small>
      </div>
    </section>
    <section class="plan-section" v-if="step >= 2">
      <label v-if="step === 3"
        >计划名称：{{ draft.name }} · 总目标 {{ total }}</label
      >
      <section
        v-for="(p, pi) in draft.platformOrder"
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
          <t-button variant="text" @click="add(p)">新增批次</t-button
          ><t-button
            variant="text"
            :disabled="pi === 0"
            @click="
              [draft.platformOrder[pi - 1], draft.platformOrder[pi]] = [
                draft.platformOrder[pi],
                draft.platformOrder[pi - 1],
              ]
            "
            >平台组上移</t-button
          >
        </h3>
        <div
          class="plan-split"
          v-for="b in draft.splits.filter((b) => b.platform === p)"
          :key="b.id || b.name"
        >
          <t-input v-model="b.name" aria-label="批次名称" /><label
            v-for="t in b.targets"
            :key="t.category"
            >{{ t.category
            }}<t-input-number
              v-model="t.count"
              :min="0"
              :decimal-places="0"
              :aria-label="b.name + t.category + '分目标'" /></label
          ><t-button variant="text" @click="move(b, -1)">上移</t-button
          ><t-button variant="text" @click="move(b, 1)">下移</t-button
          ><t-button
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
.plan-section {
  margin: 24px 0;
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
