<script setup>
import { computed, ref, watch } from "vue";
import { PublicationStatus } from "../status.js";
import { publicationPublishedUTC } from "../dateTime.js";
import ResourceRequiredLabel from "../../../shared/ui/resource/ResourceRequiredLabel.vue";
import ResourceDateTime from "../../../shared/ui/resource/ResourceDateTime.vue";
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
const props = defineProps({
  visible: Boolean,
  task: Object,
  busy: Boolean,
  error: String,
  management: Boolean,
  savedEvidence: Object,
});
const emit = defineEmits(["update:visible", "submit"]);
const conclusion = ref(""),
  reason = ref(""),
  basis = ref(""),
  validation = ref("");
const submissionId = ref(""),
  targetUrl = ref(""),
  workId = ref(""),
  workUrl = ref(""),
  publishedTime = ref("");
const sceneClosed = ref(false),
  noPublishableWork = ref(false),
  workMatched = ref(false),
  publiclyAccessible = ref(false);
const supplement = computed(
  () => !!props.management && props.task?.status === PublicationStatus.Reviewing,
);
const hasReceipt = computed(() =>
  [PublicationStatus.Reviewing, PublicationStatus.Published].includes(
    conclusion.value,
  ),
);
const conclusions = computed(() =>
  supplement.value
    ? [{ label: "已公开", value: PublicationStatus.Published }]
    : [
        { label: "已公开", value: PublicationStatus.Published },
        { label: "仍在审核", value: PublicationStatus.Reviewing },
        { label: "确认未发布", value: PublicationStatus.Failed },
        { label: "仍无法判断", value: PublicationStatus.Uncertain },
      ],
);
watch(
  () => [props.visible, props.task?.id],
  () => {
    if (!props.visible) return;
    conclusion.value = supplement.value ? PublicationStatus.Published : "";
    reason.value = "";
    basis.value = "";
    validation.value = "";
    submissionId.value = props.task?.platform_submission_id || "";
    targetUrl.value = props.task?.target_url || "";
    workId.value = "";
    workUrl.value = "";
    publishedTime.value = "";
    sceneClosed.value = false;
    noPublishableWork.value = false;
    workMatched.value = false;
    publiclyAccessible.value = false;
  },
  { immediate: true },
);
function submit() {
  validation.value = "";
  if (props.busy) return;
  if (props.management && (!supplement.value || conclusion.value !== PublicationStatus.Published || !props.savedEvidence?.scene_closed)) {
    validation.value = "原尝试缺少安全结束记录，请由任务所属运营先核实。";
    return;
  }
  const confirmedSceneClosed = props.management ? props.savedEvidence.scene_closed : sceneClosed.value;
  if (
    !conclusion.value ||
    !props.task?.current_attempt_id ||
    !reason.value.trim() ||
    basis.value.trim().length < 10
  ) {
    validation.value = "请选择核实结论，填写原因及实际核实依据（至少 10 字）。";
    return;
  }
  if (conclusion.value !== PublicationStatus.Uncertain && !confirmedSceneClosed) {
    validation.value = "请先核实原执行进程与浏览器已安全结束。";
    return;
  }
  if (
    conclusion.value === PublicationStatus.Failed &&
    !noPublishableWork.value
  ) {
    validation.value = "请确认原账号没有可继续发布的稿件。";
    return;
  }
  if (
    hasReceipt.value &&
    (!/^[1-9][0-9]*$/.test(submissionId.value) || !workMatched.value)
  ) {
    validation.value = "请填写指定投稿回执 ID，并确认原账号、稿件和内容一致。";
    return;
  }
  let publishedAt;
  if (conclusion.value === PublicationStatus.Published) {
    if (
      !/^[1-9][0-9]*$/.test(workId.value) ||
      !workUrl.value.trim() ||
      !publiclyAccessible.value
    ) {
      validation.value = "请填写公开作品 ID和链接，并核实无需登录即可访问。";
      return;
    }
    try {
      publishedAt = publicationPublishedUTC(publishedTime.value);
    } catch (e) {
      validation.value = e.message;
      return;
    }
  }
  const input = {
    attempt_id: props.task.current_attempt_id,
    expected_revision: props.task.revision,
    conclusion: conclusion.value,
    evidence: {
      platform_uid: props.task.platform_account_uid,
      reason: reason.value.trim(),
      basis: basis.value.trim(),
      scene_closed: confirmedSceneClosed,
      no_publishable_work:
        conclusion.value === PublicationStatus.Failed &&
        noPublishableWork.value,
      work_matched: hasReceipt.value && workMatched.value,
      publicly_accessible:
        conclusion.value === PublicationStatus.Published &&
        publiclyAccessible.value,
    },
  };
  if (hasReceipt.value) {
    input.platform_submission_id = submissionId.value;
    input.target_url = targetUrl.value.trim();
  }
  if (publishedAt) {
    input.platform_work_id = workId.value;
    input.work_url = workUrl.value.trim();
    input.published_at = publishedAt;
  }
  emit("submit", input);
}
</script>
<template>
  <ResourceOverlay
    form-actions
    :visible="visible"
    :title="supplement ? '补录公开作品' : '核实发布结果'"
    :width="680"
    :busy="busy"
    @update:visible="emit('update:visible', $event)"
  >
    <FeedbackAlert
      v-if="error || validation"
      theme="error"
      :message="error || validation"
    />
    <div class="publication-verification">
      <p>
        任务 {{ task?.id }} · 原尝试
        {{ task?.current_attempt_id }}。请核对原账号的指定稿件和执行证据。
      </p>
      <label
        ><ResourceRequiredLabel>核实结论</ResourceRequiredLabel
        ><t-select
          v-model="conclusion"
          aria-label="核实结论"
          :disabled="busy || supplement"
          :options="conclusions"
      /></label>
      <p v-if="conclusion === PublicationStatus.Failed">
        请检查原账号没有可继续发布的稿件。缺少提交标记不能单独作为未发布依据。
      </p>
      <template v-if="hasReceipt">
        <label
          ><ResourceRequiredLabel>投稿回执 ID</ResourceRequiredLabel
          ><t-input
            v-model="submissionId"
            aria-label="投稿回执 ID"
            :maxlength="255"
            :disabled="busy || !!task?.platform_submission_id"
        /></label>
        <label
          >平台目标链接<t-input
            v-model="targetUrl"
            aria-label="平台目标链接"
            :maxlength="2048"
            :disabled="busy"
            placeholder="填写平台实际提供的指定稿件链接，尚未取得可留空"
        /></label>
      </template>
      <template v-if="conclusion === PublicationStatus.Published">
        <label
          ><ResourceRequiredLabel>公开作品 ID</ResourceRequiredLabel
          ><t-input
            v-model="workId"
            aria-label="公开作品 ID"
            :maxlength="255"
            :disabled="busy"
        /></label>
        <label
          ><ResourceRequiredLabel>公开作品链接</ResourceRequiredLabel
          ><t-input
            v-model="workUrl"
            aria-label="公开作品链接"
            :maxlength="2048"
            :disabled="busy"
        /></label>
        <label
          ><ResourceRequiredLabel
            >平台实际发布时间（北京时间）</ResourceRequiredLabel
          ><ResourceDateTime
            v-model:value="publishedTime"
            seconds
            :disabled="busy"
            aria-label="平台实际发布时间"
        /></label>
      </template>
      <label
        ><ResourceRequiredLabel>核实原因</ResourceRequiredLabel
        ><t-input
          v-model="reason"
          aria-label="核实原因"
          :maxlength="200"
          :disabled="busy"
      /></label>
      <label
        ><ResourceRequiredLabel>核实依据</ResourceRequiredLabel
        ><t-textarea
          v-model="basis"
          aria-label="核实依据"
          :maxlength="2000"
          :autosize="{ minRows: 4, maxRows: 8 }"
          :disabled="busy"
          placeholder="记录如何核对原账号、指定稿件、视频及公开时间"
      /></label>
      <FeedbackAlert v-if="management" theme="info" :message="savedEvidence?.basis || '原尝试已记录现场安全结束'" />
      <t-checkbox
        v-if="!management && conclusion && conclusion !== PublicationStatus.Uncertain"
        v-model="sceneClosed"
        :disabled="busy"
        >已核实原执行进程与浏览器已安全结束</t-checkbox
      >
      <t-checkbox
        v-if="conclusion === PublicationStatus.Failed"
        v-model="noPublishableWork"
        :disabled="busy"
        >已核实原账号没有可继续发布的稿件</t-checkbox
      >
      <t-checkbox v-if="hasReceipt" v-model="workMatched" :disabled="busy"
        >已核实指定稿件与原账号、视频及发布内容一致</t-checkbox
      >
      <t-checkbox
        v-if="conclusion === PublicationStatus.Published"
        v-model="publiclyAccessible"
        :disabled="busy"
        >已核实公开作品无需登录即可访问</t-checkbox
      >
    </div>
    <template #footer
      ><t-button
        theme="default"
        variant="outline"
        :disabled="busy"
        @click="emit('update:visible', false)"
        >取消</t-button
      ><t-button :loading="busy" @click="submit">{{
        supplement ? "保存补录" : "保存核实结果"
      }}</t-button></template
    >
  </ResourceOverlay>
</template>
<style scoped>
.publication-verification {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.publication-verification label:not(.t-checkbox) {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
  font-weight: 500;
  line-height: 20px;
}
.publication-verification p {
  margin: 0;
  line-height: 1.6;
  color: var(--wt-text-secondary);
}
</style>
