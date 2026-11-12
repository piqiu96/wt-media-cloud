import { it, expect } from "vitest";
import { coverPreviewState } from "../coverPreview.js";
import { coverSignature } from "../covers.js";
it("uses actual output files for each Bilibili cover location", () => {
  const row = {
    platform: "B站",
    coverMode: "模板合成",
    coverVariants: { "4:3": "four", "16:9": "wide" },
  };
  row.coverSignature = coverSignature(row);
  expect(coverPreviewState(row).images.map((i) => i.src)).toEqual([
    "four",
    "wide",
  ]);
});
it("never presents an old cover as the current template output", () => {
  const row = {
    platform: "小红书",
    coverMode: "模板合成",
    cover: "old",
    shortTitle: "changed",
    coverSignature: "old",
    coverVariants: { "3:4": "old output" },
  };
  expect(coverPreviewState(row)).toMatchObject({
    pending: true,
    images: [{ src: "" }],
  });
});
it("does not relabel a different ratio as a missing generated image", () => {
  const row = {
    platform: "B站",
    coverMode: "模板合成",
    cover: "four",
    coverVariants: { "4:3": "four" },
  };
  row.coverSignature = coverSignature(row);
  expect(coverPreviewState(row).images[1].src).toBe("");
});
it("does not invent a platform size for a source-only preview", () => {
  expect(
    coverPreviewState({
      platform: "百家号",
      coverMode: "现成封面",
      cover: "image",
    }).images,
  ).toEqual([{ key: "source", label: "当前封面", src: "image" }]);
});
