// Shared by canvas preview and exported artwork. Units are design pixels.
export function layoutText(layer, measure) {
  const text = String(layer.text || ""),
    width = Math.max(1, Number(layer.w) || 1),
    height = Math.max(1, Number(layer.h) || 1);
  const spacing = Number(layer.spacing) || 0,
    lineHeight = Math.max(1, Number(layer.lineHeight) || 1.3);
  const min = Math.max(1, Number(layer.minFont) || 12),
    max = Math.max(min, Number(layer.fontSize) || 48);
  for (let size = max; size >= min; size--) {
    const lines = [];
    for (const paragraph of text.split("\n")) {
      let line = "";
      for (const ch of Array.from(paragraph)) {
        if (
          line &&
          layer.wrap !== false &&
          measure(line + ch, size) +
            Math.max(0, Array.from(line + ch).length - 1) * spacing >
            width
        ) {
          lines.push(line);
          line = "";
        }
        line += ch;
      }
      lines.push(line);
    }
    const overflow =
      lines.length > Math.max(1, Number(layer.maxLines) || 3) ||
      lines.length * size * lineHeight > height ||
      lines.some(
        (l) =>
          measure(l, size) + Math.max(0, Array.from(l).length - 1) * spacing >
          width,
      ) ||
      (Number(layer.maxChars) > 0 &&
        Array.from(text).length > Number(layer.maxChars));
    if (!overflow || layer.autoShrink === false || size === min)
      return { lines, size, overflow, lineHeight: size * lineHeight };
  }
}
