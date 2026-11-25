import { createApiClient } from "./http.js";
export function createPublicationPlanClient({
  base = "/api/v1",
  fetch = globalThis.fetch,
} = {}) {
  const a = createApiClient({ base, fetchImpl: fetch }),
    path = (id) => `/publication-plan/${id}`;
  return {
    list: (q) => a.get("/publication-plan", q),
    get: (id) => a.get(path(id)),
    create: (v) => a.post("/publication-plan", v),
    update: (id, v) => a.put(path(id) + "/structure", v),
    validate: (v) => a.post("/publication-plan/validate", v),
    recommend: (v, o) => a.stream("/publication-plan/recommend", v, o),
    batches: (id, q) => a.get(path(id) + "/batch", q),
    records: (id, q) => a.get(path(id) + "/record", q),
    copy: (id, r) => a.get(path(id) + "/copy", { expected_revision: r }),
    ...Object.fromEntries(
      [
        "start",
        "pause",
        "resume",
        "confirm-next",
        "takeover",
        "finish",
        "cancel",
      ].map((k) => [k, (id, v) => a.post(path(id) + "/" + k, v)]),
    ),
  };
}
