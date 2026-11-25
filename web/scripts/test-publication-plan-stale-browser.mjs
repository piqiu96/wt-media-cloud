import assert from "node:assert/strict";
import { chromium } from "playwright";
const browser = await chromium.launch({
    channel: process.env.WT_MEDIA_BROWSER_CHANNEL || "chrome",
    headless: true,
  }),
  base = process.env.WT_MEDIA_BROWSER_BASE_URL || "http://127.0.0.1:5173";
for (const scenario of ["not_applied", "other_key"]) {
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1000 },
  });
  let posts = 0,
    first,
    rev = "1",
    started = false;
  const batch = {
    id: "2",
    operator_id: "9",
    plan_id: "1",
    name: "第一批",
    revision: "1",
    platform: "baidu",
    targets: [{ category_id: "1", target_count: 1 }],
    config: {
      candidates: [
        {
          row_key: "r",
          content: { confirmed: true, description: "原候选" },
          category_id: "1",
          media_account_id: "9",
          composite_output_id: "41",
        },
      ],
    },
    execution_setting: {
      concurrency_limit: 5,
      group_interval_seconds: 0,
      between_group_policy: "auto_continue",
      exception_task_limit: 3,
    },
    execution_status: "not_started",
    allowed_actions: [],
  };
  await page.route("**/api/v1/**", async (route) => {
    const req = route.request(),
      path = new URL(req.url()).pathname,
      send = (data) =>
        route.fulfill({
          json: { errcode: 0, message: "success", data, logid: "stale" },
        });
    if (path.endsWith("/auth/me"))
      return send({
        id: "9",
        role: "operator",
        status: "enabled",
        team_id: "7",
        category_ids: ["1"],
      });
    if (path.endsWith("/categories"))
      return send([{ id: "1", name: "三角洲行动" }]);
    if (path.endsWith("/record")) return send({ items: [], total: 0 });
    if (path === "/api/v1/publication-plan/1")
      return send({
        plan: {
          id: "1",
          owner_id: "9",
          revision: rev,
          name: "原请求核对",
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
          processed_batch_count: 0,
          batch_count: 1,
          status: started ? "running" : "not_started",
          progress: started ? "running" : "preparing",
          current_batch_id: started ? "2" : null,
          allowed_actions: started ? ["pause", "cancel"] : ["start", "cancel"],
        },
        batches: [
          { batch_id: "2", batch, configured: true, allowed_actions: [] },
        ],
        issues: [],
      });
    if (path === "/api/v1/publication-batch/2") return send(batch);
    if (path.endsWith("/execution-preview"))
      return send({
        batch_id: "2",
        revision: "1",
        can_start: true,
        rows: [{ row_key: "r", ready: true }],
        shortfalls: [{ category_id: "1", shortfall: 0 }],
      });
    if (path.endsWith("/start")) {
      const input = req.postDataJSON();
      posts++;
      if (posts === 1) {
        first = input;
        rev = "2";
        if (scenario === "other_key") {
          started = true;
          return route.abort("failed");
        }
        return route.fulfill({
          status: 409,
          json: {
            errcode: 18003,
            message: "配置已变化",
            data: null,
            error: {
              code: "publication_conflict",
              details: {
                command_applied: false,
                command: "start",
                request_key: input.request_key,
                expected_revision: input.expected_revision,
                current_revision: rev,
              },
            },
            logid: "stale",
          },
        });
      }
      if (scenario === "other_key") {
        assert.deepEqual(input, first);
        return route.fulfill({
          status: 409,
          json: {
            errcode: 18003,
            message: "其他请求已开始",
            data: null,
            error: {
              code: "publication_conflict",
              details: {
                command_applied: false,
                command: "start",
                request_key: "someone-else",
                expected_revision: input.expected_revision,
                current_revision: rev,
              },
            },
            logid: "other",
          },
        });
      }
      assert.notEqual(input.request_key, first.request_key);
      assert.equal(input.expected_revision, "2");
      started = true;
      rev = "3";
      return send({
        plan_id: "1",
        revision: rev,
        first_started_at: "2026-11-25T04:00:00Z",
        current_batch_id: "2",
        batch_confirmations: input.batch_confirmations,
        first_batch_receipt: {
          batch_id: "2",
          revision: "2",
          execution_status: "running",
          first_started_at: "2026-11-25T04:00:00Z",
          task_ids: ["11"],
          created_count: 1,
          target_count: 1,
          creation_shortfall: 0,
        },
      });
    }
    return send({ items: [], total: 0 });
  });
  await page.goto(base + "/publish-plans?view=detail&id=1");
  await page.getByRole("button", { name: "执行", exact: true }).click();
  await page.getByText("确认全部批次范围及目标缺口").click();
  await page.getByRole("button", { name: "确认执行", exact: true }).click();
  if (scenario === "not_applied") {
    await page.getByRole("dialog").waitFor({ state: "hidden" });
    assert.equal(
      await page
        .getByRole("button", { name: "核对原计划请求", exact: true })
        .count(),
      0,
    );
    await page.getByRole("button", { name: "执行", exact: true }).click();
    await page.getByText("确认全部批次范围及目标缺口").click();
    await page.getByRole("button", { name: "确认执行", exact: true }).click();
    await page.getByRole("button", { name: "暂停计划", exact: true }).waitFor();
  } else {
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "核对原计划请求", exact: true })
      .click();
    await page.getByText("操作结果待确认", { exact: false }).first().waitFor();
    assert.equal(
      await page
        .getByRole("dialog")
        .getByRole("button", { name: "核对原计划请求", exact: true })
        .count(),
      1,
    );
    const pending = await page.evaluate(() =>
      Object.entries(localStorage).find(([key]) =>
        key.startsWith("wt-media:plan-command:v1"),
      ),
    );
    assert.equal(JSON.parse(pending[1]).input.request_key, first.request_key);
  }
  assert.equal(posts, 2);
  await page.close();
}
await browser.close();
console.log(
  "Plan stale command proof: exact-bound false restores preview; other-key proof preserves unknown; immutable receipt branch remains separate PASS",
);
