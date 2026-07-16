<!--
  模板 4/6: Detail Drawer
  用途：任何页面的详情侧边抽屉
  结构：统一 Tab（基本信息 → 关联记录 → 执行历史 → 审计日志）
-->
<script setup>
defineProps({
  visible: { type: Boolean, default: false },
  title: { type: String, default: "详情" },
})
const emit = defineEmits(["close"])
</script>

<template>
  <t-drawer
    :visible="visible"
    :header="title"
    :size="'640px'"
    @close="emit('close')"
    destroy-on-close
  >
    <t-tabs default-value="basic" :style="{ marginTop: '-16px' }">
      <t-tab-panel value="basic" label="基本信息">
        <slot name="basic">
          <p class="placeholder">暂无基本信息</p>
        </slot>
      </t-tab-panel>

      <t-tab-panel value="related" label="关联记录">
        <slot name="related">
          <p class="placeholder">暂无关联记录</p>
        </slot>
      </t-tab-panel>

      <t-tab-panel value="history" label="执行历史">
        <slot name="history">
          <p class="placeholder">暂无执行历史</p>
        </slot>
      </t-tab-panel>

      <t-tab-panel value="audit" label="审计日志">
        <slot name="audit">
          <p class="placeholder">暂无审计日志</p>
        </slot>
      </t-tab-panel>
    </t-tabs>
  </t-drawer>
</template>

<style scoped>
.placeholder {
  color: var(--td-text-color-placeholder);
  text-align: center;
  padding: 40px 0;
}
</style>
