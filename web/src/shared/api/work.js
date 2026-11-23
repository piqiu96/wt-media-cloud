import { createApiClient } from "./http.js";
export function createWorkClient({
  base = "/api/v1",
  fetch = globalThis.fetch,
} = {}) {
  const api = createApiClient({ base, fetchImpl: fetch });
  return {
    list: (params) => api.get("/work", params),
    get: (id) => api.get(`/work/${id}`),
    summary: (params) => api.get("/work/summary", params),
  };
}
