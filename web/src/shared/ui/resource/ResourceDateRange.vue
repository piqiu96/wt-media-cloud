<script setup>
import { DateRangePicker } from 'tdesign-vue-next'
defineOptions({ inheritAttrs: false })
</script>
<template>
  <DateRangePicker class="resource-date-range" value-type="YYYY-MM-DD" format="YYYY-MM-DD" :placeholder="['开始日期', '结束日期']" clearable v-bind="$attrs" />
</template>
<style scoped>
.resource-date-range { width:100%; min-width:0; }
.resource-date-range :deep(.t-range-input) { height:36px; }
.resource-date-range :deep(.t-input) { min-height:0 !important; height:auto; }
</style>
