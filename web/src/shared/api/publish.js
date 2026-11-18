import { createApiClient } from "./http.js";
export function createPublishClient({
  base = "/api/v1",
  fetch = globalThis.fetch,
} = {}) {
  const api = createApiClient({ base, fetchImpl: fetch });
  return {
    list: (params) => api.get("/publish", params),
    get: (id) => api.get(`/publish/${id}`),
    create: (input) => api.post("/publish", input),
    edit: (id, input) => api.put(`/publish/${id}/content`, input),
  };
}
