import { createApiClient } from './http.js'

const api = createApiClient()

export function createTaskClient() {
  return {
    create(taskType = 'noop_task', idempotencyKey = '') {
      return api.post('/tasks', { task_type: taskType, idempotency_key: idempotencyKey })
    },
    get(taskId) {
      return api.get(`/cloud-agent/tasks/${encodeURIComponent(taskId)}`)
    },
    claim(agentId, leaseSeconds = 60) {
      return api.post('/cloud-agent/tasks/claim', { agent_id: agentId, lease_seconds: leaseSeconds })
    },
    cancel(taskId, message = '') {
      return api.post(`/cloud-agent/tasks/${encodeURIComponent(taskId)}/cancel`, { message })
    },
  }
}
