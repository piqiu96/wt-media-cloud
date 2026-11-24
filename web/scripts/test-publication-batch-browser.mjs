// Exercises the production Cloud UI with isolated synthetic responses.
import assert from "node:assert/strict";
import { chromium } from "playwright";

const base = process.env.WT_MEDIA_BROWSER_BASE_URL || "http://127.0.0.1:5173";
const browser = await chromium.launch({ channel: process.env.WT_MEDIA_BROWSER_CHANNEL || "chrome", headless: true });
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
const exceptions = [];
page.on("pageerror", error => exceptions.push(error.message));
const recordedCalls = [];
let saved;
let savedKey;
const actor = { id: "71", role: "operator", team_id: "3", category_ids: ["12"], status: "enabled", username: "test" };
const category = { id: "12", name: "合成测试类目", enabled: true };
const account = { id: "9007199254740993", name: "由我发布", platform: "baidu", category_ids: ["12"], eligible: true };
const outputs = [
  { composite_output_id: "9007199254740995", id: 1, name: "合成测试成片 A" },
  { composite_output_id: "9007199254740997", id: 2, name: "合成测试成片 B" },
].map(item => ({ ...item, category_id: "12", team_id: "3", occupied: false, platforms: ["baidu"], video: { status: "ready" } }));
const now = "2026-11-25T10:00:00Z";

function readiness(input) {
  const included = input.config.candidates;
  return {
    ready: false, issues: included.map(row => ({ code: "content_required", row_key: row.row_key, message: "请补充封面" })),
    shortfalls: input.targets.map(item => ({ ...item, candidate_count: included.length, shortfall: item.target_count - included.length })),
  };
}
function batch(input, revision) {
  const { name, platform, targets, config, execution_setting } = input;
  return { id: "9007199254740999", revision, name, platform, targets, config, execution_setting,
    candidate_count: config.candidates.length, configuration_status: "incomplete", execution_status: "not_started",
    allowed_actions: ["edit"], operator_id: "71", team_id: "3", updated_at: now, readiness: readiness(input) };
}
function packet(data) {return { errcode: 0, message: "success", data, logid: "test" };}

await page.route("**/api/v1/**", async route => {
  const request = route.request();
  const url = new URL(request.url());
  const params = url.searchParams;
  const resource = url.pathname;
  const method = request.method();
  const input = request.postDataJSON();
  recordedCalls.push({ resource, method, params, input });
  function send(data) { return route.fulfill({ json: packet(data) }); }
  switch (resource) {
    case "/api/v1/auth/me": return send(actor);
    case "/api/v1/categories": return send([category]);
    case "/api/v1/publication/account": return send({ items: [account], total: 1 });
    case "/api/v1/publication/resource": return send({ availability: "available", available_output_count: 2,
      eligible_account_count: 1, account_count: 1, unknown_output_count: 0, as_of: now });
    case "/api/v1/publication/output": {
      const selected = params.get("output_ids")?.split(",");
      const items = selected ? outputs.filter(row => selected.includes(row.composite_output_id)) : outputs;
      return send({ items, total: items.length });
    }
    case "/api/v1/publication-batch/allocate":
      assert.equal(input.accounts[0].media_account_id, account.id);
      assert.deepEqual(input.output_ids, outputs.map(item => item.composite_output_id));
      return send({ config: { candidates: outputs.map((item, index) => ({
        category_id: "12", media_account_id: account.id, composite_output_id: item.composite_output_id,
        row_key: `candidate-${index}`, input_revision: "1", field_revisions: {}, order: index, content: {},
      })) }, shortfalls: [] });
    case "/api/v1/publication-batch/validate": return send(readiness(input));
    case "/api/v1/publication/generate": {
      const fields = input.items[0].field_revisions;
      const body = `event: item_result\ndata: ${JSON.stringify({
        row_key: input.items[0].row_key, input_revision: "300", field_revisions: fields,
        result: { content: { description: "迟到的文案" }, field_revisions: fields },
      })}\n\nevent: done\ndata: {"total":1,"success":1,"failed":0}\n\n`;
      return route.fulfill({ status: 200, contentType: "text/event-stream", body });
    }
    case "/api/v1/publication-batch":
      if (method === "POST") {
        savedKey = input.request_key;
        saved = batch(input, "1");
        return route.abort("failed");
      }
      if (params.has("request_key")) assert.equal(params.get("request_key"), savedKey);
      return send({ items: saved ? [saved] : [], total: saved ? 1 : 0, page: 1, page_size: 10 });
    case "/api/v1/publication-batch/9007199254740999": return send(saved);
    case "/api/v1/publication-batch/9007199254740999/configuration":
      assert.equal(input.expected_revision, "1");
      saved = batch(input, "2");
      return send(saved);
    case "/api/v1/publication-batch/9007199254740999/record":
      return send({ items: [{ id: "audit-1", actor_id: "71", event: "publication_batch_created", created_at: now }], total: 1 });
    default: return send([]);
  }
});

const button = (name) => page.getByRole("button", { name, exact: true });
const heading = name => page.getByRole("heading", { name, exact: true });
const input = label => page.locator(`[aria-label="${label}"] input, input[aria-label="${label}"]`).first();
const tableRows = () => page.locator("tbody tr");
const next = () => button("下一步").click();
try {
  await page.goto(new URL("/publish-batches", base).href);
  await heading("发布批次").waitFor();
  await button("新建批次").click();
  await input("批次名称").fill("浏览器批次");
  await page.locator('[aria-label="目标类目"]').click();
  await page.locator(".t-select-option").getByText(category.name, { exact: true }).click();
  await button("添加类目").click();
  await input(`${category.name}发布目标`).fill("2");
  await button("载入可用资源").click();
  await input("本批上限").fill("2");
  await button("按顺序自动配对").click();
  await page.getByText("已配对 2 条候选", { exact: true }).waitFor();
  await next();

  const firstRow = tableRows().filter({ hasText: outputs[0].name });
  await firstRow.locator(".t-checkbox").click();
  await button("批量配置").click();
  await page.getByText("AI 文案", { exact: true }).click();
  await button("预览配置").click();
  await page.getByText("生成结果与请求的行或版本不一致", { exact: true }).waitFor();
  assert.equal(await page.getByText("迟到的文案", { exact: true }).count(), 0);
  await page.keyboard.press("Escape");
  await firstRow.getByRole("button", { name: "编辑内容" }).click();
  await page.locator('[aria-label="发布文案"] textarea, textarea[aria-label="发布文案"]').fill("手动保留文案");
  await button("完成").click();
  await next();
  await input("异常任务上限").fill("5");
  await button("上一步").click();
  await firstRow.locator(".t-checkbox").click();
  await button("确认内容").click();
  await button("确认已核对内容").click();
  await page.getByText("已确认 0 条，1 条待完善。请查看对应候选。", { exact: true }).waitFor();
  await next();
  await button("保存批次").click();
  await heading("浏览器批次").waitFor();
  await page.getByText("批次尚未开始，正式发布任务 0 条。", { exact: true }).waitFor();
  assert.equal(saved.config.candidates[0].content.description, "手动保留文案");
  assert.equal(saved.targets[0].target_count, 2);
  const originalRowKey = saved.config.candidates[0].row_key;
  assert.equal(recordedCalls.filter(call => call.method === "POST" && call.resource === "/api/v1/publication-batch").length, 1);
  assert.equal(recordedCalls.some(call => call.resource === "/api/v1/publication-batch" && call.params.get("request_key") === savedKey), true);

  await page.getByText("操作记录", { exact: true }).click();
  await tableRows().getByText("保存批次", { exact: true }).waitFor();
  await button("返回").click();
  await heading("发布批次").waitFor();
  await input("搜索批次").fill("浏览器");
  await button("查询").click();
  await button("编辑配置").click();
  await heading("编辑批次 · 浏览器批次").waitFor();
  await next();
  await tableRows().filter({ hasText: outputs[1].name }).getByRole("button", { name: "移除" }).click();
  await page.locator(".resource-feedback-dialog").getByRole("button", { name: "移除" }).click();
  await next();
  await page.getByText("目标 2 · 候选 1 · 缺口 1", { exact: true }).waitFor();
  await button("保存修改").click();
  await heading("浏览器批次").waitFor();
  assert.equal(saved.config.candidates.length, 1);
  assert.equal(saved.config.candidates[0].row_key, originalRowKey);
  assert.equal(saved.targets[0].target_count, 2);
  assert.equal(saved.revision, "2");
  await button("返回").click();
  await heading("发布批次").waitFor();
  assert.equal(new URL(page.url()).searchParams.get("search"), "浏览器");
  assert.deepEqual(exceptions, []);
  assert.equal(recordedCalls.some(call => call.method === "POST" && call.resource === "/api/v1/publication"), false);
  console.log("Publication batch browser checks passed");
} finally {
  await browser.close();
}
