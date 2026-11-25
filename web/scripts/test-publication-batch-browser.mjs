// Exercises the production Cloud UI with isolated synthetic responses.
import assert from "node:assert/strict";
import { chromium } from "playwright";

const base = process.env.WT_MEDIA_BROWSER_BASE_URL || "http://127.0.0.1:5173";
const browser = await chromium.launch({ channel: process.env.WT_MEDIA_BROWSER_CHANNEL || "chrome", headless: true });
const page = await browser.newPage({ viewport: { width: Number(process.env.WT_MEDIA_VIEWPORT_WIDTH) || 1440, height: 1000 } });
const exceptions = [];
page.on("pageerror", error => exceptions.push(error.message));
const recordedCalls = [];
let saved;
let savedKey;
const actor = { id: "71", role: "operator", team_id: "3", category_ids: ["12","13"], status: "enabled", username: "test" };
const category = { id: "12", name: "合成测试类目", enabled: true };
const secondCategory={id:"13",name:"第二测试类目",enabled:true};
const account = { id: "9007199254740993", name: "由我发布", platform: "baidu", category_ids: ["12"], eligible: true };
const alternateAccount={...account,id:"9007199254741001",name:"由我发布备用"};
const outputs = [
  { composite_output_id: "9007199254740995", id: 1, name: "合成测试成片 A" },
  { composite_output_id: "9007199254740997", id: 2, name: "合成测试成片 B" },
].map(item => ({ ...item, category_id: "12", team_id: "3", occupied: false, platforms: ["baidu"], video: { status: "ready" }, media: {duration_ms:30000} }));
const spareOutput={...outputs[1],id:3,composite_output_id:"9007199254741011",name:"替换后的测试成片 C"};
const now = "2026-11-25T10:00:00Z";
const template = { id: "903", name: "合成横版模板", canvas_width: 1280, canvas_height: 720,
  enabled: true, published_revision: 2, published_config: { backgroundMode: "solid" } };
const libraryCover = { id: "904", file_id: "905", name: "合成现成封面", metadata: { width: 1280, height: 720 },
  resource_path: "/api/v1/file-resources/" + "c".repeat(64) };

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
    case "/api/v1/categories": return send([category,secondCategory]);
    case "/api/v1/publication/account": return send({ items: [account,alternateAccount], total: 2 });
    case "/api/v1/publication/resource": await new Promise(resolve=>setTimeout(resolve,300)); return send({ availability: "available", available_output_count: 2,
      eligible_account_count: 2, account_count: 2, unknown_output_count: 0, as_of: now });
    case "/api/v1/publication/output": {
      const selected = params.get("output_ids")?.split(",");
      const items = selected ? [...outputs,spareOutput].filter(row => selected.includes(row.composite_output_id)) : [...outputs,spareOutput];
      return send({ items, total: items.length });
    }
    case "/api/v1/publication-batch/allocate":
      assert.equal(input.accounts[0].media_account_id, account.id);
      assert.deepEqual(input.output_ids, outputs.map(item => item.composite_output_id));
      return send({ config: { candidates: outputs.map((item, index) => ({
        category_id: "12", media_account_id: account.id, composite_output_id: item.composite_output_id,
        row_key: `candidate-${index}`, input_revision: "1", field_revisions: {}, order: index, content: {},
      })) }, shortfalls: [] });
    case "/api/v1/cover-template": return send({ items: [template], total: 1 });
    case "/api/v1/cover-template/903": return send(template);
    case "/api/v1/image-asset": return send({ items: [libraryCover], total: 1 });
    case "/api/v1/cover/render": {
      const { row_key, input_revision, template_id, expected_published_revision, title } = input;
      const artwork = {
        row_key, input_revision, preview: false, render_input_hash: "b".repeat(64),
        snapshot: { template_id, published_revision: expected_published_revision, title },
        file: { id: "906", metadata: { width: 1280, height: 720 },
          resource_path: "/api/v1/file-resources/" + "d".repeat(64) },
      };
      const body = `event: item_result\ndata: ${JSON.stringify({ result: { artwork } })}\n\nevent: done\ndata: {"total":1,"success":1,"failed":0}\n\n`;
      return route.fulfill({ status: 200, contentType: "text/event-stream", body });
    }
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
const next = async () => {
  if (await button("确认资源").count()) {
    await button("确认资源").click();
    await page.getByRole("dialog",{name:"确认资源",exact:true}).waitFor();
    await button("进入内容配置").click();
  } else await button("预览检查").click();
};
try {
  await page.goto(new URL("/publish-batches", base).href);
  await heading("发布批次").waitFor();
  await button("新建批次").click();
  await input("批次名称").fill("浏览器批次");
  await page.locator(".batch-category-chip").filter({hasText:category.name}).click();
  assert.equal(await page.getByRole("checkbox", {name:category.name,exact:true}).isChecked(),true);
  await page.locator('.batch-category-chip').filter({hasText:secondCategory.name}).click();
  await input(`${secondCategory.name}发布目标`).waitFor();
  await page.locator('.batch-category-chip').filter({hasText:secondCategory.name}).click();
  await input(`${secondCategory.name}发布目标`).waitFor({state:'hidden'});
  await input(`${category.name}发布目标`).fill("2");
  await input('目标数量').fill('2');
  const quantityControl = page.locator('.batch-goal-field .t-input-number');
  const quantityGeometry = await quantityControl.evaluate(control => {
    const box = control.getBoundingClientRect();
    const children = [...control.children].map(child => child.getBoundingClientRect());
    return { width: box.width, height: box.height, border: getComputedStyle(control).borderLeftWidth,
      gaps: children.slice(1).map((child,index) => child.left - children[index].right) };
  });
  assert.equal(quantityGeometry.height,36,'numeric input must retain the prototype single joined 36px control');
  assert.equal(quantityGeometry.border,'1px','one outer border must enclose minus, input and plus');
  assert.ok(quantityGeometry.gaps.every(gap => Math.abs(gap) < 1),'stepper segments must have no gaps');
  await input('目标数量').press('ArrowUp');
  assert.equal(await input('目标数量').inputValue(),'3');
  await input('目标数量').press('ArrowDown');
  assert.equal(await input('目标数量').inputValue(),'2');
  await button('采用系统推荐').click();
  await page.getByText('已采用系统推荐，可继续调整类目数量和匹配。',{exact:true}).waitFor();
  assert.equal(await input(`${category.name}发布目标`).inputValue(),'2');
  assert.equal(await page.locator('.batch-section').count(),3,'resource preparation retains the prototype three-section frame');

  await button("账号与本批上限").click();
  await button("载入可用资源").click();
  assert.ok(
    Math.abs(
      (
        await page
          .locator(".batch-resource-row .t-input-number")
          .last()
          .boundingBox()
      ).width - 240,
    ) < 2,
  );
  await input("本批上限").fill("2");
  await button("完成").click();
  await button("重新自动匹配").click();
  await page.getByText("已配对 2 条候选", { exact: true }).waitFor();
  const rowAccount=()=>page.locator('[aria-label="第1条发布账号"]').first();
  await rowAccount().click();
  await page.locator('.t-select-option:visible').getByText(alternateAccount.name,{exact:true}).click();
  await page.locator('.t-select-option:visible').first().waitFor({state:'hidden'});
  assert.equal(await input('第1条发布账号').inputValue(),alternateAccount.name,'inline account selection must update the original matching row');
  await rowAccount().click();
  await page.locator('.t-select-option:visible').getByText(account.name,{exact:true}).click();
  await page.waitForFunction(name=>[...document.querySelectorAll('[aria-label="第1条发布账号"] input, input[aria-label="第1条发布账号"]')].some(input=>input.value===name),account.name);
  assert.equal(await input('第1条发布账号').inputValue(),account.name);

  await next();

  const contentTable = page.getByRole("region", { name: "内容配置" });
  await contentTable.getByText("成片 ID：9007199254740995 · 时长：30.0 秒",{exact:true}).waitFor();
  for (const title of ["发布内容", "发布账号", "发布成片", "配置状态"])
    await contentTable
      .getByRole("columnheader", { name: title, exact: true })
      .waitFor();
  await page.locator('[aria-label="筛选候选类目"]').click();
  await page
    .locator(".t-select-option:visible")
    .getByText(category.name, { exact: true })
    .click();
  await page.locator('[aria-label="筛选候选状态"]').click();
  await page
    .locator(".t-select-option:visible")
    .getByText("待完善", { exact: true })
    .click();
  await button("查询").click();
  assert.equal(await contentTable.locator("tbody tr").count(), 2);
  await button("重置").click();
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth,
    ),
    false,
  );
  assert.ok((await contentTable.locator(".batch-content-issues").count()) > 0);
  await contentTable.locator(".batch-content-issues summary").first().click();
  await page
    .locator(
      '[aria-label="搜索候选内容"] input,input[aria-label="搜索候选内容"]',
    )
    .first()
    .fill(outputs[0].name);
  await button("查询").click();
  assert.equal(await contentTable.locator("tbody tr").count(), 1);
  await contentTable.locator("tbody tr .t-checkbox").first().click();
  await button("重置").click();
  assert.equal(await contentTable.locator("tbody tr").count(), 2);
  assert.equal(
    (
      await page
        .getByRole("region", { name: "批量操作" })
        .locator("strong")
        .innerText()
    ).trim(),
    "1",
  );
  await page
    .locator(
      '[aria-label="搜索候选内容"] input,input[aria-label="搜索候选内容"]',
    )
    .first()
    .fill(outputs[1].name);
  await button("查询").click();
  await contentTable.locator("tbody tr .t-checkbox").first().click();
  assert.equal(
    (
      await page
        .getByRole("region", { name: "批量操作" })
        .locator("strong")
        .innerText()
    ).trim(),
    "2",
  );
  assert.equal(await contentTable.locator('.batch-content-more').count(),0,'confirmed resource mapping has no content-stage mutation actions');
  await button("重置").click();
  await page
    .getByRole("region", { name: "批量操作" })
    .getByRole("button", { name: "取消选择", exact: true })
    .click();

  const firstRow = tableRows().filter({ hasText: outputs[0].name });
  await firstRow.locator(".t-checkbox").click();
  await button("批量配置").click();
  await page.getByText("AI 文案", { exact: true }).click();
  await button("预览配置").click();
  await page.getByText("生成结果与请求的行或版本不一致", { exact: true }).waitFor();
  assert.equal(await page.getByText("迟到的文案", { exact: true }).count(), 0);
  await page.keyboard.press("Escape");
  await firstRow.getByRole("button", { name: "编辑" }).click();
  await page.locator('[aria-label="发布文案"] textarea, textarea[aria-label="发布文案"]').fill("手动保留文案");
  const horizontal = page.locator(".batch-cover-row").filter({ hasText: "横版封面 · 16:9" });
  await input("封面短标题").fill("原生成标题");
  await horizontal.getByText("模板合成", { exact: true }).click();
  await horizontal.getByPlaceholder("搜索并选择封面模板").click();
  await page.locator(".t-select-option").getByText(/合成横版模板/).click();
  await horizontal.getByRole("button", { name: "生成封面", exact: true }).click();
  const adopt = horizontal.getByRole("button", { name: "采用生成封面", exact: true });
  await adopt.waitFor();
  await input("封面短标题").fill("人工新标题");
  await input("封面短标题").press("Tab");
  assert.equal(await adopt.count(), 0, "修改短标题后不应继续提供旧封面的采用入口");
  await horizontal.getByRole("button", { name: "生成封面", exact: true }).click();
  await adopt.click();
  await horizontal.getByAltText("已采用横版封面 · 16:9").waitFor();

  await horizontal.getByText("使用现成封面", { exact: true }).click();
  await horizontal.getByRole("button", { name: "选择图片", exact: true }).click();
  const imagePicker = page.getByRole("dialog", { name: "选择发布封面", exact: true });
  const assetRow = imagePicker.getByRole("row").filter({ hasText: libraryCover.name });
  await assetRow.waitFor({ timeout: 3000 });
  await assetRow.getByRole("button", { name: "选择", exact: true }).click();
  await button("完成").click();
  await next();
  await input("异常任务上限").fill("5");
  await button("返回内容配置").click();
  await firstRow.locator(".t-checkbox").click();
  await button("确认内容").click();
  await button("确认已核对内容").click();
  await page.getByText("已确认 0 条，1 条待完善。请查看对应候选。", { exact: true }).waitFor();
  await next();
  await page
    .getByRole("region", { name: "准备结果" })
    .getByLabel("复查批次名称")
    .last()
    .waitFor();
  await input("复查批次名称").fill("复查一致性名称");
  await button("返回内容配置").click();
  assert.equal(await button("返回内容配置").count(),0,'content stage cannot return to confirmed resources');
  assert.equal(await button("确认资源").count(),0);
  await next();
  assert.equal(await input("复查批次名称").inputValue(),"复查一致性名称");
  await input("复查批次名称").fill("浏览器批次");
  const settings = page.locator(".batch-settings-grid .t-input-number").first();
  const settingsBox = await settings.boundingBox();
  assert.ok(Math.abs(settingsBox.width - 240) < 2);
  const reviewContents = page.getByRole("region", { name: "发布内容" });
  assert.equal(await reviewContents.locator(".t-checkbox").count(), 0);
  assert.equal(
    await reviewContents.locator(".batch-content-filters").count(),
    0,
  );
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth,
    ),
    false,
  );
  assert.equal(
    await reviewContents
      .getByRole("button", { name: "调整配对", exact: true })
      .count(),
    0,
  );
  assert.equal(await reviewContents.locator(".batch-content-more").count(), 0);
  await reviewContents.getByText("手动保留文案", { exact: true }).waitFor();
  await reviewContents
    .getByRole("button", { name: "编辑", exact: true })
    .first()
    .click();
  const reviewingDescription = page.locator(
    '[aria-label="发布文案"] textarea,textarea[aria-label="发布文案"]',
  );
  await reviewingDescription.fill("复查编辑后的文案");
  await button("完成").click();
  await reviewContents.getByText("复查编辑后的文案", { exact: true }).waitFor();
  await reviewContents
    .getByRole("button", { name: "编辑", exact: true })
    .first()
    .click();
  await reviewingDescription.fill("手动保留文案");
  await button("完成").click();
  await button("保存批次").click();
  await heading("浏览器批次").waitFor();
  await page.getByText("批次尚未开始，正式发布任务 0 条。", { exact: true }).waitFor();
  assert.equal(saved.config.candidates[0].content.description, "手动保留文案");
  assert.equal(saved.config.candidates[0].cover_title, "人工新标题");
  assert.equal(saved.config.candidates[0].content.cover_file_id, libraryCover.file_id);
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
  await button("继续配置").click();
  await heading("编辑批次 · 浏览器批次").waitFor();
  await tableRows().filter({hasText:outputs[1].name}).getByRole("button",{name:"更换成片",exact:true}).click();
  await page.getByRole('dialog',{name:'选择成片',exact:true}).getByRole('row').filter({hasText:spareOutput.name}).getByRole('button',{name:'选择',exact:true}).click();
  await tableRows().filter({hasText:spareOutput.name}).getByRole('button',{name:'移出',exact:true}).click();
  await page.locator(".resource-feedback-dialog").getByRole("button", { name: "移除" }).click();
  await next();
  await next();
  await page.getByText("目标 2 · 候选 1 · 缺口 1", { exact: true }).waitFor();
  await button("保存修改").click();
  await heading("浏览器批次").waitFor();
  assert.equal(saved.config.candidates.length, 1);
  assert.equal(saved.config.manual_pairs.some(pair=>pair.composite_output_id===spareOutput.composite_output_id),false,"removing a replaced row must clear its current manual pair");
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
