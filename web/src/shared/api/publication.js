import { createApiClient } from "./http.js";
export function createPublicationClient({
  base = "/api/v1",
  fetch = globalThis.fetch,
} = {}) {
  const api = createApiClient({ base, fetchImpl: fetch });
  return {
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

// Batch configuration has its own lifecycle. Saving never calls task creation
// or execution; CHG108 adds the separately authorized start command.
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
    generate: (input, options) => api.stream("/publication/generate", input, options),
  };
}
