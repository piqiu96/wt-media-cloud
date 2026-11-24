// Decode complete SSE frames across arbitrary network chunks, including CRLF and
// split multi-byte characters. A response without done is never a success.
export async function readGenerationStream(response, onEvent = () => {}, expectedItems) {
  if (
    !response.body ||
    !response.headers.get("content-type")?.includes("text/event-stream")
  )
    throw new Error("生成接口未返回有效事件流");
  const reader = response.body.getReader(),
    decoder = new TextDecoder("utf-8", { fatal: true });
  let buffer = "",
    terminal = false,
    done = false,
    result,
    failure;
  const expected = expectedItems && new Map(expectedItems.map(item => [item.row_key, item]));
  const completed = new Map();
  let succeeded = 0;
  function consumeBatch(event, value) {
    if (event === "item_result" || event === "item_error") {
      const input = expected.get(value.row_key);
      const versions = value.field_revisions;
      if (!input || input.input_revision !== value.input_revision || !versions ||
        Object.keys(versions).length !== Object.keys(input.field_revisions).length ||
        Object.entries(input.field_revisions).some(([field, version]) => versions[field] !== version)) {
        throw new Error("生成结果与请求的行或版本不一致");
      }
      if (completed.has(value.row_key)) throw new Error("生成接口重复返回结果");
      completed.set(value.row_key, { event, data: value });
      if (event === "item_result") succeeded++;
    }
    if (event === "error") throw new Error(value.message || "生成中断");
    if (event === "done") {
      if (value.total !== expected.size || value.total !== completed.size ||
        value.success !== succeeded || value.failed !== completed.size - succeeded) throw new Error("生成结果不完整");
      done = true;
    }
  }
  function consume(frame) {
    let event = "message";
    const data = [];
    for (const line of frame.split("\n")) {
      if (line.startsWith("event:")) event = line.slice(6).trim();
      if (line.startsWith("data:")) data.push(line.slice(5).trimStart());
    }
    if (!data.length) return;
    const value = JSON.parse(data.join("\n"));
    if (expected) {
      consumeBatch(event, value);
      onEvent({ event, data: value });
      return;
    }
    if (event === "item_result" || event === "item_error") {
      if (terminal) throw new Error("生成接口重复返回结果");
      terminal = true;
      if (event === "item_result") result = value.result;
      else {
        failure = new Error(value.message || "生成失败");
        failure.details = value.details;
      }
    }
    if (event === "error") {
      failure = new Error(value.message || "生成中断");
    }
    if (event === "done") {
      if (
        !terminal ||
        value.total !== 1 ||
        value.success + value.failed !== 1 ||
        value.success !== (failure ? 0 : 1)
      )
        throw new Error("生成结果不完整");
      done = true;
    }
    onEvent({ event, data: value });
  }
  try {
    while (true) {
      const chunk = await reader.read();
      buffer += decoder.decode(chunk.value, { stream: !chunk.done });
      // Normalize after appending, preserving a CR split across chunks.
      buffer = buffer.replace(/\r\n/g, "\n");
      let boundary;
      while ((boundary = buffer.indexOf("\n\n")) >= 0) {
        consume(buffer.slice(0, boundary));
        buffer = buffer.slice(boundary + 2);
      }
      if (chunk.done || done) break;
    }
    if (failure) throw failure;
    if (!done) throw new Error("生成连接中断，已填写内容已保留，请重新生成");
    return expected ? [...completed.values()] : result;
  } finally {
    await reader.cancel().catch(() => {});
    reader.releaseLock();
  }
}
