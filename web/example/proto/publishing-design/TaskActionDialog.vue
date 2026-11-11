<script setup>
import { ref } from "vue";
import Overlay from "./PrototypeOverlay.vue";
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
};
function open(t, a) {
  task.value = t;
  action.value = a;
  form.value = {
    status: "",
    evidence: "",
    reason: "",
    url: "",
    human: "",
    attempt: t.attemptHistory?.at(-1)?.number,
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
    const before = JSON.parse(JSON.stringify(t)),
      works = JSON.parse(JSON.stringify(props.db.works || []));
    if (action.value === "cancel") cancelPublication(t, form.value.reason);
    if (action.value === "human") processHuman(t, form.value.human);
    if (action.value === "verify")
      verifyResult(props.db, t, form.value, "本地原型操作人（非认证身份）");
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
      throw Error("本地保存失败，未应用此次操作，请重试");
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
    :title="titles[action]"
    :width="640"
    :busy="busy"
    ><template v-if="task"
      ><div class="v4-dialog-identity">
        <img :src="task.cover" alt="任务封面" />
        <div>
          <h3>{{ task.title || task.description }}</h3>
          <p>
            {{ task.platform }} ·
            {{ db.accounts.find((a) => a.id === task.accountId)?.name }} · #{{
              task.id
            }}
          </p>
        </div>
      </div>
      <t-alert v-if="error" theme="warning" :message="error" />
      <template v-if="action === 'cancel'"
        ><p>
          停止后续执行不等于撤回平台投稿。取消成功后资源可能仍占用，原任务不可恢复。
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
        <p>
          请在原账号的官方平台会话完成处理。本地原型未连接人工接管窗口，也不会采集密码或验证码。
        </p>
        <t-radio-group v-model="form.human" class="v4-options"
          ><t-radio value="validated">已完成验证、未手动投稿</t-radio
          ><t-radio value="submitted">可能已提交内容</t-radio
          ><t-radio value="later">尚未完成，稍后处理</t-radio></t-radio-group
        ></template
      >
      <template v-if="action === 'verify'"
        ><p>
          已知证据：{{ task.evidence || "尚无平台反馈" }} · 尝试
          {{ form.attempt || "缺失" }}
        </p>
        <label class="v4-field"
          >核实结论<t-select
            v-model="form.status"
            :options="[
              { label: '确认已发布', value: '已发布' },
              { label: '已提交平台审核中', value: '平台审核中' },
              { label: '确认发布失败', value: '执行失败' },
              { label: '仍无法判断', value: '结果待确认' },
            ]" /></label
        ><label v-if="form.status === '已发布'" class="v4-field"
          >最终公开作品链接 *<t-input
            v-model="form.url"
            placeholder="具体作品的 HTTPS 链接"
          /><small
            >当前平台配置未授权无链接例外；不能使用主页或审核页面。</small
          ></label
        ><label v-if="form.status === '执行失败'" class="v4-field"
          >失败原因 *<t-input v-model="form.reason" /></label
        ><label v-if="form.status" class="v4-field"
          >{{
            form.status === "平台审核中"
              ? "投稿编号或有效审核证据 *"
              : form.status === "执行失败"
                ? "核实依据 *"
                : "核实备注（选填）"
          }}<t-textarea v-model="form.evidence" :maxlength="2000" /></label
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
  background: #f3f6fc;
  border: 1px solid #e1e8f5;
  padding: 16px;
  border-radius: 8px;
  margin-bottom: 22px;
}
.v4-dialog-identity img {
  width: 100px;
  height: 66px;
  object-fit: cover;
  border-radius: 5px;
}
.v4-dialog-identity h3 {
  margin: 0;
  font-size: 17px;
}
.v4-dialog-identity p {
  font-size: 13px;
  color: #64748b;
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
  color: #7c8799;
}
</style>
