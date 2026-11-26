import { createApiClient } from "./http.js";
export function createPublicationClient({
  base = "/api/v1",
  fetch = globalThis.fetch,
} = {}) {
  const api = createApiClient({ base, fetchImpl: fetch });
  return {
    platformRules: () => api.get("/publication/platform-rules"),
    generate: (input, options) => api.stream("/publication/generate", input, options),
    list: (params) => api.get("/publication", params),
    outputs: (params) => api.get("/publication/output", params),
    get: (id) => api.get(`/publication/${id}`),
    copy: (id, revision) => api.get(`/publication/${id}/copy`, { expected_revision: revision }),
    findByRequest: (requestKey) => api.get("/publication/request", { request_key: requestKey }),
    create: (input) => api.post("/publication", input),
    start: (id, input) => api.post(`/publication/${id}/start`, input),
    resolve: (id, input) => api.post(`/publication/${id}/resolve`, input),
    retry: (id, input) => api.post(`/publication/${id}/retry`, input),
    cancel: (id, input) => api.post(`/publication/${id}/cancel`, input),
    abandon: (id, input) => api.post(`/publication/${id}/abandon`, input),
    attempt: (id) => api.get(`/publication/${id}/attempt`),
    edit: (id, input) => api.put(`/publication/${id}/content`, input),
  };
}

// Batch configuration saves and explicit execution commands have separate endpoints.
export function createPublicationBatchClient({
  base = "/api/v1",
  fetch = globalThis.fetch,
} = {}) {
  const api = createApiClient({ base, fetchImpl: fetch });
  return {
    list: (params) => api.get("/publication-batch", params),
    get: (id) => api.get(`/publication-batch/${id}`),
    copy: (id, revision) => api.get(`/publication-batch/${id}/copy`, { expected_revision: revision }),
    resources: (params) => api.get("/publication/resource", params),
    accounts: (params) => api.get("/publication/account", params),
    allocate: (input) => api.post("/publication-batch/allocate", input),
    validate: (input) => api.post("/publication-batch/validate", input),
    save: (input) => api.post("/publication-batch", input),
    update: (id, input) => api.put(`/publication-batch/${id}/configuration`, input),
    records: (id, params) => api.get(`/publication-batch/${id}/record`, params),
    tasks: (id, params) => api.get(`/publication-batch/${id}/task`, params),
    executionPreview: (id, input) => api.post(`/publication-batch/${id}/execution-preview`, input),
    start: (id, input) => api.post(`/publication-batch/${id}/start`, input),
    pause: (id, input) => api.post(`/publication-batch/${id}/pause`, input),
    resume: (id, input) => api.post(`/publication-batch/${id}/resume`, input),
    stop: (id, input) => api.post(`/publication-batch/${id}/stop`, input),
    finish: (id, input) => api.post(`/publication-batch/${id}/finish`, input),
    generate: (input, options) => api.stream("/publication/generate", input, options),
  };
}
