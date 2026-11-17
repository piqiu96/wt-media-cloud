import { describe, it, expect } from "vitest";
import { createTemplate, saveDraft, publishDraft, setUsage, cloneTemplate, contentOf, inspectDocument } from "../coverTemplates.js";
const base = () => createTemplate({ id: 99, name: "测试", size: "1080x1440", categories: [], platforms: [] });
describe("cover template V1 lifecycle", () => {
  it("allows incomplete drafts, locks basics, and never enables on first publish", () => {
    let row = base();
    expect(row).not.toHaveProperty("isDefault");
    const doc = contentOf(row);
    doc.layers = [];
    row = saveDraft(row, doc);
    expect(row.version).toBe(0);
    expect(row.enabled).toBe(false);
    expect(inspectDocument(doc).some((i) => i.code === "title")).toBe(true);
    expect(() => setUsage(row, true)).toThrow();
    expect(() => saveDraft(row, { ...doc, width: 100 })).toThrow();
  });
  it("keeps published content and switch unchanged when editing and publishing updates", () => {
    let row = publishDraft(base(), "first");
    expect(row.enabled).toBe(false);
    row = setUsage(row, true);
    const before = JSON.stringify(row.document);
    const doc = contentOf(row);
    doc.layers[0].color = "#ff0000";
    row = saveDraft(row, doc);
    expect(JSON.stringify(row.document)).toBe(before);
    expect(row.preview).toBe("first");
    row = publishDraft(row, "next");
    expect(row.enabled).toBe(true);
    expect(row.draft).toBe(null);
    expect(row.version).toBe(2);
  });
  it("clones to a disabled unpublished size without deforming glyphs or mutating source", () => {
    const row = setUsage(publishDraft(base(), "image"), true);
    row.isDefault = true; // Legacy designation is not carried into a new template.
    const before = JSON.stringify(row);
    const next = cloneTemplate(row, { id: 100, name: "副本", size: "1920x1080", categories: [], platforms: [] });
    expect(next.enabled).toBe(false);
    expect(next.version).toBe(0);
    expect(next).not.toHaveProperty("isDefault");
    expect(contentOf(next).width).toBe(1920);
    expect(contentOf(next).layers[0].fontSize).toBe(contentOf(row).layers[0].fontSize);
    expect(JSON.stringify(row)).toBe(before);
  });
  it("requires visible unique dynamic title, permits solid backgrounds and warns on decorations", () => {
    const doc = contentOf(base());
    expect(inspectDocument(doc).filter((i) => i.level === "error")).toEqual([]);
    doc.layers.push({ ...doc.layers[0], id: "second" });
    expect(inspectDocument(doc).some((i) => i.code === "title")).toBe(true);
    doc.layers.pop();
    doc.layers.push({ id: "d", kind: "decoration", name: "装饰", x: 0, y: 0, w: 300, h: 100, visible: true });
    expect(inspectDocument(doc).find((i) => i.layerId === "d").level).toBe("warning");
  });
});
it("keeps confirmed PNGs and snapshots when their source template is disabled", async () => {
  const { ensureCoverResults, coverSignature } = await import("../covers.js");
  const template = setUsage(publishDraft(base(), "preview"), true);
  const row = { coverMode: "模板合成", coverSource: "confirmed-source", cover: "confirmed-png", shortTitle: "实际标题", templateId: template.id, templateVersion: template.version, templateDocument: contentOf(template), coverVariants: { source: "confirmed-png" }, coverApproved: true };
  row.coverSignature = coverSignature(row);
  const snapshot = JSON.stringify(row);
  expect(setUsage(template, false).enabled).toBe(false);
  await ensureCoverResults(row);
  expect(row.coverVariants).toEqual({ source: "confirmed-png" });
  expect(JSON.stringify(row)).toBe(snapshot);
});
it("protects images referenced only by an editing draft", async () => {
  const { imageReferences } = await import("../library.js");
  const row = base();
  row.draft.document.image = "draft-image";
  expect(imageReferences({ templates: [row], tasks: [] }, { id: 1, src: "draft-image" })).toEqual([{ kind: "模板", id: row.id, name: row.name }]);
});
