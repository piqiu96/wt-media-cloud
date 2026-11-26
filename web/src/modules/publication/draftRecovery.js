import { emptyCoverConfig } from "./coverPreparation.js";
import { DialogPlugin } from "tdesign-vue-next";
import { publicationContentInput } from './platformContent.js';

export function choosePublicationDraft(savedAt, { subject = "发布任务" } = {}) {
  const date = new Date(savedAt);
  const message =
    savedAt && !Number.isNaN(date.getTime())
      ? `上次保存于 ${date.toLocaleString("zh-CN", { month: "long", day: "numeric", hour: "2-digit", minute: "2-digit", hour12: false })}，可以继续编辑或重新开始。`
      : "可以继续编辑上次配置，或重新开始。";
  return new Promise((resolve) => {
    let settled = false,
      dialog;
    const finish = (choice) => {
      if (settled) return;
      settled = true;
      dialog?.destroy();
      resolve(choice);
    };
    dialog = DialogPlugin.confirm({
      header: `发现未完成的${subject}配置`,
      body: message,
      dialogClassName: "resource-feedback-dialog",
      width: "min(calc(100vw - 32px), 560px)",
      confirmBtn: { content: "继续编辑", theme: "primary" },
      cancelBtn: { content: "重新开始", theme: "default", variant: "outline" },
      closeBtn: true,
      closeOnOverlayClick: false,
      onConfirm: () => finish("continue"),
      onCancel: () => finish("restart"),
      onClose: (context) => {
        if (context?.trigger !== "cancel") finish(null);
      },
    });
  });
}

// Input references and human text only; never store resource bodies or signed URLs.
export function capturePublicationDraft(
  form,
  schedule,
  cover,
  pendingRequest,
  config,
  portraitCover,
  portraitConfig,
) {
  const text = (value) => String(value ?? "");
  const bili = form.platform === 'bilibili';
  const secondary = bili ? 'space' : 'portrait';
  const options = form.content.bilibili || {};
  const result = {
    form: {
      request_key: text(form.request_key),
      platform: bili ? 'bilibili' : 'baidu',
      category_id: text(form.category_id),
      media_account_id: text(form.media_account_id),
      composite_output_id: text(form.composite_output_id),
      content: {
        ...(bili ? { title: text(form.content.title), bilibili: {
          partition_id: Number(options.partition_id || 0), partition_name: text(options.partition_name),
          tags: Array.isArray(options.tags) ? options.tags.map(text) : [],
          statement: text(options.statement), rights_reserved: options.rights_reserved === true,
          source: text(options.source), visibility: text(options.visibility),
        }, space_cover_file_id: text(form.content.space_cover_file_id),
        ...(form.content.space_cover_render ? { space_cover_render: captureCoverReference(form.content.space_cover_render) } : {}) } : {}),
        description: text(form.content.description),
        cover_file_id: text(form.content.cover_file_id),
        portrait_cover_file_id: text(form.content.portrait_cover_file_id),
        ...(form.content.portrait_cover_render
          ? {
              portrait_cover_render: captureCoverReference(
                form.content.portrait_cover_render,
              ),
            }
          : {}),
        topics: text(form.content.topics),
        declaration: text(form.content.declaration),
        confirmed: !!form.content.confirmed,
        ...(form.content.cover_render
          ? { cover_render: captureCoverReference(form.content.cover_render) }
          : {}),
      },
    },
    cover_config: config ? captureCoverConfig(config) : undefined,
    [secondary + '_cover_config']: portraitConfig
      ? captureCoverConfig(portraitConfig)
      : undefined,
    [secondary + '_image_asset_id']: text(portraitCover?.image_asset_id),
    [secondary + '_cover_origin']: text(portraitCover?.origin),
    [secondary + '_cover_output_id']: text(portraitCover?.output_id),
    schedule: text(schedule),
    image_asset_id: text(cover?.image_asset_id),
    cover_origin: text(cover?.origin),
    cover_output_id: text(cover?.output_id),
    pending_request: pendingRequest
      ? JSON.parse(JSON.stringify(pendingRequest))
      : null,
  };
  result.form.content = publicationContentInput(form.platform,result.form.content);
  return result;
}

function captureCoverReference(ref) {
  return {
    template_id: String(ref.template_id || ""),
    published_revision: Number(ref.published_revision || 0),
    title: String(ref.title || ""),
    background_image_asset_id: String(ref.background_image_asset_id || "0"),
    background_output_id: String(ref.background_output_id || "0"),
    render_input_hash: String(ref.render_input_hash || ""),
  };
}
function captureCoverConfig(config) {
  const result = emptyCoverConfig();
  for (const key of Object.keys(result))
    result[key] =
      typeof result[key] === "number"
        ? Number(config[key] || 0)
        : String(config[key] ?? result[key]);
  return result;
}
