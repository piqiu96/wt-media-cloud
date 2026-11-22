export function coverRenderInput(config, output, rowKey, revision) {
  if (
    !config.template_id ||
    !config.published_revision ||
    !config.title?.trim()
  )
    throw new Error("请选择模板并填写封面标题");
  const input = {
    template_id: String(config.template_id),
    expected_published_revision: config.published_revision,
    title: config.title,
    row_key: rowKey,
    input_revision: revision,
  };
  if (config.background_mode === "business") {
    if (config.background_source === "image") {
      if (!config.background_image_asset_id) throw new Error("请选择可用底图");
      input.background_image_asset_id = String(
        config.background_image_asset_id,
      );
    } else {
      if (!output?.id || !output?.thumbnail?.file_id)
        throw new Error("成片没有可用封面，请从图片库选择底图");
      input.background_output_id = String(output.id);
    }
  }
  return input;
}
export function coverReference(result) {
  const s = result.snapshot;
  return {
    template_id: String(s.template_id),
    published_revision: s.published_revision,
    title: s.title,
    background_image_asset_id: String(s.background_image_asset_id || "0"),
    background_output_id: String(s.background_output_id || "0"),
    render_input_hash: result.render_input_hash,
  };
}
export function coverResultMatches(result, input) {
  if (
    !result?.file?.id ||
    !result.file.resource_path ||
    result.preview !== false ||
    !result.render_input_hash ||
    result.row_key !== input.row_key ||
    result.input_revision !== input.input_revision
  )
    return false;
  const s = result.snapshot;
  return (
    !!s &&
    String(s.template_id) === input.template_id &&
    s.published_revision === input.expected_published_revision &&
    s.title === input.title &&
    (!input.background_output_id ||
      String(s.background_output_id) === input.background_output_id) &&
    (!input.background_image_asset_id ||
      String(s.background_image_asset_id) === input.background_image_asset_id)
  );
}
export function emptyCoverConfig() {
  return {
    method: "image",
    template_id: "",
    published_revision: 0,
    title: "",
    background_mode: "business",
    background_source: "output",
    background_image_asset_id: "",
  };
}
export function coverConfigFromReference(ref, snapshot) {
  return {
    ...emptyCoverConfig(),
    method: "template",
    template_id: ref.template_id,
    published_revision: ref.published_revision,
    title: ref.title,
    background_mode:
      snapshot?.document?.backgroundMode ||
      ((ref.background_output_id && ref.background_output_id !== "0") ||
      (ref.background_image_asset_id && ref.background_image_asset_id !== "0")
        ? "business"
        : "solid"),
    background_source:
      ref.background_image_asset_id && ref.background_image_asset_id !== "0"
        ? "image"
        : "output",
    background_image_asset_id:
      ref.background_image_asset_id === "0"
        ? ""
        : ref.background_image_asset_id,
  };
}

export function coverMatchesRatio(file, width, height) {
  const metadata = file?.metadata || file;
  return (
    Number(metadata?.width) > 0 &&
    Number(metadata?.height) > 0 &&
    Number(metadata.width) * height === Number(metadata.height) * width &&
    (!file?.size_bytes || Number(file.size_bytes) <= 5 * 1024 * 1024)
  );
}
