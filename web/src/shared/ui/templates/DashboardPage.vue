<!--
  模板 6/6: Data Dashboard
  用途：首页工作台、数据统计看板
  结构：统计卡片行 + 图表区域 + 最近列表
-->
<script setup>
defineProps({
  title: { type: String, default: "" },
  loading: { type: Boolean, default: false },
})
</script>

<template>
  <div class="dashboard-page">
    <!-- 标题 -->
    <t-breadcrumb v-if="title">
      <t-breadcrumb-item>{{ title }}</t-breadcrumb-item>
    </t-breadcrumb>

    <!-- 统计卡片 (slot) -->
    <t-row :gutter="16" class="dashboard-stats">
      <t-col :span="6">
        <slot name="stat-1">
          <t-card>
            <template #title>待办任务</template>
            <div class="stat-number">0</div>
          </t-card>
        </slot>
      </t-col>
      <t-col :span="6">
        <slot name="stat-2">
          <t-card>
            <template #title>今日素材</template>
            <div class="stat-number">0</div>
          </t-card>
        </slot>
      </t-col>
      <t-col :span="6">
        <slot name="stat-3">
          <t-card>
            <template #title>进行中合成</template>
            <div class="stat-number">0</div>
          </t-card>
        </slot>
      </t-col>
      <t-col :span="6">
        <slot name="stat-4">
          <t-card>
            <template #title>待发布</template>
            <div class="stat-number">0</div>
          </t-card>
        </slot>
      </t-col>
    </t-row>

    <!-- 图表区域 (slot) -->
    <t-row :gutter="16" class="dashboard-charts">
      <t-col :span="12">
        <t-card title="趋势">
          <slot name="chart-trend">
            <p class="placeholder">ECharts 集成（M9）</p>
          </slot>
        </t-card>
      </t-col>
    </t-row>

    <!-- 最近列表 (slot) -->
    <t-card title="最近更新" class="dashboard-recent">
      <slot name="recent-list">
        <t-empty description="暂无最近更新" />
      </slot>
    </t-card>
  </div>
</template>

<style scoped>
.dashboard-page {
  padding: 16px;
}
.dashboard-stats {
  margin-bottom: 16px;
}
.stat-number {
  font-size: 32px;
  font-weight: 700;
  line-height: 1.2;
  margin-top: 8px;
}
.dashboard-charts {
  margin-bottom: 16px;
}
.dashboard-recent {
  margin-bottom: 16px;
}
.placeholder {
  color: var(--td-text-color-placeholder);
  text-align: center;
  padding: 40px 0;
}
</style>
