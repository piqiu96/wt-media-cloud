<!--
  模板 3/6: Batch Configuration Wizard
  用途：批量发布、批量互动、批量领取
  结构：步骤条（选择 → 配置 → 预览 → 执行 → 结果）
-->
<script setup>
import { ref } from "vue"

defineProps({
  title: { type: String, default: "" },
  steps: {
    type: Array,
    default: () => ["选择", "配置", "预览", "执行", "结果"],
  },
  loading: { type: Boolean, default: false },
})

const emit = defineEmits(["next", "prev", "submit", "cancel"])
const current = ref(0)
</script>

<template>
  <div class="batch-wizard">
    <!-- 标题 -->
    <t-breadcrumb v-if="title">
      <t-breadcrumb-item>{{ title }}</t-breadcrumb-item>
    </t-breadcrumb>

    <!-- 步骤条 -->
    <t-steps :current="current" layout="horizontal" class="wizard-steps">
      <t-step-item v-for="(s, i) in steps" :key="i" :title="s" />
    </t-steps>

    <!-- 步骤内容 (slot) -->
    <t-card class="step-content">
      <slot :name="`step-${current}`" :step="current" />
      <p v-if="!$slots[`step-${current}`]">步骤 {{ current + 1 }}: {{ steps[current] }}</p>
    </t-card>

    <!-- 操作按钮 -->
    <div class="wizard-actions">
      <t-button v-if="current > 0" variant="outline" @click="current--; emit('prev', current)">上一步</t-button>
      <t-space>
        <t-button variant="outline" @click="emit('cancel')">取消</t-button>
        <t-button
          v-if="current < steps.length - 1"
          theme="primary"
          @click="current++; emit('next', current)"
        >
          下一步
        </t-button>
        <t-button
          v-else
          theme="primary"
          :loading="loading"
          @click="emit('submit')"
        >
          执行
        </t-button>
      </t-space>
    </div>
  </div>
</template>

<style scoped>
.batch-wizard {
  padding: 16px;
}
.wizard-steps {
  margin-bottom: 24px;
}
.step-content {
  min-height: 200px;
  margin-bottom: 24px;
}
.wizard-actions {
  display: flex;
  justify-content: space-between;
}
</style>
