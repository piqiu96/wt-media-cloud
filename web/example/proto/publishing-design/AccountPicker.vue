<script setup>
import { computed, ref } from "vue";
import Overlay from "./PrototypeOverlay.vue";
import { accountAvailability } from "./accountAvailability.js";
const props = defineProps({ db: Object, platform: String, category: String });
const emit = defineEmits(["select"]);
const visible = ref(false),
  search = ref(""),
  level = ref(""),
  only = ref(false);
const levels = computed(() => [
  ...new Set(
    props.db.accounts
      .filter((a) => a.platform === props.platform)
      .map((a) => a.level)
      .filter((v) => v != null),
  ),
]);
function access(a) {
  return accountAvailability(props.db, a, props.platform, props.category);
}
function reason(a) {
  return access(a).reason;
}
const rows = computed(() =>
  props.db.accounts.filter(
    (a) =>
      a.platform === props.platform &&
      `${a.name} ${a.id}`.includes(search.value) &&
      (!level.value || a.level === level.value) &&
      (!only.value || !reason(a)),
  ),
);
defineExpose({
  open() {
    visible.value = true;
  },
});
</script>
<template>
  <Overlay v-model:visible="visible" title="选择发布账号" :width="900"
    ><p>{{ platform }} · {{ category }} · 仅可选择当前类目可用账号</p>
    <div class="account-tools">
      <t-input
        v-model="search"
        placeholder="搜索账号昵称 / ID"
        clearable
      /><t-select
        v-model="level"
        clearable
        placeholder="全部等级"
        :options="levels.map((v) => ({ label: String(v), value: v }))"
      /><t-checkbox v-model="only">仅显示可发布账号</t-checkbox>
    </div>
    <div class="account-list">
      <article v-for="a in rows" :key="a.id">
        <div class="avatar">{{ a.name?.slice(0, 1) }}</div>
        <div>
          <strong>{{ a.name }}</strong
          ><small
            >#{{ a.id }} · {{ a.platform }} ·
            {{ a.categories?.join(" / ") }}</small
          >
        </div>
        <div>
          <span>等级 {{ a.level ?? "未同步" }}</span
          ><small
            >今日余量 {{ access(a).remaining ?? "未知" }} / 上限
            {{ access(a).limit ?? "未配置" }}</small
          >
        </div>
        <div>
          <small v-if="reason(a)" class="blocked">{{ reason(a) }}</small
          ><t-button
            v-else
            variant="outline"
            @click="
              emit('select', a);
              visible = false;
            "
            >选择账号</t-button
          >
        </div>
      </article>
      <p v-if="!rows.length">未找到匹配账号，请调整筛选条件。</p>
    </div>
    <p class="muted">账号额度以平台最新同步结果为准。</p>
    <template #footer
      ><t-button variant="outline" @click="visible = false"
        >取消</t-button
      ></template
    ></Overlay
  >
</template>
<style scoped>
.account-tools {
  display: flex;
  gap: 12px;
  margin: 20px 0;
}
.account-tools > .t-input__wrap {
  flex: 1;
}
.account-tools > .t-select__wrap {
  width: 150px;
}
.account-tools > .t-checkbox {
  white-space: nowrap;
}
.account-list article {
  display: grid;
  grid-template-columns: 40px 1fr 1fr auto;
  gap: 16px;
  align-items: center;
  padding: 16px 8px;
  border-bottom: 1px solid var(--wt-color-border);
}
.account-list small {
  display: block;
  color: var(--wt-color-text-secondary);
  margin-top: 5px;
}
.avatar {
  background: var(--wt-color-primary-light);
  border-radius: 50%;
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  color: var(--wt-color-primary);
}
.blocked {
  color: var(--wt-color-warning-text) !important;
}
.muted {
  color: var(--wt-color-text-secondary);
  font-size: 12px;
}
</style>
