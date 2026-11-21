<script setup>
import { ref, watch } from "vue";
import { CloudUploadIcon } from "tdesign-icons-vue-next";
import { createImageClient } from "../../../shared/api/image.js";
import { createFilesClient } from "../../../shared/api/files.js";
import ResourceOverlay from "../../../shared/ui/resource/ResourceOverlay.vue";
import ResourceForm from "../../../shared/ui/resource/ResourceForm.vue";
import FeedbackAlert from "../../../shared/ui/feedback/FeedbackAlert.vue";
import { feedback } from "../../../shared/ui/feedback/index.js";
import { createImageUpload } from "../uploadImage.js";
const props = defineProps({
  visible: Boolean,
  item: Object,
  categories: Array,
  teams: Array,
  admin: Boolean,
  maxBytes: Number,
  catalogError: String,
});
const emit = defineEmits(["update:visible", "saved"]);
const client = createImageClient(),
  upload = createImageUpload(createFilesClient(), client);
const form = ref({}),
  file = ref(null),
  busy = ref(false),
  error = ref("");
watch(
  () => props.visible,
  (visible) => {
    if (!visible) return;
    const row = props.item;
    form.value = row
      ? {
          name: row.name,
          allow_as_cover: row.allow_as_cover,
          allow_as_background: row.allow_as_background,
          category_ids: [...row.category_ids],
        }
      : {
          name: "",
          allow_as_cover: true,
          allow_as_background: false,
          category_ids: [],
          team_id: "",
        };
    file.value = null;
    upload.reset(null);
    error.value = "";
  },
);
function chooseFile(list) {
  const next = list?.[0];
  if (!next || busy.value) return;
  if (!/\.(png|jpe?g|webp|gif)$/i.test(next.name) || !next.size) {
    error.value = "请选择非空 PNG、JPG、WEBP 或 GIF 图片";
    return;
  }
  if (props.maxBytes && next.size > props.maxBytes) {
    error.value = "图片超出上传大小限制";
    return;
  }
  file.value = next;
  upload.reset(next);
  form.value.name ||= next.name.replace(/\.[^.]+$/, "");
  error.value = "";
}
async function save() {
  if (busy.value) return;
  if (!form.value.name.trim()) {
    error.value = "请填写图片名称";
    return;
  }
  if (!form.value.allow_as_cover && !form.value.allow_as_background) {
    error.value = "请至少选择一种用途";
    return;
  }
  if (!props.item && !file.value) {
    error.value = "请选择图片";
    return;
  }
  if (!props.item && props.admin && !form.value.team_id) {
    error.value = "请选择运营团队";
    return;
  }
  busy.value = true;
  error.value = "";
  try {
    const input = { ...form.value, name: form.value.name.trim() };
    if (props.item)
      await client.updateImageAsset(props.item.id, {
        ...input,
        expected_revision: props.item.revision,
      });
    else {
      if (!input.team_id) delete input.team_id;
      await upload.save(input);
    }
    emit("update:visible", false);
    emit("saved");
    feedback.toast.success(props.item ? "图片已保存" : "图片已上传");
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
</script>
<template>
  <ResourceOverlay
    :visible="visible"
    :title="item ? '编辑图片' : '上传图片'"
    :busy="busy"
    @update:visible="emit('update:visible', $event)"
  >
    <FeedbackAlert
      v-if="error || catalogError"
      theme="error"
      :message="error || catalogError"
    />
    <ResourceForm>
      <t-form-item v-if="!item" label="选择图片" required>
        <t-upload
          theme="custom"
          draggable
          :auto-upload="false"
          :files="[]"
          :multiple="false"
          :disabled="busy"
          accept=".png,.jpg,.jpeg,.webp,.gif"
          :use-mock-progress="false"
          @select-change="chooseFile"
          ><template #dragContent
            ><div class="image-upload">
              <CloudUploadIcon /><span>{{
                file ? file.name : "点击选择或拖拽图片"
              }}</span>
            </div></template
          ></t-upload
        >
        <small
          >支持 PNG、JPG、WEBP、GIF<span v-if="maxBytes"
            >，不超过 {{ Math.floor(maxBytes / 1048576) }} MB</span
          ></small
        >
      </t-form-item>
      <t-form-item label="图片名称" required
        ><t-input v-model="form.name" :maxlength="160" :disabled="busy"
      /></t-form-item>
      <t-form-item label="用途" required
        ><div class="image-usage">
          <t-checkbox v-model="form.allow_as_cover" :disabled="busy"
            >现成封面</t-checkbox
          ><t-checkbox v-model="form.allow_as_background" :disabled="busy"
            >底图</t-checkbox
          >
        </div></t-form-item
      >
      <t-form-item label="内容类目"
        ><t-select
          v-model="form.category_ids"
          multiple
          clearable
          :options="categories"
          :disabled="busy || !!catalogError"
          placeholder="不选择时为通用图片"
      /></t-form-item>
      <t-form-item v-if="!item && admin" label="运营团队" required
        ><t-select v-model="form.team_id" :options="teams" :disabled="busy"
      /></t-form-item>
    </ResourceForm>
    <template #footer
      ><t-button
        theme="default"
        variant="outline"
        :disabled="busy"
        @click="emit('update:visible', false)"
        >取消</t-button
      ><t-button :loading="busy" :disabled="!!catalogError" @click="save"
        >保存</t-button
      ></template
    >
  </ResourceOverlay>
</template>
<style scoped>
.image-usage {
  display: flex;
  gap: 24px;
}
.image-upload {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 24px;
  color: var(--wt-text-secondary);
}
.image-upload svg {
  font-size: 32px;
}
</style>
