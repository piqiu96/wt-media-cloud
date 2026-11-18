import { createApiClient } from "./http.js";
export function createPublicationClient({
  base = "/api/v1",
  fetch = globalThis.fetch,
} = {}) {
  const api = createApiClient({ base, fetchImpl: fetch });
  return {
    list: (params) => api.get("/publication", params),
    get: (id) => api.get(`/publication/${id}`),
    create: (input) => api.post("/publication", input),
    edit: (id, input) => api.put(`/publication/${id}/content`, input),
  };
}
