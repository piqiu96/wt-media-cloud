import assert from "node:assert/strict";
import {writeFile} from "node:fs/promises";
import { chromium } from "playwright";
const base = process.env.WT_MEDIA_BROWSER_BASE_URL || "http://127.0.0.1:5173";
const browser = await chromium.launch({
  channel: process.env.WT_MEDIA_BROWSER_CHANNEL || "chrome",
  headless: true,
});
const cases = [
  ["waiting_continue", ["confirm-next", "pause", "cancel"], "confirm-next"],
  ["waiting_skip", ["confirm-next", "pause", "cancel"], "confirm-next"],
  ["manual_paused", ["resume", "cancel"], "resume"],
  ["waiting_verification", ["pause", "cancel"], null],
  ["waiting_resources", ["pause", "cancel"], null],
  ["running", ["takeover", "pause", "cancel"], "takeover"],
  ["awaiting_finish", ["finish", "cancel"], "finish"],
];
for (const [progress, actions, action] of cases) {
  const page = await browser.newPage({
    viewport: { width: 1024, height: 768 },
  });
  let calls = 0;
  const p = {
    id: "3",
    owner_id: "9",
    name: "衔接验证",
    date_start: "2026-11-25",
    date_end: "2026-11-26",
    goals: [{ platform: "baidu", category_id: "1", target_count: 10 }],
    goal_results: [
      {
        platform: "baidu",
        category_id: "1",
        target_count: 10,
        published_count: 8,
        shortfall: 2,
      },
    ],
    published_count: 8,
    pending_count: 0,
    batch_count: 2,
    processed_batch_count: 1,
    revision: "5",
    status: progress === "manual_paused" ? "paused" : "running",
    progress,
    current_batch_id: "1",
    started_at: "2026-11-25T04:00:00Z",
    allowed_actions: actions,
    handoff_basis: {
      current_batch_id: "1",
      next_batch_id: "2",
      fingerprint: "a".repeat(64),
      explicit_count: 10,
      failed_count: 2,
    },
  };
  await page.route("**/api/v1/**", async (route) => {
    const r = route.request(),
      url = new URL(r.url());
    const send = (data) =>
      route.fulfill({
        json: { errcode: 0, message: "success", data, logid: "controls" },
      });
    if (url.pathname === "/api/v1/auth/me")
      return send({
        id: "9",
        role: "operator",
        team_id: "7",
        status: "enabled",
        category_ids: ["1"],
      });
    if (url.pathname === "/api/v1/categories")
      return send([{ id: "1", name: "三角洲行动" }]);
    if (url.pathname.endsWith("/record")) return send({ items: [], total: 0 });
    if (url.pathname === "/api/v1/publication-plan/3")
      return send({
        plan: p,
        batches: [
          {
            batch_id: "1",
            batch: {
              id: "1",
              revision: "7",
              name: "当前批次",
              platform: "baidu",
              targets: [{ category_id: "1", target_count: 10 }],
              execution_status: "running",
              allowed_actions: [],
            },
            allowed_actions: [],
            configured: true,
          },
        ],
        issues: [],
      });
    if (r.method() === "POST") {
      calls++;
      assert.ok(url.pathname.endsWith("/" + action));
      const input = r.postDataJSON();
      assert.equal(input.expected_revision, "5");
      if (action === "confirm-next") {
        assert.equal(input.expected_basis_fingerprint, "a".repeat(64));
        assert.equal(input.current_batch_id, "1");
        assert.equal(input.next_batch_id, "2");
        assert.equal(
          input.decision,
          progress === "waiting_skip" ? "skip" : "continue",
        );
        p.progress = "running";
        p.allowed_actions = ["pause", "cancel"];
        p.revision = "6";
        return send({
          plan_id: "3",
          revision: "6",
          previous_batch_id: "1",
          current_batch_id: "2",
          decision: input.decision,
          advanced_at: "2026-11-25T05:00:00Z",
          current_batch_status: "cancelled",
          batch_receipt: null,
        });
      }
      if (action === "takeover") {
        assert.equal(input.current_batch_id, "1");
        assert.equal(input.expected_batch_revision, "7");
      }
      if (action === "finish") {
        assert.equal(input.end_reason, "unmet");
        assert.deepEqual(Object.keys(input).sort(),["end_note","end_reason","expected_revision","expected_shortfalls"]);
        await writeFile("/private/tmp/chg109-finish-client-request.json",JSON.stringify(input));
        assert.deepEqual(input.expected_shortfalls, [
          { platform: "baidu", category_id: "1", shortfall: 2 },
        ]);
        assert.ok(input.end_note);
      }
      p.progress = "running";
      p.allowed_actions = ["pause", "cancel"];
      p.revision = "6";
      return send({ plan_id: "3", revision: "6" });
    }
    return send({ items: [], total: 0 });
  });
  await page.goto(base + "/publish-plans?view=detail&id=3");
  await page.getByRole("heading", { name: "发布计划详情" }).waitFor();
  if (!action) {
    assert.equal(
      await page
        .getByRole("button", { name: "确认进入下一批", exact: true })
        .count(),
      0,
    );
    assert.equal(
      await page.getByRole("button", { name: "结束计划", exact: true }).count(),
      0,
    );
  } else {
    const label =
      action === "confirm-next"
        ? progress === "waiting_skip"
          ? "确认跳过当前批次"
          : "确认进入下一批"
        : action === "resume"
          ? "恢复计划"
          : action === "takeover"
            ? "接管当前批次"
            : "结束计划";
    await page.getByRole("button", { name: label, exact: true }).click();
    const dialog = page.getByRole("dialog");
    if (action !== "resume")
      await dialog.locator("textarea").fill("已核对原批次依据，按当前范围处理");
    await dialog
      .getByRole("button", {
        name: action === "confirm-next" ? label : "确认" + label,
        exact: true,
      })
      .click();
    await dialog.waitFor({ state: "hidden" });
    assert.equal(calls, 1);
  }
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth,
    ),
    false,
  );
  await page.close();
}
await browser.close();
console.log(
  "CHG109 plan controls: exact basis, skip, pause/resume, takeover, final gaps and unresolved blockers PASS",
);
