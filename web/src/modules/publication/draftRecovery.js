import { DialogPlugin } from "tdesign-vue-next";

export function choosePublicationDraft(savedAt) {
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
      header: "发现未完成的发布任务配置",
      body: message,
      dialogClassName: "resource-feedback-dialog",
      width: "min(calc(100vw - 32px), 560px)",
      confirmBtn: { content: "继续编辑", theme: "primary" },
      cancelBtn: { content: "重新开始", theme: "default", variant: "outline" },
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
export function capturePublicationDraft(form, schedule, cover, pendingRequest) {
  const text = (value) => String(value ?? "");
  return {
    form: {
      request_key: text(form.request_key),
      platform: "baidu",
      category_id: text(form.category_id),
      media_account_id: text(form.media_account_id),
      composite_output_id: text(form.composite_output_id),
      content: {
        description: text(form.content.description),
        cover_file_id: text(form.content.cover_file_id),
        topics: text(form.content.topics),
        declaration: text(form.content.declaration),
        confirmed: !!form.content.confirmed,
      },
    },
    schedule: text(schedule),
    image_asset_id: text(cover?.image_asset_id),
    cover_origin: text(cover?.origin),
    cover_output_id: text(cover?.output_id),
    pending_request: pendingRequest
      ? JSON.parse(JSON.stringify(pendingRequest))
      : null,
  };
}
