<!--
  模板 5/6: Settings Page
  用途：平台配置、风控规则、生产规则、权限设置
  结构：左侧分类导航 + 右侧分组表单 + 保存操作栏
-->
<script setup>
import { ref } from "vue"

defineProps({
  title: { type: String, default: "" },
  /** 左侧导航项: [{value, label}] */
  navItems: {
    type: Array,
    default: () => [],
  },
  saving: { type: Boolean, default: false },
})

const emit = defineEmits(["save", "reset"])
const activeNav = ref("")
</script>

<template>
  <div class="settings-page">
    <!-- 标题 -->
    <t-breadcrumb v-if="title">
      <t-breadcrumb-item>{{ title }}</t-breadcrumb-item>
    </t-breadcrumb>

    <div class="settings-body">
      <!-- 左侧导航 -->
      <t-menu
        v-model="activeNav"
        :value="navItems[0]?.value"
        :style="{ width: '200px', flexShrink: 0 }"
        theme="light"
      >
        <t-menu-item
          v-for="item in navItems"
          :key="item.value"
          :value="item.value"
          @click="activeNav = item.value"
        >
          {{ item.label }}
        </t-menu-item>
      </t-menu>

      <!-- 右侧表单区域 (slot) -->
      <div class="settings-form">
        <t-card>
          <slot :name="`nav-${activeNav}`" :nav="activeNav">
            <p class="placeholder">选择左侧分类查看设置项</p>
          </slot>
        </t-card>
      </div>
    </div>

    <!-- 保存操作栏 -->
    <div class="save-bar">
      <t-space>
        <t-button theme="primary" :loading="saving" @click="emit('save')">保存</t-button>
        <t-button variant="outline" @click="emit('reset')">重置</t-button>
      </t-space>
    </div>
  </div>
</template>

<style scoped>
.settings-page {
  padding: 16px;
}
.settings-body {
  display: flex;
  gap: 16px;
  margin-bottom: 24px;
}
.settings-form {
  flex: 1;
  min-width: 0;
}
.save-bar {
  display: flex;
  justify-content: flex-start;
  border-top: 1px solid var(--td-border-level-1-color);
  padding-top: 16px;
}
.placeholder {
  color: var(--td-text-color-placeholder);
  text-align: center;
  padding: 40px 0;
}
</style>
