<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { createTaskClient } from '../../../shared/api/tasks.js'

const taskClient = createTaskClient()
const tasks = ref([])
const newTaskType = ref('noop_task')
const error = ref('')
let pollTimer

async function loadTaskFromQuery() {
  const taskId = new URLSearchParams(window.location.search).get('task_id')
  if (!taskId) return
  try {
    const task = await taskClient.get(taskId)
    tasks.value = [task]
    if (['pending', 'leased', 'running'].includes(task.status)) {
      pollTimer = window.setInterval(async () => {
        try { tasks.value = [await taskClient.get(taskId)] } catch (e) { error.value = e.message }
      }, 2000)
    }
  } catch (e) { error.value = e.message }
}

onMounted(loadTaskFromQuery)
onBeforeUnmount(() => { if (pollTimer) window.clearInterval(pollTimer) })

async function createTask() {
  error.value = ''
  try {
    const task = await taskClient.create(newTaskType.value)
    tasks.value.unshift(task)
  } catch (e) {
    error.value = e.message
  }
}

async function cancelLastTask() {
  if (!tasks.value.length) return
  error.value = ''
  try {
    await taskClient.cancel(tasks.value[0].task_id, 'operator cancelled')
    tasks.value[0].status = 'cancelled'
  } catch (e) {
    error.value = e.message
  }
}

async function retryTask(task) {
  error.value = ''
  try {
    const retry = await taskClient.retry(task.task_id)
    tasks.value.unshift(retry)
  } catch (e) { error.value = e.message }
}

const statusMap = {
  pending: { label: '待执行', theme: 'warning' },
  leased: { label: '已领取', theme: 'primary' },
  running: { label: '执行中', theme: 'primary' },
  succeeded: { label: '已完成', theme: 'success' },
  failed: { label: '失败', theme: 'danger' },
  cancelled: { label: '已取消', theme: 'default' },
}

const columns = [
  { colKey: 'task_id', title: '任务 ID', width: 200 },
  { colKey: 'task_type', title: '类型', width: 100 },
  { colKey: 'status', title: '状态', width: 120 },
  { colKey: 'progress', title: '进度', width: 150 },
  { colKey: 'message', title: '消息' },
  { colKey: 'op', title: '操作', width: 90 },
]
</script>

<template>
  <t-alert v-if="error" :message="error" theme="error" style="margin-bottom:16px" closable @close="error=''" />
  <t-card title="创建和查看任务" :bordered="true">
    <div style="display:flex; gap:12px; margin-bottom:16px">
      <t-select v-model="newTaskType" style="width:200px">
        <t-option value="noop_task" label="验证任务 (noop)" />
      </t-select>
      <t-button @click="createTask">创建任务</t-button>
      <t-button v-if="tasks.length" theme="default" @click="cancelLastTask">取消最近任务</t-button>
    </div>
  <t-table v-if="tasks.length" :data="tasks" :columns="columns" size="small" hover>
      <template #status="{ row }">
        <t-tag :theme="(statusMap[row.status] || {}).theme || 'default'" size="small">
          {{ (statusMap[row.status] || {}).label || row.status }}
        </t-tag>
      </template>
      <template #op="{ row }">
        <t-button v-if="row.status === 'failed' || row.status === 'cancelled'" size="small" variant="text" @click="retryTask(row)">重试</t-button>
      </template>
      <template #progress="{ row }">
        <t-progress v-if="row.progress" :percentage="row.progress" :stroke-width="8" />
        <span v-else style="color:var(--td-text-color-secondary)">-</span>
      </template>
    </t-table>
    <t-empty v-else description="还没有创建任务" />
  </t-card>
</template>
