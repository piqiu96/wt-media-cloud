import { defaultDocument, renderDocument, supportedFonts } from "./covers.js";
import { layoutText } from "../../../src/shared/ui/media/textLayout.js";
const copy = (value) => JSON.parse(JSON.stringify(value));
const sizes = [
  { label: "3:4 · 1080 × 1440", value: "1080x1440" },
  { label: "4:3 · 1200 × 900", value: "1200x900" },
  { label: "16:9 · 1920 × 1080", value: "1920x1080" },
  { label: "9:16 · 1080 × 1920", value: "1080x1920" },
  { label: "1:1 · 1080 × 1080", value: "1080x1080" }
];
const samples = { short: "精彩实战指南", long: "新手进阶必看：从基础操作到实战技巧的完整攻略" };
const isPublished = (row) => !!row.document && Number(row.version) > 0;
const contentOf = (row) => copy(row.draft?.document || row.document);
const dimensions = (row) => {
  const d = contentOf(row);
  return `${d.width} × ${d.height}`;
};
function createTemplate({ id, name, size = "1080x1440", categories = [], platforms = [] }) {
  if (!name?.trim()) throw new Error("请填写模板名称");
  if (!sizes.some((s) => s.value === size)) throw new Error("请选择支持的画布规格");
  const [w, h] = size.split("x").map(Number), document2 = defaultDocument(w, h);
  document2.backgroundMode = "solid";
  document2.layers[0].maxChars = 0;
  return { id, name: name.trim(), categories: copy(categories), platforms: copy(platforms), contentModel: 1, version: 0, document: null, enabled: false, preview: "", ratio: `${w}×${h}`, draft: { document: document2, preview: "", savedAt: (/* @__PURE__ */ new Date()).toISOString() }, createdAt: (/* @__PURE__ */ new Date()).toISOString(), updatedAt: (/* @__PURE__ */ new Date()).toISOString() };
}
function saveDraft(row, document2, preview = "") {
  const old = contentOf(row);
  if (old.width !== document2.width || old.height !== document2.height) throw new Error("画布规格已固定，请复制为新规格");
  if (!Array.isArray(document2.layers) || !Number.isFinite(Number(document2.safe))) throw new Error("设计数据无效");
  const same = isPublished(row) && JSON.stringify(row.document) === JSON.stringify(document2);
  return { ...copy(row), contentModel: 1, draft: same ? null : { document: copy(document2), preview, savedAt: (/* @__PURE__ */ new Date()).toISOString() }, updatedAt: (/* @__PURE__ */ new Date()).toISOString() };
}
function publishDraft(row, preview) {
  if (!row.draft) throw new Error("没有待发布草稿");
  return { ...copy(row), contentModel: 1, document: copy(row.draft.document), preview, version: (Number(row.version) || 0) + 1, draft: null, enabled: isPublished(row) ? !!row.enabled : false, updatedAt: (/* @__PURE__ */ new Date()).toISOString() };
}
function setUsage(row, enabled) {
  if (!isPublished(row)) throw new Error("请先发布内容");
  return { ...copy(row), enabled: !!enabled, updatedAt: (/* @__PURE__ */ new Date()).toISOString() };
}
function cloneTemplate(row, fields) {
  const next = createTemplate(fields), source = contentOf(row), target = contentOf(next), sx = target.width / source.width, sy = target.height / source.height;
  Object.assign(target, copy(source), { width: target.width, height: target.height });
  target.layers.forEach((l) => {
    l.x = Math.round(l.x * sx);
    l.y = Math.round(l.y * sy);
    l.w = Math.round(l.w * sx);
    l.h = Math.round(l.h * sy);
  });
  next.draft.document = target;
  return next;
}
function inspectDocument(doc, sample = null, measure = null) {
  const issues = [], add = (level, code, message, l) => issues.push({ level, code, message, layerId: l?.id || "background", layerName: l?.name || "背景" });
  const titles = doc.layers.filter((l) => l.kind === "text" && l.slot === "main_title");
  if (titles.length !== 1 || !titles[0]?.visible) add("error", "title", "必须有且仅有一个可见的动态主标题", titles[0]);
  if (doc.layers.some((l) => l.slot === "sub_title")) add("error", "legacy", "副标题绑定不属于 V1，请改为固定文字", doc.layers.find((l) => l.slot === "sub_title"));
  for (const l of doc.layers.filter((l2) => l2.visible)) {
    if (![l.x, l.y, l.w, l.h].every(Number.isFinite) || l.w <= 0 || l.h <= 0) {
      add("error", "bounds", "位置或尺寸无效", l);
      continue;
    }
    const safe = Number(doc.safe) || 0, outside = l.x < safe || l.y < safe || l.x + l.w > doc.width - safe || l.y + l.h > doc.height - safe;
    if (l.kind === "decoration") {
      if (outside) add("warning", "crop", "装饰可能被裁切，请确认效果", l);
      continue;
    }
    if (outside) add("error", "safe", "文字超出画布或安全区", l);
    if (!supportedFonts.includes(l.fontFamily || "sans-serif")) add("error", "font", "字体不受支持", l);
    const text = l.slot === "main_title" ? sample ?? samples.short : l.text;
    if (!String(text || "").trim()) add("error", "empty", "文字内容不能为空", l);
    const count = Array.from(String(text || "")).length, limit = Number(l.maxChars) || 0;
    if (limit > 0 && count > limit) {
      add("error", "overflow", `共 ${count} 字，超过上限 ${limit} 字，请调整「排版规则 → 最多字数」`, l);
      issues[issues.length - 1].field = "maxChars";
    } else if (measure && layoutText({ ...l, text }, (s, size) => measure(s, size, l)).overflow) {
      add("error", "overflow", "文字区域不足：增加区域、开启换行或调整字号", l);
      issues[issues.length - 1].field = "minFont";
    }
  }
  if (doc.backgroundMode === "fixed" && !doc.image) add("error", "image", "请选择固定背景图片");
  if (!Number.isFinite(Number(doc.safe)) || doc.safe < 0 || doc.safe * 2 >= Math.min(doc.width, doc.height)) add("error", "safe", "安全区数值无效");
  return issues;
}
async function checkDocument(doc, { sample = samples.short, businessImage = "", allSamples = false } = {}) {
  const context = document.createElement("canvas").getContext("2d");
  await document.fonts.ready;
  const measure = (s, size, l) => {
    context.font = `${l.weight || 400} ${size}px ${l.fontFamily || "sans-serif"}`;
    return context.measureText(s).width;
  };
  let issues = [];
  for (const text of allSamples ? [samples.short, samples.long] : [sample]) {
    for (const issue of inspectDocument(doc, text, measure)) {
      if (issues.some((i) => i.code === issue.code && i.layerId === issue.layerId)) continue;
      if (allSamples && issue.code === "overflow" && doc.layers.some(l => l.id === issue.layerId && l.slot === "main_title")) {
        issue.message = `${text === samples.short ? "精简样例" : "较长样例"}：${issue.message}`;
      }
      issues.push(issue);
    }
  }
  let image = "";
  const renderDoc = copy(doc);
  if (renderDoc.backgroundMode === "solid") renderDoc.image = "";
  if (renderDoc.backgroundMode === "business") {
    renderDoc.image = businessImage;
    if (!businessImage) issues.push({ level: "warning", code: "business", layerId: "background", layerName: "背景", message: "业务底图未选择预览样例，当前显示背景色" });
  }
  if (renderDoc.image && !issues.some((i) => i.level === "error")) {
    try {
      await new Promise((resolve, reject) => {
        const image2 = new Image(), timer = setTimeout(() => reject(new Error("图片读取超时")), 8e3);
        image2.onload = () => {
          clearTimeout(timer);
          resolve();
        };
        image2.onerror = () => {
          clearTimeout(timer);
          reject(new Error("背景图片已失效，请重新选择"));
        };
        image2.src = renderDoc.image;
      });
    } catch (e) {
      issues.push({ level: "error", code: "image", layerId: "background", layerName: "背景", message: e.message });
    }
  }
  if (!issues.some((i) => i.level === "error")) {
    try {
      image = await renderDocument(renderDoc, { shortTitle: sample });
    } catch (e) {
      issues.push({ level: "error", code: "render", layerId: "background", layerName: "背景", message: e.message });
    }
  }
  return { issues, image };
}
export {
  checkDocument,
  cloneTemplate,
  contentOf,
  copy,
  createTemplate,
  dimensions,
  inspectDocument,
  isPublished,
  publishDraft,
  samples,
  saveDraft,
  setUsage,
  sizes
};
