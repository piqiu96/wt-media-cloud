import { describe, it, expect } from "vitest";
import { readGenerationStream } from "./generationStream.js";
function response(text, step = 1) {
  const bytes = new TextEncoder().encode(text);
  return new Response(
    new ReadableStream({
      start(c) {
        for (let n = 0; n < bytes.length; n += step)
          c.enqueue(bytes.slice(n, n + step));
        c.close();
      },
    }),
    { headers: { "Content-Type": "text/event-stream" } },
  );
}
const event = (name, data) =>
  `event: ${name}\r\ndata: ${JSON.stringify(data)}\r\n\r\n`;
describe("generation stream framing", () => {
  it("parses split Chinese bytes and CRLF frames, not chunks", async () => {
    const text =
      event("start", { total: 1 }) +
      event("item_result", { result: { title: "真实中文标题" } }) +
      event("done", { total: 1, success: 1, failed: 0 });
    await expect(readGenerationStream(response(text))).resolves.toEqual({
      title: "真实中文标题",
    });
  });
  it("rejects incomplete and duplicate results", async () => {
    const item = event("item_result", { result: { title: "保留" } });
    await expect(readGenerationStream(response(item))).rejects.toThrow("中断");
    await expect(readGenerationStream(response(item + item))).rejects.toThrow(
      "重复",
    );
  });
  it("keeps per-layer failure details for correction", async () => {
    const text =
      event("item_error", {
        message: "标题溢出",
        details: { issues: [{ layerId: "title" }] },
      }) + event("done", { total: 1, success: 0, failed: 1 });
    try {
      await readGenerationStream(response(text));
      throw new Error("accepted failure");
    } catch (e) {
      expect(e.details.issues[0].layerId).toBe("title");
    }
  });
});
