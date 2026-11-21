// Upload and library registration are separate requests. Keep the returned file
// so retrying a failed registration never uploads the same bytes again.
export function createImageUpload(files, image) {
  let source = null,
    uploaded = null;
  return {
    reset(file) {
      source = file;
      uploaded = null;
    },
    async save(attributes) {
      if (!source) throw new Error("请选择图片");
      if (!uploaded) uploaded = await files.upload(source.name, source);
      return image.registerImageAsset({
        ...attributes,
        file_id: String(uploaded.id),
      });
    },
  };
}
