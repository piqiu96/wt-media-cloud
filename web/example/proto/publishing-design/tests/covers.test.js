import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { fitImage } from "../../../../src/shared/ui/media/imageOutput.js";
import {
  coverSpecs,
  coverTextFields,
  coverTextErrors,
  defaultDocument,
  ensureCoverResults,
  renderDocument,
} from "../covers.js";

vi.mock("../../../../src/shared/ui/media/imageOutput.js", () => ({
  fitImage: vi.fn(async (src, width, height) => `png:${width}x${height}`),
}));
beforeEach(() => {
  vi.stubGlobal("document", {
    fonts: { ready: Promise.resolve(), check: () => true },
    createElement: () => ({
      getContext: () => ({
        measureText: (text) => ({ width: Array.from(text).length * 24 }),
      }),
    }),
  });
  fitImage.mockClear();
});
afterEach(() => vi.unstubAllGlobals());

it.each([
  [1200, 900],
  [1600, 900],
  [1080, 1440],
])(
  "uses the shared text layout and exact %ix%i export dimensions",
  async (width, height) => {
    const doc = defaultDocument(width, height, "data:image/png;base64,source");
    expect(await renderDocument(doc, { shortTitle: "当前短标题" })).toBe(
      `png:${width}x${height}`,
    );
    const svg = decodeURIComponent(fitImage.mock.calls[0][0]);
    expect(svg).toContain("当前短标题");
    expect(svg).not.toContain("精彩实战指南");
    expect(svg).toContain(`viewBox="0 0 ${width} ${height}"`);
  },
);
it("rejects overflowing titles and mismatched ratios before image export", async () => {
  const doc = defaultDocument(1200, 900, "image");
  await expect(
    renderDocument(doc, { shortTitle: "超长".repeat(20) }),
  ).rejects.toThrow("溢出");
  await expect(
    renderDocument(doc, { width: 1080, height: 1440 }),
  ).rejects.toThrow("比例不匹配");
  expect(fitImage).not.toHaveBeenCalled();
});
it("renders both Bilibili variants from their versioned ratio documents", async () => {
  const row = {
    platform: "B站",
    coverMode: "模板合成",
    coverSource: "image",
    shortTitle: "短标题",
    templateDocument: defaultDocument(1200, 900, "image"),
    templateDocuments: { "16:9": defaultDocument(1600, 900, "image") },
  };
  await ensureCoverResults(row);
  expect(row.coverVariants).toEqual({
    "4:3": "png:1200x900",
    "16:9": "png:1600x900",
  });
  expect(row.coverApproved).toBe(false);
  expect(coverSpecs("百家号")).toEqual([]);
});
it("locates the missing Bilibili ratio and clears its error after repair", async () => {
  const row = {
    platform: "B站",
    coverMode: "模板合成",
    coverSource: "image",
    shortTitle: "短标题",
    templateDocument: defaultDocument(1200, 900, "image"),
  };
  await expect(ensureCoverResults(row)).rejects.toThrow("个人空间 · 16:9");
  expect(row.coverRender).toBe("失败");
  row.templateDocuments = { "16:9": defaultDocument(1600, 900, "image") };
  await ensureCoverResults(row);
  expect(row.coverError).toBe("");
  expect(row.coverRender).toBe("已合成");
});
it("discards an in-flight cover if the input changes before completion", async () => {
  let complete;
  fitImage.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        complete = resolve;
      }),
  );
  const row = {
    platform: "小红书",
    coverMode: "模板合成",
    coverSource: "image",
    shortTitle: "旧",
    templateDocument: defaultDocument(1080, 1440, "image"),
    coverApproved: true,
  };
  const work = ensureCoverResults(row);
  await vi.waitFor(() => expect(complete).toBeTypeOf("function"));
  row.shortTitle = "新";
  complete("old PNG");
  await expect(work).rejects.toThrow("输入已改变");
  expect(row.coverApproved).toBe(false);
  expect(row.coverVariants).toBeUndefined();
});

it("derives required fields from visible template slots and renders both entered titles", async () => {
  const doc = defaultDocument(1080, 1440, "image");
  doc.layers.push({
    ...doc.layers[0],
    id: "subtitle",
    slot: "sub_title",
    name: "副标题",
    y: 300,
  });
  const row = {
    platform: "小红书",
    coverMode: "模板合成",
    coverSource: "image",
    shortTitle: "主标题输入",
    templateDocument: doc,
  };
  await expect(ensureCoverResults(row)).rejects.toThrow("请填写封面副标题");
  row.subTitle = "副标题输入";
  await ensureCoverResults(row);
  const svg = decodeURIComponent(fitImage.mock.calls[0][0]);
  expect(svg).toContain("主标题输入");
  expect(svg).toContain("副标题输入");
});
it("does not require a title for hidden or fixed text layers", async () => {
  const doc = defaultDocument(1080, 1440, "image");
  doc.layers[0].slot = "";
  doc.layers.push({
    ...doc.layers[0],
    id: "subtitle",
    slot: "sub_title",
    visible: false,
  });
  await ensureCoverResults({
    platform: "小红书",
    coverMode: "模板合成",
    coverSource: "image",
    templateDocument: doc,
  });
  expect(decodeURIComponent(fitImage.mock.calls[0][0])).toContain(
    "精彩实战指南",
  );
});

it("orders main then subtitle independently of layer painting order, and ignores duplicate slots", () => {
  const main = defaultDocument().layers[0];
  const sub = { ...main, id: "subtitle", slot: "sub_title" };
  const doc = { layers: [sub, main, { ...main, id: "main-again" }] };
  expect(coverTextFields(doc).map((f) => f.key)).toEqual([
    "shortTitle",
    "subTitle",
  ]);
  expect(
    coverTextErrors({
      coverMode: "模板合成",
      templateDocument: { layers: [sub] },
      subTitle: "只需副标题",
    }),
  ).toEqual([]);
});
