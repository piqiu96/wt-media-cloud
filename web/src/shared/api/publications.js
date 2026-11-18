import { createApiClient } from "./http.js";
export function createPublicationsClient({
  base = "/api/v1",
  fetch = globalThis.fetch,
} = {}) {
  const api = createApiClient({ base, fetchImpl: fetch });
  return {
    list: (params) => api.get("/publications", params),
    get: (id) => api.get(`/publications/${id}`),
    create: (input) => api.post("/publications", input),
    start: (id, input) => api.post(`/publications/${id}/start`, input),
    attempts: (id) => api.get(`/publications/${id}/attempts`),
    resolve: (id, input) => api.post(`/publications/${id}/resolve`, input),
    retry: (id, input) => api.post(`/publications/${id}/retry`, input),
    edit: (id, input) => api.put(`/publications/${id}/content`, input),
  };
}
