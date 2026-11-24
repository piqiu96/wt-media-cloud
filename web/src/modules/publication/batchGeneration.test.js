import { expect, it } from "vitest";
import { readGenerationStream } from "../../shared/api/generationStream.js";

function response(events) {
  return new Response(events.map(([event, data]) => `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`).join(""),
    { headers: { "content-type": "text/event-stream" } });
}

const rows = [{ row_key: "a", input_revision: "1", field_revisions: { description: "3" } },
  { row_key: "b", input_revision: "1", field_revisions: { description: "0" } }];

it("returns each matched render or generation result and reason", async () => {
  const success = { ...rows[0], result: { content: { description: "内容" } } };
  const failure = { ...rows[1], message: "资源已变化" };
  const result = await readGenerationStream(response([
    ["item_error", failure], ["item_result", success], ["done", { total: 2, success: 1, failed: 1 }],
  ]), undefined, rows);
  expect(result).toEqual([{ event: "item_error", data: failure }, { event: "item_result", data: success }]);
});

it("rejects stale or incomplete batch stream frames", async () => {
  await expect(readGenerationStream(response([
    ["item_result", { ...rows[0], input_revision: "2" }],
  ]), undefined, rows)).rejects.toThrow("行或版本");
  await expect(readGenerationStream(response([
    ["item_result", rows[0]],
  ]), undefined, rows)).rejects.toThrow("连接中断");
});
