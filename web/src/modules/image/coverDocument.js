export const copy = (value) => JSON.parse(JSON.stringify(value));
export const samples = {
  short: "精彩实战指南",
  long: "新手进阶必看：从基础操作到实战技巧的完整攻略",
};
export const sizes = [
  { label: "3:4 · 1080 × 1440", value: "1080x1440" },
  { label: "4:3 · 1200 × 900", value: "1200x900" },
  { label: "16:9 · 1920 × 1080", value: "1920x1080" },
  { label: "9:16 · 1080 × 1920", value: "1080x1920" },
  { label: "1:1 · 1080 × 1080", value: "1080x1080" },
];
export const isPublished = (row) => row.published_revision > 0;
export const contentOf = (row) =>
  copy(row.draft_config || row.published_config);
export const fontFamilyMap = {
  "sans-serif": "WT Cover Sans",
  "system-ui": "WT Cover Sans",
  serif: "WT Cover Serif",
  monospace: "WT Cover Mono",
};
const loaded = new Map();
export async function ensureImageFonts(doc, client) {
  const fonts = new Map(
    (doc.layers || [])
      .filter((l) => l.kind === "text" && l.visible)
      .map((l) => [`${l.fontFamily}/${l.weight}`, l]),
  );
  await Promise.all(
    [...fonts].map(([key, layer]) => {
      if (!fontFamilyMap[layer.fontFamily]) throw new Error("字体不受支持");
      if (!loaded.has(key))
        loaded.set(
          key,
          (async () => {
            const data = await client.getImageFont(
              layer.fontFamily,
              layer.weight,
            );
            const face = new FontFace(fontFamilyMap[layer.fontFamily], data, {
              weight: layer.weight,
            });
            await face.load();
            document.fonts.add(face);
          })().catch((e) => {
            loaded.delete(key);
            throw e;
          }),
        );
      return loaded.get(key);
    }),
  );
}
// A resource URL is display state. Persist the authorized asset identity only.
export function documentInput(doc) {
  const input = copy(doc);
  input.image = "";
  if (input.backgroundMode !== "fixed") delete input.imageAssetId;
  return input;
}
