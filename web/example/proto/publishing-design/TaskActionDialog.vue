<script setup>
import { batchCommand } from "./batchApi.js";
import { ref } from "vue";
import Overlay from "./PrototypeOverlay.vue";
import RequiredLabel from "./RequiredLabel.vue";
import {
  cancelPublication,
  verifyResult,
  processHuman,
} from "./taskActions.js";
import { canRetry } from "./model.js";
import { taskChecks } from "./taskExperience.js";
import { queueTask } from "./relationships.js";
const props = defineProps({ db: Object, persist: Function });
const visible = ref(false),
  task = ref(null),
  action = ref(""),
  error = ref(""),
  busy = ref(false),
  form = ref({});
const titles = {
  cancel: "确认取消任务",
  human: "处理发布异常",
  verify: "核实发布结果",
  retry: "重试任务",
  start: "执行任务",
  supplement: "补录公开结果",
};
function open(t, a) {
  task.value = t;
  action.value = a;
  form.value = {
    status: a === "supplement" ? "已发布" : "",
    evidence: "",
    reason: "",
    url: "",
    publicAt: "",
    human: "",
    attempt: t.attemptHistory?.at(-1)?.number,
    expectedStatus: t.status,
    expectedAttempts: t.attempts || 0,
    taskId: t.id,
    requestKey: crypto.randomUUID(),
  };
  error.value = "";
  visible.value = true;
}
async function submit() {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    const t = task.value;
    if (t.batchId) {
      if (!(await props.persist())) throw Error("当前数据尚未保存，请重试");
      const { requestKey, ...input } = form.value;
      const result = await batchCommand(
        t.batchId,
        "taskOperation",
        { ...input, action: action.value },
        requestKey,
      );
      Object.assign(props.db, result);
      visible.value = false;
      return;
    }
    const before = JSON.parse(JSON.stringify(t)),
      works = JSON.parse(JSON.stringify(props.db.works || []));
    if (action.value === "cancel") cancelPublication(t, form.value.reason);
    if (action.value === "human") processHuman(t, form.value.human);
    if (action.value === "verify")
      verifyResult(props.db, t, form.value, "未登录用户");
    if (action.value === "retry") {
      if (!canRetry(t)) throw Error("原执行结束或未发布证据不足，不能重试");
      const errors = taskChecks(t, props.db);
      if (errors.length) throw Error(errors.join("；"));
      if (!queueTask(props.db, t, { retry: true }))
        throw Error("状态已变化，未重复入队");
    }
    if (!(await props.persist())) {
      for (const key of Object.keys(t)) delete t[key];
      Object.assign(t, before);
      props.db.works = works;
      throw Error("保存失败，本次操作未生效，请重试");
    }
    visible.value = false;
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
defineExpose({ open });
</script>
<template>
  <Overlay
    v-model:visible="visible"
    :title="titles[action] || '任务操作'"
    :width="640"
    :busy="busy"
    ><template v-if="task"
      ><div class="v4-dialog-identity">
        <img :src="task.cover" alt="任务封面" />
        <div>
          <h3>{{ task.title || task.description }}</h3>
          <p>
            {{ task.platform }} ·
            {{ db.accounts.find((a) => a.id === task.accountId)?.name }} ·
            {{ task.id }}
          </p>
        </div>
      </div>
      <t-alert v-if="error" theme="warning" :message="error" />
      <template v-if="action === 'cancel'"
        ><p>
          取消后停止后续执行，保留当前成片的平台占用；原任务不可恢复。已提交的平台稿件不会被撤回。
        </p>
        <label class="v4-field"
          >取消原因（选填）<t-textarea
            v-model="form.reason"
            :maxlength="500" /></label
      ></template>
      <template v-if="action === 'retry'"
        ><p>上次失败原因：{{ task.evidence || "历史记录未保留原因" }}</p>
        <p>
          确认后重新发起一次执行，任务进入排队中；历史尝试与内容快照保留。提交时重新校验资源、内容与执行证据。
        </p></template
      >
      <template v-if="action === 'human'"
        ><p>当前阻塞：{{ task.evidence || "等待原账号登录或平台验证" }}</p>
        <p>请在对应账号的平台窗口完成登录或验证，再确认处理结果。</p>
        <t-radio-group v-model="form.human" class="v4-options"
          ><t-radio value="validated">已完成验证、未手动投稿</t-radio
          ><t-radio value="submitted">可能已提交内容</t-radio
          ><t-radio value="later">尚未完成，稍后处理</t-radio></t-radio-group
        ></template
      >
      <template v-if="['verify', 'supplement'].includes(action)"
        ><p>
          已知证据：{{ task.evidence || "尚无平台反馈" }} · 尝试
          {{ form.attempt || "缺失" }}
        </p>
        <label class="v4-field"
          ><RequiredLabel>核实结论</RequiredLabel
          ><t-select
            v-model="form.status"
            @change="error = ''"
            :options="
              action === 'supplement'
                ? [{ label: '确认已发布', value: '已发布' }]
                : [
                    { label: '确认已发布', value: '已发布' },
                    ...(task.batchId
                      ? []
                      : [{ label: '已提交平台审核中', value: '平台审核中' }]),
                    { label: '确认未发布', value: '执行失败' },
                    { label: '仍无法判断', value: '结果待确认' },
                  ]
            " /></label
        ><label v-if="form.status === '已发布'" class="v4-field"
          ><RequiredLabel>最终公开作品链接</RequiredLabel
          ><t-input
            v-model="form.url"
            placeholder="具体作品的 HTTPS 链接"
          /><small
            >请填写具体作品的公开链接，不能使用账号主页或审核页面。</small
          ></label
        ><label v-if="form.status === '已发布' && task.batchId" class="v4-field"
          ><RequiredLabel>实际发布时间</RequiredLabel
          ><t-input v-model="form.publicAt" type="datetime-local" /></label
        ><label v-if="form.status === '执行失败'" class="v4-field"
          ><RequiredLabel>失败原因</RequiredLabel
          ><t-input v-model="form.reason" /></label
        ><label v-if="form.status" class="v4-field"
          ><RequiredLabel
            :required="['平台审核中', '执行失败'].includes(form.status)"
            >{{
              form.status === "平台审核中"
                ? "投稿编号或有效审核证据"
                : form.status === "执行失败"
                  ? "核实依据"
                  : "核实备注（选填）"
            }}</RequiredLabel
          ><t-textarea v-model="form.evidence" :maxlength="2000" /></label
      ></template> </template
    ><template #footer
      ><t-button theme="default" variant="outline" @click="visible = false">{{
        action === "cancel"
          ? "保留任务"
          : action === "human"
            ? "稍后处理"
            : "取消"
      }}</t-button
      ><t-button
        :theme="action === 'cancel' ? 'danger' : 'primary'"
        :loading="busy"
        @click="submit"
        >{{
          action === "cancel"
            ? "确认取消任务"
            : action === "retry"
              ? "确认重试"
              : action === "human"
                ? "确认处理结果"
                : action === "start"
                  ? "确认执行任务"
                  : "保存核实结果"
        }}</t-button
      ></template
    ></Overlay
  >
</template>
<style scoped>
.v4-dialog-identity {
  display: flex;
  gap: 16px;
  background: var(--wt-color-bg-subtle);
  border: 1px solid var(--wt-color-border);
  padding: 16px;
  border-radius: 4px;
  margin-bottom: 24px;
}
.v4-dialog-identity img {
  width: 72px;
  height: 52px;
  object-fit: cover;
  border-radius: 4px;
}
.v4-dialog-identity h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 500;
}
.v4-dialog-identity p {
  font-size: 13px;
  color: var(--wt-color-text-secondary);
}
.v4-field {
  display: grid;
  gap: 8px;
  margin: 20px 0;
}
.v4-options {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 16px;
  margin: 20px 0;
}
small {
  color: var(--wt-color-text-secondary);
}
</style>
