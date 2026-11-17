<script setup>
import { ref, computed } from "vue";
import {
  initCandidate,
  editField,
  requestSuggestion,
  receiveSuggestion,
  adopt,
  contentFields,
  invalidateCover,
} from "./content.js";
import { ensureCoverResults, coverSignature, coverSpecs } from "./covers.js";
import { clone } from "./relationships.js";
import { feedback } from "../../../src/shared/ui/feedback/index.js";
import { declarationOptions } from "./model.js";
import PublishingExecutionSettings from "./PublishingExecutionSettings.vue";
const props = defineProps({ rows: Array, db: Object, single: Boolean });
const emit = defineEmits(["preview"]);
const scope = ref("全部"),
  fields = ref([...contentFields]),
  style = ref("实战攻略"),
  selected = ref([]),
  busy = ref(false),
  done = ref(0),
  total = ref(0),
  q = ref(""),
  onlyProblems = ref(false),
  page = ref(1),
  error = ref(""),
  notice = ref(""),
  activeId = ref(null);
const active = computed({
  get: () => props.rows.find((r) => r.candidateId === activeId.value),
  set: (r) => (activeId.value = r?.candidateId || null),
});
let generation = 0;
const rendering = ref(false);
const labels = {
  title: "发布标题",
  shortTitle: "封面短标题",
  topics: "话题",
  tags: "平台标签",
  description: "简介 / 正文",
};
const visible = computed(() =>
  props.rows.filter(
    (r) =>
      (r.title + r.category + r.candidateId).includes(q.value) &&
      (!onlyProblems.value || !r.contentApproved || !r.coverApproved),
  ),
);
const paged = computed(() =>
  visible.value.slice((page.value - 1) * 6, page.value * 6),
);
function rowsFor() {
  return props.rows.filter(
    (r) =>
      scope.value === "全部" ||
      (scope.value === "勾选" && selected.value.includes(r.candidateId)) ||
      (scope.value === "仅缺失" && fields.value.some((f) => !r[f])),
  );
}
async function generate(
  rows = rowsFor(),
  singleFields = fields.value,
  retry = false,
) {
  const run = ++generation;
  if (!singleFields.length || !rows.length) {
    error.value = "请选择候选范围和至少一个生成字段";
    return;
  }
  busy.value = true;
  done.value = 0;
  total.value = rows.length;
  error.value = "";
  notice.value = "";
  await Promise.all(
    rows.map(async (r, i) => {
      const req = requestSuggestion(r, singleFields);
      r.ai.progress = "生成中";
      receiveSuggestion(
        r,
        req,
        {},
        "暂时无法获取 AI 建议，请手动填写或稍后重试。",
      );
      if (req.fields.some((f) => r.ai.latest[f] === req.token))
        r.ai.progress = "生成失败";
      if (run === generation) done.value++;
    }),
  );
  if (run === generation) busy.value = false;
}
function adoptAll() {
  let count = 0;
  for (const r of rowsFor())
    for (const f of fields.value) if (adopt(r, f)) count++;
  notice.value = `已采用 ${count} 个有效字段；人工修改、失败和过期建议均保留。`;
}
function adoptField(r, f) {
  if (adopt(r, f)) notice.value = "已采用" + labels[f];
  else error.value = "建议已失败或过期，请重新生成当前字段";
}
function setTemplate(r, id) {
  const t = props.db.templates.find((t) => t.id === Number(id));
  editField(r, "templateId", t?.id || null);
  r.templateVersion = t?.version || 1;
  if (t?.document.image && !r.imageId)
    editField(r, "coverSource", t.document.image);
  r.templateDocument = t ? clone(t.document) : null;
  r.templateDocuments = {};
  r.templateSnapshots = {};
  for (const spec of coverSpecs(r.platform)) {
    const same = [t, ...props.db.templates].find(
      (t) =>
        t?.enabled &&
        Math.abs(
          t.document.width / t.document.height - spec.width / spec.height,
        ) < 0.01,
    );
    if (same) {
      r.templateDocuments[spec.key] = clone(same.document);
      r.templateSnapshots[spec.key] = {
        id: same.id,
        version: same.version || 1,
      };
    }
  }
  invalidateCover(r);
}
function random(r) {
  const pool = props.db.images.filter(
    (i) => i.enabled && i.uses.includes("底图"),
  );
  const exact = pool.filter((i) => i.category === r.category);
  const candidates = exact.length
    ? exact
    : pool.filter((i) => i.category === "通用");
  if (!candidates.length) {
    error.value = "无可用底图，请进入图片库上传对应游戏或通用底图";
    return;
  }
  const image = candidates[Math.floor(Math.random() * candidates.length)];
  editField(r, "coverSource", image.src);
  r.imageId = image.id;
  r.imageVersion = image.version || 1;
}
async function render(rows = rowsFor()) {
  rendering.value = true;
  error.value = "";
  try {
    for (const r of rows) {
      try {
        await ensureCoverResults(r);
      } catch (e) {
        error.value = r.candidateId + "：" + e.message;
      }
    }
  } finally {
    rendering.value = false;
  }
}
function approve(r) {
  if (r.coverSignature !== coverSignature(r) || r.coverRender !== "已合成") {
    error.value = "请先重新合成并检查当前版本封面";
    return;
  }
  r.contentApproved = true;
  r.coverApproved = true;
  r.coverError = "";
}
async function approveSelected() {
  const rows = rowsFor();
  if (
    rows.some(
      (r) =>
        r.coverSignature !== coverSignature(r) || r.coverRender !== "已合成",
    )
  ) {
    error.value = "范围中存在过期或未合成封面，请修复后审核";
    return;
  }
  if (
    await feedback.confirmWarning({
      title: "确认当前版本人工审核",
      message: `已逐条检查这 ${rows.length} 条内容与实际封面，声明符合本次内容事实。修改后必须重新审核。`,
      confirmText: "确认审核 " + rows.length + " 条",
    })
  )
    rows.forEach(approve);
}
function inspect(r) {
  initCandidate(r);
  active.value = r;
}
props.rows.forEach(initCandidate);
if (props.single) active.value = props.rows[0] || null;
</script>
<template>
  <section class="ai-workspace">
    <div class="ai-heading"><span>CONTENT GENERATION</span><h3>{{ single ? '配置这条任务的内容与封面' : 'AI 批量内容配置工作台' }}</h3><p>{{ single ? 'AI 只为当前任务生成建议；人工确认声明、成品封面和发布时机。' : '按字段生成建议，逐条核对文案与封面后再创建任务。' }}</p></div>
    <div v-if="single && active" class="single-ai-toolbar"><t-button :loading="busy" @click="generate([active])">为这条任务生成建议</t-button><t-button variant="outline" @click="adoptAll">采用未修改字段建议</t-button><span>生成进度 {{ done }} / {{ total }}</span></div>
    <template v-if="!single">
    <div class="ai-tools">
      <label
        >作用范围<select v-model="scope">
          <option>全部</option>
          <option>勾选</option>
          <option>仅缺失</option>
        </select></label
      ><label
        >文案风格<select v-model="style">
          <option>实战攻略</option>
          <option>简洁</option>
        </select></label
      ><label v-for="f in contentFields" :key="f"
        ><input type="checkbox" v-model="fields" :value="f" />{{
          labels[f]
        }}</label
      >
    </div>
    <p>
      覆盖策略：先生成建议，仅当前输入及字段版本可采用；已手改字段保留。进度
      {{ done }} / {{ total }}
    </p>
    <div class="ai-tools">
      <t-button :loading="busy" @click="generate()">一次生成所选字段</t-button
      ><t-button variant="outline" @click="adoptAll"
        >采用未修改字段建议</t-button
      ><t-button
        variant="outline"
        @click="
          generate(
            rowsFor().filter((r) =>
              Object.values(r.ai?.suggestions || {}).some((s) => s.error),
            ),
            fields,
            true,
          )
        "
        >仅重试失败项</t-button
      ><t-button variant="outline" :loading="rendering" @click="render()"
        >批量合成真实封面</t-button
      ><t-button
        variant="outline"
        :disabled="rendering"
        @click="approveSelected"
        >审核当前范围的内容与封面</t-button
      >
    </div>
    <t-alert v-if="error" theme="warning" :message="error" />
    <t-alert v-if="notice" theme="info" :message="notice" />
    <div class="ai-tools">
      <t-input v-model="q" placeholder="搜索候选标题 / 类目" /><label
        ><input
          type="checkbox"
          v-model="onlyProblems"
          @change="page = 1"
        />只看问题项</label
      >
    </div>
    <div class="ai-table-wrap resource-data-table resource-embedded-table is-editable">
      <table class="ai-table">
        <thead>
          <tr>
            <th>选择 / 候选</th>
            <th>发布标题</th>
            <th>独立封面短标题</th>
            <th>话题 / 标签</th>
            <th>实际封面</th>
            <th>审核 / 操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in paged" :key="r.candidateId">
            <td>
              <input
                type="checkbox"
                v-model="selected"
                :value="r.candidateId"
              /><small
                >{{ r.candidateId.slice(0, 8) }} · 成片 {{ r.assetId }}</small
              ><span>{{ r.category }}</span>
            </td>
            <td>
              <input
                :value="r.title"
                aria-label="候选发布标题"
                @input="editField(r, 'title', $event.target.value)"
              /><small>{{ r.ai?.progress || "未生成" }}</small>
            </td>
            <td>
              <input
                :value="r.shortTitle"
                aria-label="候选封面短标题"
                @input="editField(r, 'shortTitle', $event.target.value)"
              />
            </td>
            <td>
              {{ r.topics }}<small>{{ r.tags }}</small>
            </td>
            <td>
              <button class="cover-button" @click="emit('preview', { row: r })">
                <img :src="r.cover" alt="封面实际渲染结果" /></button
              ><small
                >{{ r.coverRender || "待合成" }} /
                {{ r.coverApproved ? "已审核" : "待审核" }}</small
              >
            </td>
            <td>
              <t-button variant="text" @click="inspect(r)"
                >内容 / 建议 / 封面</t-button
              ><small>{{
                r.contentApproved ? "内容已审核" : "内容待审核"
              }}</small>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="ai-tools">
      <t-button variant="outline" :disabled="page <= 1" @click="page--"
        >上一页</t-button
      ><span
        >{{ page }} / {{ Math.max(1, Math.ceil(visible.length / 6)) }} · 共
        {{ visible.length }}</span
      ><t-button
        variant="outline"
        :disabled="page * 6 >= visible.length"
        @click="page++"
        >下一页</t-button
      >
    </div>
    </template>
    <div v-if="active" class="candidate-panel">
      <h3>{{ single ? '发布文案与逐字段建议' : `候选 ${active.candidateId.slice(0, 8)} · 逐字段建议与封面` }}</h3>
      <t-button v-if="!single" variant="text" @click="active = null">收起本条</t-button>
      <div v-for="f in contentFields" :key="f" class="suggestion">
        <label
          >{{ labels[f]
          }}<input
            :value="active[f]"
            @input="editField(active, f, $event.target.value)"
            :aria-label="'编辑' + labels[f]"
        /></label>
        <p v-if="active.ai?.suggestions?.[f]">
          建议：{{
            active.ai.suggestions[f].value || active.ai.suggestions[f].error
          }}
          ·
          {{
            active.ai.suggestions[f].expired
              ? "已过期"
              : active.ai.suggestions[f].status
          }}<span v-if="active.ai.suggestions[f].error" role="alert">
            · {{ active.ai.suggestions[f].error }}</span
          ><t-button
            variant="text"
            :disabled="
              !!active.ai.suggestions[f].error ||
              active.ai.suggestions[f].expired ||
              active.ai.suggestions[f].status === '已采用'
            "
            @click="adoptField(active, f)"
            >采用{{ labels[f] }}</t-button
          >
        </p>
        <t-button variant="text" @click="generate([active], [f])"
          >重生{{ labels[f] }}</t-button
        >
      </div>
      <div v-if="single" class="single-platform-fields"><h3>平台设置</h3><div class="single-platform-grid"><label>创作声明 *<t-select :value="active.declaration" :options="declarationOptions(active.platform)" placeholder="按内容事实选择" @change="(value) => editField(active, 'declaration', value)" /></label><label v-if="active.platform === 'B站'">分区 *<t-select :value="active.partition" :options="[{ label: '游戏', value: '游戏' }]" @change="(value) => editField(active, 'partition', value)" /></label><label v-if="active.platform !== '百家号'">可见范围<t-select :value="active.visibility || '公开'" :options="[{ label: '公开', value: '公开' }, { label: '仅自己可见', value: '仅自己可见' }]" @change="(value) => editField(active, 'visibility', value)" /></label></div></div>
      <div class="ai-tools">
        <label
          >封面模式<select
            :value="active.coverMode"
            @change="editField(active, 'coverMode', $event.target.value)"
          >
            <option>现成封面</option>
            <option>模板合成</option>
          </select></label
        ><label
          >图片<select
            :value="active.imageId || ''"
            @change="
              (i) => {
                const image = db.images.find(
                  (x) => x.id === Number(i.target.value),
                );
                if (image) {
                  editField(active, 'coverSource', image.src);
                  active.imageId = image.id;
                  active.imageVersion = image.version || 1;
                }
              }
            "
          >
            <option value="">素材自带封面</option>
            <option
              v-for="im in db.images.filter(
                (i) =>
                  i.enabled &&
                  i.uses.includes(
                    active.coverMode === '模板合成' ? '底图' : '现成封面',
                  ),
              )"
              :value="im.id"
              :key="im.id"
            >
              {{ im.name }}
            </option>
          </select></label
        ><t-button variant="outline" @click="random(active)"
          >按游戏换一张底图</t-button
        ><label v-if="active.coverMode === '模板合成'"
          >模板<select
            :value="active.templateId || ''"
            @change="setTemplate(active, $event.target.value)"
          >
            <option value="">请选择模板</option>
            <option
              v-for="t in db.templates.filter((t) => t.enabled)"
              :key="t.id"
              :value="t.id"
            >
              {{ t.name }} · v{{ t.version || 1 }}
            </option>
          </select></label
        >
      </div>
      <p v-if="active.coverMode !== '模板合成'" class="cover-mode-note">现成封面保留图片原有文字；AI 封面短标题不会写入图片。需要把短标题印在封面上时，请切换为“模板合成”并选择模板。</p>
      <p v-if="active.platform === '百家号'">
        百家号精确封面比例待核验，当前按模板画布或原图预览。
      </p>
      <p v-if="active.coverError">{{ active.coverError }}</p>
      <div v-if="single" class="single-cover-check"><button @click="emit('preview', { row: active })"><img :src="active.cover" alt="当前封面" /></button><div><strong>{{ active.coverRender || '待合成' }} · {{ active.coverApproved ? '当前版本已审核' : '待审核' }}</strong><p>更换图片、模板或短标题后需要重新合成并审核。</p></div></div>
      <div class="ai-tools">
        <t-button @click="render([active])">合成当前版本</t-button
        ><t-button variant="outline" @click="emit('preview', { row: active })"
          >检查实际成品</t-button
        ><t-button variant="outline" @click="approve(active)"
          >确认内容与封面已审核</t-button
        >
      </div>
      <PublishingExecutionSettings v-if="single" :row="active" @change="(field, value) => editField(active, field, value)" />
    </div>
  </section>
</template>
<style scoped>
.ai-workspace {
  margin: 0 0 16px;
  background: #fff;
  border: 1px solid #e2e9f3;
  padding: 22px;
  border-radius: 10px;
  box-shadow:0 3px 14px #24446b0a;
}
.ai-heading { margin-bottom:14px; }
.ai-heading span { color:#3976d6; font-size:10px; font-weight:700; letter-spacing:.1em; }
.ai-heading h3 { margin:4px 0; font-size:18px; color:#20385e; }
.ai-heading p { margin:0; font-size:12px; color:#7d8ca2; }
.single-ai-toolbar { display:flex; flex-wrap:wrap; gap:10px; align-items:center; margin:17px 0; }
.single-ai-toolbar span { color:#7487a0; font-size:12px; }
.single-platform-fields { margin:22px 0 0; padding-top:18px; border-top:1px solid #e2e9f3; }
.cover-mode-note { margin:10px 0 14px; padding:10px 12px; border-radius:6px; background:#eef5ff; color:#54749e; font-size:12px; line-height:1.6; }
.single-platform-fields h3 { margin:0 0 13px; color:#263d60; font-size:15px; }
.single-platform-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:15px; }
.single-platform-grid label { display:flex; flex-direction:column; gap:8px; min-width:0; color:#5f718e; font-size:12px; }
.single-cover-check { display:flex; align-items:center; gap:15px; padding:12px; border:1px solid #e2e9f3; border-radius:8px; background:#fff; }
.single-cover-check button { width:120px; flex:none; border:0; padding:0; background:none; cursor:zoom-in; }
.single-cover-check img { display:block; width:100%; aspect-ratio:4/3; object-fit:cover; border-radius:5px; }
.single-cover-check strong { font-size:13px; color:#31557f; }
.single-cover-check p { margin:5px 0 0; color:#788ba3; font-size:12px; }
.ai-tools {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  align-items: center;
  margin: 16px 0;
}
.ai-tools label { display:inline-flex; align-items:center; gap:7px; color:#5f718e; font-size:12px; }
.ai-tools select { min-height:34px; background:#fff; }
.ai-workspace > p { color:#71829a; font-size:12px; }
.ai-workspace > .t-alert { margin:11px 0; }
.ai-tools > .t-input__wrap {
  max-width: 320px;
}
.ai-workspace select,
.ai-workspace input:not([type="checkbox"]) {
  padding: 8px;
  border: 1px solid #d5deea;
  border-radius: 5px;
  max-width: 100%;
}
.ai-table-wrap {
  overflow: auto;
}
.ai-table {
  width: 100%;
  min-width: 950px;
  border-collapse: collapse;
}
.ai-table th { background:#f7f9fd; color:#657892; font-size:12px; font-weight:600; white-space:nowrap; }
.ai-table tr:hover td { background:#fbfdff; }
.ai-table td,
.ai-table th {
  padding: 12px 8px;
  border-bottom: 1px solid #e5e9f0;
  text-align: left;
  max-width: 230px;
}
.ai-table small {
  display: block;
  color: #7a879b;
  font-size: 12px;
  margin-top: 5px;
}
.cover-button {
  border: 0;
  background: transparent;
  padding: 0;
}
.cover-button img {
  width: 100px;
  max-height: 80px;
  object-fit: contain;
}
.cover-button { cursor:zoom-in; border-radius:5px; overflow:hidden; }
.candidate-panel {
  border: 1px solid #d8e4fb;
  background: #f7faff;
  padding: 20px;
  margin-top: 20px;
  border-radius:8px;
}
.candidate-panel h3 { margin:0 0 10px; color:#244977; font-size:16px; }
.candidate-panel:has(.single-platform-fields) { background:#fff; border-color:#e2e9f3; border-radius:9px; }
@media (max-width:700px) { .single-platform-grid { grid-template-columns:1fr; } }
.suggestion {
  padding: 12px 0;
  border-bottom: 1px solid #e5e9f0;
}
.suggestion label {
  display: flex;
  gap: 16px;
  align-items: center;
}
.suggestion input {
  flex: 1;
}
</style>
