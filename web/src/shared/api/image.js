import { createApiClient } from "./http.js";
export function createImageClient({
  base = "/api/v1",
  fetch = globalThis.fetch,
} = {}) {
  const api = createApiClient({ base, fetchImpl: fetch });
  return {
    listCoverTemplate: (params) => api.get("/cover-template", params),
    getCoverTemplate: (id) => api.get(`/cover-template/${id}`),
    createCoverTemplate: (input) => api.post("/cover-template", input),
    saveCoverTemplateDraft: (id, input) =>
      api.put(`/cover-template/${id}/draft`, input),
    publishCoverTemplate: (id, input) =>
      api.post(`/cover-template/${id}/publish`, input),
    setCoverTemplateUsage: (id, input) =>
      api.put(`/cover-template/${id}/usage`, input),
    copyCoverTemplate: (id, input) =>
      api.post(`/cover-template/${id}/copy`, input),
    deleteCoverTemplate: (id, revision) =>
      api.delete(
        `/cover-template/${id}?expected_revision=${encodeURIComponent(revision)}`,
      ),
    validateCoverTemplate: (id, input, options) =>
      api.stream(`/cover-template/${id}/validate`, input, options),
    getCover: (id) => api.get(`/cover/${id}`),
    renderCover: (input, options) =>
      api.stream("/cover/render", input, options),
    getImageFont: (family, weight) =>
      api.getBinary(`/image-font/${family}/${weight}`),
    listImageAsset: (params) => api.get("/image-asset", params),
    getImageAsset: (id) => api.get(`/image-asset/${id}`),
    registerImageAsset: (input) => api.post("/image-asset", input),
    updateImageAsset: (id, input) => api.put(`/image-asset/${id}`, input),
    setImageAssetUsage: (id, input) =>
      api.put(`/image-asset/${id}/usage`, input),
    batchImageAsset: (input) => api.post("/image-asset/batch", input),
    selectImageAsset: (input) => api.post("/image-asset/select", input),
    listImageAssetReference: (id) => api.get(`/image-asset/${id}/reference`),
    deleteImageAsset: (id, revision) =>
      api.delete(
        `/image-asset/${id}?expected_revision=${encodeURIComponent(revision)}`,
      ),
    listCategory: () => api.get("/categories", { enabled: true }),
    listTeam: () => api.get("/operation-teams"),
  };
}
