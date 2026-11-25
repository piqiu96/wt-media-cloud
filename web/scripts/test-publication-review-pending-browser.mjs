// Controlled production-UI regression: these intercepted contracts are not real E2E evidence.
import assert from "node:assert/strict";
import { chromium } from "playwright";
const browser = await chromium.launch({
  channel: process.env.WT_MEDIA_BROWSER_CHANNEL || "chrome",
  headless: true,
});
const base = process.env.WT_MEDIA_BROWSER_BASE_URL || "http://127.0.0.1:5173";
const time = "2026-11-25T04:00:00Z";
try {
  for (const [scene, count] of [
    ["normal_reviewing", 0],
    ["manual_reviewing", 0],
    ["stopped_verification", 1],
    ["expired_verification", 1],
    ["missing_statistics", undefined],
  ]) {
    const page = await browser.newPage({
        viewport: { width: 1440, height: 1000 },
      }),
      errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    const task = {
      id: "21",
      platform: "baidu",
      category_id: "1",
      media_account_id: "11",
      composite_output_id: "41",
      status: "reviewing",
      content: { description: scene },
      allowed_actions: count === 1 ? ["resolve"] : [],
      resources: { account_name: "本人账号", output_name: "本人素材" },
    };
    const batch = {
      id: "2",
      revision: "5",
      name: "待办口径批次",
      plan_id: "3",
      plan_name: "待办口径计划",
      platform: "baidu",
      operator_id: "9",
      configuration_status: "ready",
      execution_status: "ended",
      candidate_count: 1,
      allowed_actions: [],
      targets: [{ category_id: "1", target_count: 1 }],
      config: { accounts: [], output_ids: [], candidates: [] },
      readiness: { issues: [], shortfalls: [] },
      execution_setting: {
        concurrency_limit: 1,
        group_interval_seconds: 0,
        between_group_policy: "auto_continue",
        exception_task_limit: 1,
      },
      execution: {
        phase: "ended",
        first_started_at: time,
        ended_at: time,
        target_count: 1,
        published_count: 0,
        created_count: 1,
        creation_shortfall: 0,
        target_shortfall: 1,
        task_counts: { reviewing: 1 },
        ...(count === undefined ? {} : { verification_required_count: count }),
      },
    };
    const plan = {
      id: "3",
      revision: "4",
      name: "待办口径计划",
      owner_id: "9",
      date_start: "2026-11-25",
      date_end: "2026-11-26",
      goals: [{ platform: "baidu", category_id: "1", target_count: 1 }],
      goal_results: [
        {
          platform: "baidu",
          category_id: "1",
          target_count: 1,
          published_count: 0,
          shortfall: 1,
        },
      ],
      published_count: 0,
      batch_count: 1,
      processed_batch_count: 1,
      status: "running",
      progress: count === 1 ? "waiting_verification" : "awaiting_finish",
      current_batch_id: "2",
      started_at: time,
      allowed_actions: [],
    };
    await page.route("**/api/v1/**", async (route) => {
      const r = route.request(),
        p = new URL(r.url()).pathname;
      assert.equal(r.method(), "GET");
      const send = (data) =>
        route.fulfill({
          json: { errcode: 0, message: "success", data, logid: "pending" },
        });
      if (p === "/api/v1/auth/me")
        return send({
          id: "9",
          role: "operator",
          team_id: "7",
          status: "enabled",
          username: "operator",
          category_ids: ["1"],
        });
      if (p === "/api/v1/categories")
        return send([{ id: "1", name: "游戏", enabled: true }]);
      if (p === "/api/v1/publication-batch")
        return send({ items: [batch], total: 1 });
      if (p === "/api/v1/publication-batch/2") return send(batch);
      if (p === "/api/v1/publication-batch/2/task")
        return send({ items: [task], total: 1 });
      if (p === "/api/v1/publication-plan/3")
        return send({
          plan,
          batches: [
            {
              batch_id: "2",
              row_key: "stable",
              group_order: 1,
              batch_order: 1,
              configured: true,
              allowed_actions: [],
              batch,
            },
          ],
          issues: [],
        });
      return send({ items: [], total: 0 });
    });
    await page.goto(base + "/publish-batches");
    const batchRow = page
      .locator(".resource-data-table tbody tr")
      .filter({ hasText: "待办口径批次" });
    await batchRow.waitFor();
    assert.equal(
      (await batchRow.locator("td").nth(7).innerText()).trim(),
      count === undefined ? "待刷新" : count ? "1 项" : "—",
      scene + " batch list",
    );
    await page.goto(base + "/publish-plans?view=detail&id=3");
    const planRow = page
      .locator("tr")
      .filter({ has: page.locator("td") })
      .filter({ hasText: "待办口径批次" });
    await planRow.waitFor();
    assert.equal(
      (await planRow.locator("td").nth(7).innerText()).trim(),
      count === undefined ? "待办统计待刷新" : count ? "1 条待处理" : "—",
      scene + " plan",
    );
    assert.equal(
      await planRow
        .getByRole("button", { name: "去处理", exact: true })
        .count(),
      count ? 1 : 0,
    );
    if (["normal_reviewing", "manual_reviewing"].includes(scene)) {
      await page.getByText("等待平台公开结果", { exact: true }).waitFor();
      const published = page.locator(".resource-stat-card").filter({
        has: page.getByText("发布成功", { exact: true }),
      });
      assert.match(await published.locator("strong").textContent(), /^0\s*条$/);
    }
    assert.equal(
      await page.getByRole("button", { name: "结束计划", exact: true }).count(),
      0,
    );
    await page.goto(base + "/publish-batches?view=detail&id=2");
    const metric = page
      .locator(".resource-stat-card")
      .filter({ has: page.getByText("待处理", { exact: true }) });
    await metric.waitFor();
    assert.equal(
      (await metric.locator("strong").innerText()).trim(),
      count === undefined ? "待刷新" : String(count),
      scene + " batch detail",
    );
    const desc = await page.locator(".resource-descriptions").allInnerTexts();
    assert.ok(
      desc.some((text) => text.includes("待平台公开") && text.includes("1 条")),
      scene + " keeps total reviewing",
    );
    assert.equal(
      await page.getByRole("button", { name: "结束批次", exact: true }).count(),
      0,
    );
    assert.deepEqual(errors, []);
    await page.close();
  }
  console.log(
    "Review pending UI: ordinary/manual reviewing zero; stopped/expired verification one; missing counts unknown; all three views consistent PASS",
  );
} finally {
  await browser.close();
}
