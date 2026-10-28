import { createApiClient } from '../../../../shared/api/http.js'

export function createUsersClient({ base = '/api/v1', fetch = globalThis.fetch } = {}) {
  const api = createApiClient({ base, fetchImpl: fetch })
  return {
    listUsers(filters = {}) {
      return api.get('/users', {
        uid: filters.uid,
        username: filters.username,
        role: filters.role,
        team_id: filters.teamId,
        status: filters.status,
        category_id: filters.categoryId,
      })
    },
    createUser(input) {
      return api.post('/users', userPayload(input, true))
    },
    updateUser(userId, input) {
      return api.patch(`/users/${userId}`, userPayload(input, false))
    },
    resetPassword(userId, newPassword) {
      return api.post(`/users/${userId}/reset-password`, { new_password: newPassword })
    },
    deleteUser(userId) {
      return api.delete(`/users/${userId}`)
    },
    listTeams(filters = {}) {
      return api.get('/operation-teams', { keyword: filters.keyword })
    },
    createTeam(name) {
      return api.post('/operation-teams', { name })
    },
    renameTeam(teamId, name) {
      return api.patch(`/operation-teams/${teamId}`, { name })
    },
    deleteTeam(teamId) {
      return api.delete(`/operation-teams/${teamId}`)
    },
    listCategories(filters = {}) {
      return api.get('/categories', {
        keyword: filters.keyword,
        type: filters.type,
        enabled: filters.enabled,
        scope: filters.scope,
      })
    },
    getCategoryReferences(categoryId) {
      return api.get(`/categories/${encodeURIComponent(categoryId)}/references`)
    },
    createCategory(input) {
      return api.post('/categories', { type: input.type, name: input.name, remark: input.remark })
    },
    updateCategory(categoryId, input) {
      return api.patch(`/categories/${categoryId}`, { name: input.name, enabled: input.enabled, remark: input.remark })
    },
    deleteCategory(categoryId) {
      return api.delete(`/categories/${categoryId}`)
    },
  }
}

function userPayload(input, includeCredentials) {
  const payload = {
    role: input.role,
    team_id: input.role === 'admin' ? null : Number(input.teamId),
    status: input.status,
    category_ids: input.role === 'admin' ? [] : input.categoryIds,
  }
  if (includeCredentials) {
    payload.username = input.username
    payload.password = input.password
    delete payload.status
  }
  return payload
}
