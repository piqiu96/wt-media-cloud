<!--
  模板 1/6: Resource List Page
  用途：内容发现、媒体账号、素材库、成片管理、评论模板
  结构：搜索栏 + 批量操作栏 + 表格 + 侧边抽屉详情
-->
<script setup>
defineProps({
  title: { type: String, default: "" },
  loading: { type: Boolean, default: false },
})

const emit = defineEmits(["search", "batch-action", "refresh"])
</script>

<template>
  <div class="resource-list-page">
    <!-- 标题 -->
    <t-breadcrumb v-if="title">
      <t-breadcrumb-item>{{ title }}</t-breadcrumb-item>
    </t-breadcrumb>

    <!-- 搜索/过滤区 -->
    <t-card class="search-bar">
      <t-form layout="inline">
        <t-form-item label="关键词">
          <t-input placeholder="搜索…" clearable @enter="emit('search')" />
        </t-form-item>
        <t-form-item label="状态">
          <t-select placeholder="全部" style="width: 140px" />
        </t-form-item>
        <t-form-item>
          <t-button theme="primary" @click="emit('search')">查询</t-button>
          <t-button @click="emit('refresh')">重置</t-button>
        </t-form-item>
      </t-form>
    </t-card>

    <!-- 批量操作栏 -->
    <div class="batch-bar">
      <t-button variant="outline" :disabled="loading">批量操作</t-button>
      <t-button variant="outline" :disabled="loading">导出</t-button>
      <t-space class="right-actions">
        <t-button theme="primary" @click="emit('batch-action', 'create')">新建</t-button>
      </t-space>
    </div>

    <!-- 表格 -->
    <t-table
      :loading="loading"
      row-key="id"
      :data="[]"
      :columns="[]"
      :pagination="{ pageSize: 20, total: 0 }"
      empty="暂无数据"
    >
      <template #empty>
        <t-empty />
      </template>
    </t-table>

    <!-- 详情抽屉 (slot) -->
    <slot name="drawer" />
  </div>
</template>

<style scoped>
.resource-list-page {
  padding: 16px;
}
.search-bar {
  margin-bottom: 16px;
}
.batch-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.right-actions {
  margin-left: auto;
}
</style>
