let revision = null;
async function request(route, method = "GET", body) {
  const response = await fetch("/__example/publishing/" + route, {
    method,
    headers: { "Content-Type": "application/json" },
    ...(body ? { body: JSON.stringify(body) } : {}),
  });
  if (!response.headers.get("content-type")?.includes("application/json"))
    throw Error("当前预览服务不支持批次校验，请使用本地 example 服务");
  const result = await response.json();
  if (!response.ok) throw Error(result.error || "操作失败，请重试");
  if (result) revision = result.revision;
  return result;
}
export async function loadBatchSnapshot(legacy) {
  let result = await request("snapshot");
  if (!result && legacy)
    result = await request("bootstrap", "POST", { data: legacy });
  return result?.data || null;
}
export async function saveBatchSnapshot(data) {
  if (revision === null) {
    await request("bootstrap", "POST", { data });
    return;
  }
  await request("snapshot", "PUT", { data, revision });
}
export async function batchCommand(
  batchId,
  action,
  input = {},
  key = crypto.randomUUID(),
) {
  const result = await request("command", "POST", {
    batchId,
    action,
    input,
    key,
    revision,
  });
  return result.data;
}
