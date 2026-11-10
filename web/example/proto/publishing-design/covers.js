import { fitImage } from "../../../src/shared/ui/media/imageOutput.js";
export const coverSpecs = (platform) =>
  platform === "B站"
    ? [
        { key: "4:3", label: "推荐展示 · 4:3", width: 1200, height: 900 },
        { key: "16:9", label: "个人空间 · 16:9", width: 1600, height: 900 },
      ]
    : platform === "小红书"
      ? [{ key: "3:4", label: "笔记封面 · 3:4", width: 1080, height: 1440 }]
      : [];
export async function ensureCoverResults(row) {
  const src = row.coverSource || row.cover;
  if (!src) return;
  row.coverSource = src;
  const current = { ...(row.coverVariants || {}) };
  for (const spec of coverSpecs(row.platform))
    if (!current[spec.key])
      current[spec.key] = await fitImage(src, spec.width, spec.height);
  row.coverVariants = current;
  const first = coverSpecs(row.platform)[0];
  if (first) row.cover = current[first.key];
}
