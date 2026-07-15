// Cloud Web task client for the generic task API (M1-R6).

const BASE = '/api/v1'

async function request(path, options = {}) {
  const resp = await fetch(`${BASE}${path}`, {
    credentials: 'same-origin',
    headers: { 'content-type': 'application/json' },
    ...options,
  })
  const body = await resp.json()
  if (!resp.ok) {
    const err = body?.error || { code: 'unknown', message: resp.statusText }
    throw new Error(err.message || err.code)
  }
  return body.data
}

export function createTaskClient() {
  return {
    async create(taskType = 'noop_task', idempotencyKey = '') {
      return request('/tasks', {
        method: 'POST',
        body: JSON.stringify({ task_type: taskType, idempotency_key: idempotencyKey }),
      })
    },

    async get(taskId) {
      return request(`/cloud-agent/tasks/${encodeURIComponent(taskId)}`)
    },

    async claim(agentId, leaseSeconds = 60) {
      return request('/cloud-agent/tasks/claim', {
        method: 'POST',
        body: JSON.stringify({ agent_id: agentId, lease_seconds: leaseSeconds }),
      })
    },

    async cancel(taskId, message = '') {
      return request(`/cloud-agent/tasks/${encodeURIComponent(taskId)}/cancel`, {
        method: 'POST',
        body: JSON.stringify({ message }),
      })
    },
  }
}
