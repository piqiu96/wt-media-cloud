<!--
  模板 2/6: Task Workbench
  用途：抓取、合成、发布、互动、自动生产任务控制台
  结构：统计卡片 + 状态页签 + 任务列表 + 进度/日志
-->
<script setup>
defineProps({
  title: { type: String, default: "" },
  loading: { type: Boolean, default: false },
})

const emit = defineEmits(["tab-change", "retry", "cancel"])
</script>

<template>
  <div class="task-workbench">
    <!-- 标题 -->
    <t-breadcrumb v-if="title">
      <t-breadcrumb-item>{{ title }}</t-breadcrumb-item>
    </t-breadcrumb>

    <!-- 统计卡片行 -->
    <t-row :gutter="16" class="stats-row">
      <t-col :span="6">
        <t-card class="stat-card">
          <template #title>待处理</template>
          <div class="stat-value">0</div>
        </t-card>
      </t-col>
      <t-col :span="6">
        <t-card class="stat-card">
          <template #title>执行中</template>
          <div class="stat-value">0</div>
        </t-card>
      </t-col>
      <t-col :span="6">
        <t-card class="stat-card">
          <template #title>已完成</template>
          <div class="stat-value">0</div>
        </t-card>
      </t-col>
      <t-col :span="6">
        <t-card class="stat-card">
          <template #title>失败</template>
          <div class="stat-value">0</div>
        </t-card>
      </t-col>
    </t-row>

    <!-- 状态页签 -->
    <t-tabs :default-value="'all'" @change="emit('tab-change')">
      <t-tab-panel value="all" label="全部" />
      <t-tab-panel value="pending" label="待处理" />
      <t-tab-panel value="running" label="执行中" />
      <t-tab-panel value="success" label="成功" />
      <t-tab-panel value="failed" label="失败" />
    </t-tabs>

    <!-- 任务列表表格 -->
    <t-table
      :loading="loading"
      row-key="id"
      :data="[]"
      :columns="[]"
      empty="暂无任务"
    >
      <template #empty>
        <t-empty />
      </template>
    </t-table>

    <!-- 进度/日志面板 (slot) -->
    <slot name="progress" />
  </div>
</template>

<style scoped>
.task-workbench {
  padding: 16px;
}
.stats-row {
  margin-bottom: 16px;
}
.stat-value {
  font-size: 28px;
  font-weight: 700;
  line-height: 1.2;
}
</style>
