<script setup>
import { batchActionLabel } from "../batchPresentation.js";
import { computed, ref, watch } from "vue";
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import { feedback } from "../../../shared/ui/feedback/index.js";
import { cloudResourceURL } from "../../../shared/api/http.js";
import { createPublicationBatchClient } from "../../../shared/api/publication.js";
import { createPublicationPlanClient } from "../../../shared/api/publicationPlan.js";
import { createSessionClient } from "../../../shared/api/session.js";
import { canRunLocalAgent } from "../../../utils.js";
import { publicationStatuses } from "../status.js";
import {
  batchCommandJournal, batchControlLabels, buildBatchControl, buildBatchStart, readBatchFailures, sendBatchCommand,
} from "../batchExecution.js";

const props = defineProps({ batch: Object, actor: Object, rowLabel: Function, categoryName: Function, taskLink: Function, inline: Boolean });
const emit = defineEmits(["updated", "busy", "pending"]);
const api = createPublicationBatchClient();
const session = createSessionClient();
const owner = computed(() => canRunLocalAgent(props.actor) && String(props.actor?.id) === props.batch.operator_id);
const actions = computed(() => owner.value ? Object.keys(batchControlLabels).filter(key => props.batch.allowed_actions?.includes(key)) : []);
const action = ref("");
const visible = ref(false);
const waiting = ref(false);
const reason = ref("");
const error = ref("");
const storageError = ref("");
const source = ref(null);
const originalPreview = ref(null);
const preview = ref(null);
const included = ref([]);
const accepted = ref(false);
const failedTasks = ref([]);
const failuresLoaded = ref(false);
const pending = ref(null);
const refreshRequired = ref(false);
let journal;
let sequence = 0;

const hasShortfall = computed(() => preview.value?.shortfalls?.some(item => item.shortfall > 0));
const canStart = computed(() => !!preview.value?.can_start && included.value.length > 0 &&
  preview.value.rows?.length === included.value.length &&
  preview.value.rows.every(row => included.value.includes(row.row_key) && row.ready) &&
  (!hasShortfall.value || accepted.value));
const pendingLabel = computed(() => pending.value?.action === "start" ? "核对原启动请求" : "核对原继续请求");
const options = computed(() => (originalPreview.value?.rows || []).filter(row => row.ready).map(({ row_key }) => ({
  value: row_key, label: candidateLabel(row_key),
})));

function candidateLabel(key) {
  const candidate = source.value?.config?.candidates?.find(row => row.row_key === key);
  return candidate ? props.rowLabel(candidate) : "原候选";
}
function setBusy(value) {
  waiting.value = value;
  emit("busy", value);
}
watch(() => [props.batch.id, props.actor?.id, owner.value], () => {
  sequence++;
  journal = undefined;
  pending.value = null;
  storageError.value = "";
  if (owner.value) {
    try {
      journal = batchCommandJournal(new URL(cloudResourceURL("/api/v1")).origin,
        window.__WT_MEDIA_APP__ || "cloud", String(props.actor.id), props.batch.id);
      pending.value = journal.read();
    } catch {
      storageError.value = "无法读取待核对的批次请求。请恢复客户端存储后重新打开。";
    }
  }
  emit("pending", !!pending.value);
}, { immediate: true });

async function checkIdentity() {
  const current = await session.me();
  if (!canRunLocalAgent(current) || String(current.id) !== String(props.actor?.id))
    throw new Error("登录用户已变化，请重新打开批次");
}
function previewProblem(failure, fallback) {
  const issues = failure?.details?.issues;
  return Array.isArray(issues) && issues.length
    ? issues.map(issue => [issue.row_key && candidateLabel(issue.row_key), issue.message].filter(Boolean).join(" · ")).join("\n")
    : failure?.message || fallback;
}
async function readPreview(keys, initial = false) {
  const current = source.value;
  const run = ++sequence;
  preview.value = null;
  accepted.value = false;
  if (!keys.length) return;
  setBusy(true);
  error.value = "";
  try {
    const result = await api.executionPreview(current.id, { expected_revision: current.revision, row_keys: keys });
    if (run !== sequence) return;
    if (result?.batch_id !== current.id || result.revision !== current.revision ||
      !Array.isArray(result.rows) || !Array.isArray(result.shortfalls) ||
      result.rows.length !== keys.length || !keys.every(key => result.rows.some(row => row.row_key === key)))
      throw new Error("启动检查与当前候选不一致，请重新读取批次后检查");
    if (initial) originalPreview.value = result;
    preview.value = result;
  } catch (failure) {
    if (run === sequence) error.value = previewProblem(failure, "启动检查失败");
  } finally {
    if (run === sequence) setBusy(false);
  }
}

async function open(mode) {
  if (waiting.value || pending.value || !actions.value.includes(mode)) return;
  if (["start", "resume"].includes(mode) && storageError.value) return;
  action.value = mode;
  refreshRequired.value = false;
  source.value = props.batch;
  reason.value = "";
  failedTasks.value = [];
  failuresLoaded.value = false;
  error.value = "";
  preview.value = null;
  originalPreview.value = null;
  included.value = [];
  accepted.value = false;
  if (mode === "start") {
    const keys = source.value.config?.candidates?.map(row => row.row_key) || [];
    if (!keys.length) {
      visible.value = true;
      error.value = "批次已保存，尚无发布候选。请完善配置后开始。";
      return;
    }
    included.value = keys;
    await readPreview(keys, true);
    if (preview.value && canStart.value) {
      await submit();
      return;
    }
    visible.value = true;
    const eligible = originalPreview.value?.rows?.filter(row => row.ready).map(row => row.row_key) || [];
    if (originalPreview.value && eligible.length !== keys.length) await changeScope(eligible);
    return;
  }
  visible.value = true;
  if (mode === "finish") await loadFailures();
}
defineExpose({ open });

async function changeScope(keys) {
  included.value = keys;
  await readPreview(keys);
}
async function loadFailures() {
  setBusy(true);
  error.value = "";
  failuresLoaded.value = false;
  try {
    failedTasks.value = await readBatchFailures(api, source.value.id);
    failuresLoaded.value = true;
  } catch (failure) {
    error.value = previewProblem(failure, "无法读取当前失败任务");
  } finally {
    setBusy(false);
  }
}

const disabled = computed(() => waiting.value || refreshRequired.value ||
  (action.value === "start" ? !canStart.value :
    action.value === "resume" ? !source.value?.execution?.pause :
      !reason.value.trim() || (action.value === "finish" && !failuresLoaded.value)));

async function remember(operation) {
  if (!journal || storageError.value) throw new Error("当前客户端无法保留待核对的批次请求");
  try {
    await journal.save(operation);
  } finally {
    pending.value = journal.read();
    emit("pending", !!pending.value);
  }
}
async function clear(operation) {
  if (!["start", "resume"].includes(operation.action)) return;
  try {
    await journal.clear(operation);
    pending.value = journal.read();
  } catch {
    storageError.value = "待核对的请求未能清除，请检查客户端存储。";
  }
  emit("pending", !!pending.value);
}

async function send(operation, retry = false) {
  const id = source.value.id;
  setBusy(true);
  error.value = "";
  try {
    await checkIdentity();
    if (["start", "resume"].includes(operation.action) && !retry) await remember(operation);
    const result = await sendBatchCommand(api, id, operation, { retry });
    await clear(operation);
    visible.value = false;
    emit("updated");
    feedback.toast.success({
      start: result.recovered ? "已核对原启动结果" : "已开始发布，正式任务与执行设置已固定",
      pause: "已暂停后续派发", resume: "已继续发布", stop: "已终止后续派发",
      finish: "批次已结束，失败记录与成片占用已保留",
    }[operation.action]);
  } catch (failure) {
    if (failure.batchRejection) await clear(operation);
    if (failure.latest || failure.batchRejection || failure.outcomeUnknown) emit("updated", failure.latest);
    error.value = previewProblem(failure, "批次操作未完成");
    if (failure.batchRejection || (failure.outcomeUnknown && !pending.value)) refreshRequired.value = true;
    if (operation.action === "start" && failure.batchRejection && !failure.supersededStart && !failure.latest?.execution?.first_started_at)
      error.value = "批次已保存。启动未获确认，请核对当前 Cloud 状态。\n" + error.value;
    if (operation.action === "start" && failure.batchRejection) {
      preview.value = null;
      accepted.value = false;
    }
    visible.value = true;
  } finally {
    setBusy(false);
  }
}

async function submit() {
  if (action.value === "takeover") {
    if (disabled.value) return;
    setBusy(true);
    try {
      await checkIdentity();
      const current = await createPublicationPlanClient().get(source.value.plan_id);
      await createPublicationPlanClient().takeover(source.value.plan_id, {expected_revision:current.plan.revision,current_batch_id:source.value.id,expected_batch_revision:source.value.revision,reason:reason.value.trim()});
      visible.value=false;emit("updated",await api.get(source.value.id));
    } catch (failure) {error.value=previewProblem(failure,"接管结果待核对，请读取当前计划");} finally {setBusy(false);}
    return;
  }
  if (disabled.value) return;
  let input;
  try {
    input = action.value === "start"
      ? buildBatchStart(source.value, preview.value, crypto.randomUUID(), !!hasShortfall.value && accepted.value)
      : buildBatchControl(source.value, action.value, { reason: reason.value, failures: failedTasks.value, requestKey: crypto.randomUUID() });
  } catch (failure) {
    error.value = previewProblem(failure, "请求内容已变化");
    visible.value = true;
    return;
  }
  return send({ action: action.value, input });
}
function recover() {
  if (waiting.value || !pending.value) return;
  source.value = props.batch;
  action.value = pending.value.action;
  refreshRequired.value = false;
  return send(pending.value, true);
}
function refreshStart() {
  emit("updated");
  visible.value = false;
}
</script>

<template>
  <span v-if="storageError" class="resource-footer-status batch-muted">{{ storageError }}</span>
  <t-button v-if="pending" theme="primary" :loading="waiting" @click="recover">{{ pendingLabel }}</t-button>
  <template v-else>
    <t-button v-for="item in actions" :key="item" :disabled="waiting || (!!storageError && ['start','resume'].includes(item))"
      :theme="item === 'stop' ? 'danger' : ['start','resume','pause'].includes(item) ? 'primary' : 'default'"
      :variant="inline ? 'text' : ['start','resume','pause'].includes(item) ? 'base' : 'outline'"
      @click="open(item)">{{ batchActionLabel(item,batch) }}</t-button>
  </template>

  <ResourceOverlay :visible="visible" :title="batchActionLabel(action,source || batch) || '批次操作'"
    form-actions :width="640" :busy="waiting" @update:visible="visible = $event">
    <div v-if="source" class="batch-stack">
      <strong>{{ source.name }}</strong>
      <FeedbackAlert v-if="error" :message="error" theme="warning" />
      <p v-if="pending" class="batch-muted">原请求已保留。继续核对时会先读取当前批次，再决定能否使用原请求。</p>
      <template v-else-if="action === 'start'">
        <template v-if="originalPreview">
          <div class="batch-inline">
            <span>原目标 {{ source.targets.reduce((sum, item) => sum + BigInt(item.target_count), 0n).toString() }} 条</span>
            <span>本次发布 {{ included.length }} 条</span>
          </div>
          <div v-if="options.length">
            <t-checkbox-group :value="included" :options="options" aria-label="本次发布候选"
              :disabled="waiting" @change="changeScope" />
          </div>
          <ul v-if="originalPreview.rows.some(row => !row.ready)" class="batch-issues">
            <li v-for="row in originalPreview.rows.filter(item => !item.ready)" :key="row.row_key">
              {{ candidateLabel(row.row_key) }} · {{ row.issues?.map(issue => issue.message).join("、") || "待处理" }}
            </li>
          </ul>
          <ul v-if="preview && !preview.can_start && preview !== originalPreview" class="batch-issues">
            <li v-for="row in preview.rows.filter(item => !item.ready)" :key="row.row_key">
              {{ candidateLabel(row.row_key) }} · {{ row.issues?.map(issue => issue.message).join("、") || "待处理" }}
            </li>
          </ul>
          <ul v-if="preview" class="batch-categories">
            <li v-for="item in preview.shortfalls" :key="item.category_id"><strong>{{ categoryName(item.category_id) }}</strong>
              <span>目标 {{ item.target_count }} · 本次发布 {{ item.created_count }} · 缺口 {{ item.shortfall }}</span>
            </li>
          </ul>
          <t-checkbox v-if="hasShortfall" v-model="accepted" :disabled="waiting">确认按所选范围发布并保留目标缺口</t-checkbox>
        </template>
      </template>
      <template v-else-if="action === 'resume'">
        <p>{{ source.execution?.pause?.reason }}</p>
        <p>确认已核对本次暂停问题后继续，待办和原尝试记录会保留。</p>
        <ul v-if="source.execution?.pause?.exceptions?.length" class="batch-issues">
          <li v-for="item in source.execution.pause.exceptions" :key="item.task_id">
            <RouterLink :to="taskLink(item.task_id)">任务 {{ item.task_id }} · {{ publicationStatuses[item.status] }}</RouterLink>
          </li>
        </ul>
      </template>
      <template v-else>
        <p v-if="action === 'pause'">暂停后续派发，已开始的任务继续安全收尾。</p>
        <p v-if="action === 'stop'">{{ source.execution?.first_started_at
          ? "停止后续派发。未领取任务将取消并保留成片占用，在途任务仍按原尝试收尾。"
          : "终止已保存的配置。批次尚未生成正式任务。" }}</p>
        <p v-if="action === 'finish'">保留 {{ failedTasks.length }} 条明确失败及成片占用。结束后本批无法重试。</p>
        <t-button v-if="action === 'finish' && !failuresLoaded && !waiting" variant="text" @click="loadFailures">重新读取失败任务</t-button>
        <label>{{ batchActionLabel(action,source || batch) }}原因
          <t-textarea v-model="reason" :maxlength="200" :disabled="waiting" :aria-label="batchActionLabel(action,source || batch) + '原因'" />
        </label>
      </template>
    </div>
    <template #footer>
      <t-button theme="default" variant="outline" :disabled="waiting" @click="visible = false">返回</t-button>
      <t-button v-if="pending" theme="primary" :loading="waiting" @click="recover">{{ pendingLabel }}</t-button>
      <t-button v-else-if="refreshRequired || (action === 'start' && !preview && error)" theme="default" variant="outline"
        :disabled="waiting" @click="refreshStart">刷新批次</t-button>
      <t-button v-else :loading="waiting" :disabled="disabled"
        :theme="action === 'stop' ? 'danger' : 'primary'" @click="submit">确认{{ batchActionLabel(action,source || batch) }}</t-button>
    </template>
  </ResourceOverlay>
</template>
