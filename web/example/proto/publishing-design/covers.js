import { fitImage } from "../../../src/shared/ui/media/imageOutput.js";
import { layoutText } from "../../../src/shared/ui/media/textLayout.js";
import { clone } from "./relationships.js";
const templateSlots = {
  main_title: { key: "shortTitle", label: "封面主标题" },
  sub_title: { key: "subTitle", label: "封面副标题" },
};
function textSlot(layer) {
  if (layer.visible === false || (layer.kind && layer.kind !== "text"))
    return null;
  return templateSlots[
    layer.slot ?? (layer.id === "title" ? "main_title" : "")
  ];
}
export function coverTextFields(doc) {
  const keys = new Set(
    (doc?.layers || [])
      .map(textSlot)
      .filter(Boolean)
      .map((field) => field.key),
  );
  return Object.values(templateSlots).filter((field) => keys.has(field.key));
}
export function coverTextFieldsForRow(row) {
  return [...new Map([row.templateDocument,...Object.values(row.templateDocuments || {})].flatMap(coverTextFields).map(field=>[field.key,field])).values()];
}
export function coverTextErrors(row) {
  if (!["模板合成", "规则生成封面"].includes(row.coverMode)) return [];
  if (!row.templateDocument) return ["请选择封面模板"];
  return coverTextFieldsForRow(row)
    .filter((field) => !row[field.key]?.trim())
    .map((field) => `请填写${field.label}`);
}
export const isTemplateCover = row => ['模板合成', '规则生成封面'].includes(row.coverMode);
export function coverReadinessIssues(row) {
  if (!row.cover) return ['请选择封面'];
  if (row.coverError) return [row.coverError];
  if (isTemplateCover(row)) {
    const specs = coverSpecs(row.platform);
    const keys = specs.length ? specs.map(spec => spec.key) : ['source'];
    if (row.coverSignature !== coverSignature(row) || keys.some(key => !row.coverVariants?.[key]))
      return ['请先生成当前配置的全部封面'];
  } else if (row.coverSource && row.cover !== row.coverSource && row.coverRender === '待重新合成') {
    return ['请重新选用当前封面图片'];
  }
  return [];
}
export function inspectCoverImage(src) {
  return new Promise((resolve,reject) => {
    const image = new Image();
    const timer=setTimeout(()=>reject(Error('图片读取超时，请重试或更换图片')),10000);
    image.onload=()=>{clearTimeout(timer);image.naturalWidth && image.naturalHeight ? resolve({width:image.naturalWidth,height:image.naturalHeight}) : reject(Error('图片尺寸无效'));};
    image.onerror=()=>{clearTimeout(timer);reject(Error('图片读取失败，请更换图片'));};
    image.src=src;
  });
}
export const coverSpecs = (p) =>
  p === "B站"
    ? [
        { key: "4:3", label: "推荐展示 · 4:3", width: 1200, height: 900 },
        { key: "16:9", label: "个人空间 · 16:9", width: 1600, height: 900 },
      ]
    : p === "小红书"
      ? [{ key: "3:4", label: "笔记封面 · 3:4", width: 1080, height: 1440 }]
      : [];
export const coverSignature = (r) =>
  JSON.stringify([
    r.coverMode,
    r.coverSource,
    r.shortTitle,
    r.subTitle,
    r.templateId,
    r.templateVersion,
    r.templateDocument,
    r.templateDocuments,
    r.templateSnapshots,
    r.platform,
  ]);
export function defaultDocument(width = 1200, height = 900, image = "") {
  return {
    width,
    height,
    background: "#172f4c",
    image,
    imageFit: "slice",
    safe: 24,
    layers: [
      {
        id: "title",
        slot: "main_title",
        kind: "text",
        name: "动态主标题",
        text: "精彩实战指南",
        x: width * 0.08,
        y: height * 0.64,
        w: width * 0.84,
        h: height * 0.26,
        fontSize: width * 0.075,
        minFont: Math.round(width * 0.032),
        fontFamily: "sans-serif",
        weight: "700",
        color: "#ffffff",
        align: "left",
        lineHeight: 1.15,
        maxLines: 2,
        maxChars: 22,
        wrap: true,
        autoShrink: true,
        stroke: 3,
        visible: true,
        locked: false,
      },
    ],
  };
}
export const supportedFonts = ["sans-serif", "serif", "monospace", "system-ui"];
export function validateDocument(doc) {
  if (!doc.image && !["solid", "business"].includes(doc.backgroundMode)) throw Error("缺少底图，请从图片库选择或使用已有封面");
  if (
    !Number.isFinite(doc.width) ||
    !Number.isFinite(doc.height) ||
    doc.width < 1 ||
    doc.height < 1
  )
    throw Error("画布尺寸无效");
  for (const l of doc.layers || []) {
    if (l.visible === false) continue;
    const safe = doc.safe ?? 16;
    if (
      ![l.x, l.y, l.w, l.h].every(Number.isFinite) ||
      l.w <= 0 ||
      l.h <= 0 ||
      (l.kind !== "decoration" && (l.x < safe ||
      l.y < safe ||
      l.x + l.w > doc.width - safe ||
      l.y + l.h > doc.height - safe))
    )
      throw Error(l.name + " 超出安全区");
    if (
      l.kind !== "decoration" &&
      !supportedFonts.includes(l.fontFamily || "sans-serif")
    )
      throw Error("字体缺失或未验证：" + l.fontFamily + "，请选择系统字体");
  }
  return true;
}
const esc = (s) =>
  String(s ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&apos;",
      })[c],
  );
export async function renderDocument(
  doc,
  {
    shortTitle = null,
    subTitle = null,
    width = doc.width,
    height = doc.height,
  } = {},
) {
  validateDocument(doc);
  await document.fonts.ready;
  if (Math.abs(width / height - doc.width / doc.height) > 0.01)
    throw Error("模板比例不匹配，请选择对应比例模板");
  const ctx = document.createElement("canvas").getContext("2d");
  const layers = [];
  for (const layer of doc.layers || []) {
    if (layer.visible === false) continue;
    const l = { ...layer };
    const field = textSlot(l);
    if (field?.key === "shortTitle") l.text = shortTitle ?? l.text;
    if (field?.key === "subTitle") l.text = subTitle ?? l.text;
    const safe = doc.safe ?? 16;
    if (
      (l.kind !== "decoration" && (l.x < safe ||
      l.y < safe ||
      l.x + l.w > doc.width - safe ||
      l.y + l.h > doc.height - safe))
    )
      throw Error(l.name + " 超出安全区");
    if (l.kind === "decoration") {
      layers.push(
        `<rect x="${l.x}" y="${l.y}" width="${l.w}" height="${l.h}" fill="${esc(l.color)}" opacity="${l.opacity ?? 1}"/>`,
      );
      continue;
    }
    const family = l.fontFamily || "sans-serif";
    if (
      !["sans-serif", "serif", "monospace", "system-ui"].includes(family) &&
      !document.fonts.check(`16px "${family}"`)
    )
      throw Error("字体缺失：" + family);
    const result = layoutText(
      {
        ...l,
        fontSize: Math.round(l.fontSize || 48),
        minFont: Math.round(l.minFont || 12),
      },
      (s, size) => {
        ctx.font = `${l.weight || 400} ${size}px ${family}`;
        return ctx.measureText(s).width;
      },
    );
    if (result.overflow)
      throw Error(l.name + " 文字溢出，请缩短短标题或调整文字区域");
    const x =
      l.x + (l.align === "center" ? l.w / 2 : l.align === "right" ? l.w : 0);
    layers.push(
      `<text font-family="${esc(family)}" font-size="${result.size}" font-weight="${esc(l.weight)}" fill="${esc(l.color)}" stroke="${l.stroke ? esc(l.strokeColor || "#000") : "none"}" stroke-width="${l.stroke || 0}" paint-order="stroke" letter-spacing="${l.spacing || 0}" style="${l.shadow ? "filter:drop-shadow(3px 3px 4px #0009)" : ""}">${result.lines.map((line, i) => `<tspan x="${x}" y="${l.y + result.size + i * result.lineHeight}" text-anchor="${l.align === "center" ? "middle" : l.align === "right" ? "end" : "start"}">${esc(line)}</tspan>`).join("")}</text>`,
    );
  }
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${doc.width} ${doc.height}"><rect width="${doc.width}" height="${doc.height}" fill="${esc(doc.background)}"/>${doc.image ? `${doc.image ? `${doc.image ? `${doc.image ? `${doc.image ? `${doc.image ? `${doc.image ? `${doc.image ? `${doc.image ? `${doc.image ? `${doc.image ? `${doc.image ? `${doc.image ? `${doc.image ? `<image href="${esc(doc.image)}" width="${doc.width}" height="${doc.height}" preserveAspectRatio="${esc(doc.imageAnchor || "xMidYMid")} ${esc(doc.imageFit || "slice")}"/>` : ""}` : ""}` : ""}` : ""}` : ""}` : ""}` : ""}` : ""}` : ""}` : ""}` : ""}` : ""}` : ""}` : ""}${layers.join("")}</svg>`;
  return fitImage(
    "data:image/svg+xml;charset=utf-8," + encodeURIComponent(svg),
    width,
    height,
  );
}
export async function ensureCoverResults(row) {
  row.coverSource ||= row.cover;
  if (!row.coverSource) throw Error("缺少封面或底图");
  if (!isTemplateCover(row)) {
    await inspectCoverImage(row.coverSource);
    if (row.cover !== row.coverSource) {row.coverApproved=false;row.contentApproved=false;}
    row.cover=row.coverSource;row.coverVariants={};row.coverSignature=null;row.coverRender='已选用';row.coverError='';
    return;
  }
  const sig = coverSignature(row);
  if (row.coverSignature === sig && !coverReadinessIssues(row).length)
    return;
  row.coverApproved = false;
  row.coverRender = "合成中";
  const specs = coverSpecs(row.platform);
  const outputs = {};
  try {
    if (row.coverMode === "模板合成" || row.coverMode === "规则生成封面") {
      const errors = coverTextErrors(row);
      if (errors.length) throw Error(errors.join("；"));
      const doc = clone(row.templateDocument);
      doc.image = doc.backgroundMode === "solid" ? "" : doc.backgroundMode === "fixed" ? doc.image : row.coverSource;
      const targets = specs.length
        ? specs
        : [{ key: "source", width: doc.width, height: doc.height }];
      for (const spec of targets) {
        const matching = row.templateDocuments?.[spec.key] || doc;
        try {
          outputs[spec.key] = await renderDocument(
            { ...matching, image: matching.backgroundMode === "solid" ? "" : matching.backgroundMode === "fixed" ? matching.image : row.coverSource },
            {
              shortTitle: row.shortTitle,
              subTitle: row.subTitle,
              width: spec.width,
              height: spec.height,
            },
          );
        } catch (e) {
          throw Error(
            `${spec.label || spec.key}：${e.message}；请调整此比例封面或选择对应模板`,
          );
        }
      }
    } else {
      for (const spec of specs)
        outputs[spec.key] = await fitImage(
          row.coverSource,
          spec.width,
          spec.height,
        );
      if (!specs.length) {
        const dimensions = await new Promise((resolve, reject) => {
          const im = new Image();
          im.onload = () => resolve([im.naturalWidth, im.naturalHeight]);
          im.onerror = () => reject(Error("图片读取失败，请更换图片"));
          im.src = row.coverSource;
        });
        outputs.source = await fitImage(row.coverSource, ...dimensions);
      }
    }
    if (coverSignature(row) !== sig)
      throw Error("输入已改变，本次封面结果作废");
    row.coverVariants = outputs;
    row.cover = Object.values(outputs)[0];
    row.coverSignature = sig;
    row.coverRender = "已合成";
    row.coverError = "";
  } catch (e) {
    row.coverRender = "失败";
    row.coverError = e.message;
    throw e;
  }
}
