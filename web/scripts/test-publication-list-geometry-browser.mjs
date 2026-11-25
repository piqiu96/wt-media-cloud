// Production opt-in list geometry, checked through browser layout (not CSS source snapshots).
import assert from "node:assert/strict";
import { chromium } from "playwright";
const browser = await chromium.launch({
  channel: process.env.WT_MEDIA_BROWSER_CHANNEL || "chrome",
  headless: true,
});
const base = process.env.WT_MEDIA_BROWSER_BASE_URL || "http://127.0.0.1:5173";
try {
  for (const width of [1440, 1024]) {
    for (const kind of ["plans", "batches"]) {
      const page = await browser.newPage({ viewport: { width, height: 1000 } }),
        errors = [];
      page.on("pageerror", (e) => errors.push(e.message));
      const batch = {
        id: "2",
        revision: "1",
        name: "几何批次",
        plan_id: "3",
        plan_name: "几何计划",
        operator_id: "9",
        platform: "baidu",
        targets: [{ category_id: "1", target_count: 1 }],
        candidate_count: 1,
        configuration_status: "ready",
        execution_status: "not_started",
        allowed_actions: ["edit", "stop"],
        execution: { verification_required_count: 0, task_counts: {} },
      };
      const plan = {
        id: "3",
        revision: "1",
        name: "几何计划",
        status: "not_started",
        progress: "preparing",
        owner_id: "9",
        goals: [{ platform: "baidu", category_id: "1", target_count: 1 }],
        published_count: 0,
        batch_count: 1,
        processed_batch_count: 0,
        date_start: "2026-11-25",
        date_end: "2026-11-26",
        updated_at: "2026-11-25T04:00:00Z",
        allowed_actions: ["edit", "cancel"],
      };
      await page.route("**/api/v1/**", async (route) => {
        const r = route.request(),
          p = new URL(r.url()).pathname;
        assert.equal(r.method(), "GET");
        const send = (data) =>
          route.fulfill({
            json: { errcode: 0, message: "success", data, logid: "geometry" },
          });
        if (p === "/api/v1/auth/me")
          return send({
            id: "9",
            username: "operator",
            role: "operator",
            team_id: "7",
            status: "enabled",
            category_ids: ["1"],
          });
        if (p === "/api/v1/categories")
          return send([{ id: "1", name: "游戏", enabled: true }]);
        if (p === "/api/v1/publication-plan")
          return send({ items: [plan], total: 1 });
        if (p === "/api/v1/publication-batch")
          return send({ items: [batch], total: 1 });
        return send({ items: [], total: 0 });
      });
      await page.goto(base + "/publish-" + kind);
      const name = kind === "plans" ? "几何计划" : "几何批次";
      const row = page
        .locator(".resource-data-table .t-table__body tr")
        .filter({ hasText: name });
      await row.waitFor();
      const card = page.locator(".resource-card").first(),
        filter = card.locator(".resource-filter-bar"),
        fields = filter.locator(".wt-filter-field");
      const [search, ordinary] = await Promise.all([
        fields.nth(0).boundingBox(),
        fields.nth(1).boundingBox(),
      ]);
      assert.ok(
        search.width > ordinary.width + 20,
        `${kind}/${width}: search must be wider than ordinary filters (${search.width}/${ordinary.width})`,
      );
      assert.equal(
        await filter.evaluate((e) => getComputedStyle(e).borderBottomWidth),
        "0px",
        `${kind}/${width}: filter must not add an extra divider`,
      );
      const table = card.locator(".t-table__content").first();
      const [outer, tableBox] = await Promise.all([
        card.boundingBox(),
        table.boundingBox(),
      ]);
      const heading = await page.locator(".resource-page-header").boundingBox();
      assert.ok(
        Math.abs(outer.y - heading.y - heading.height - 20) < 2,
        `${kind}/${width}: page heading gap must not be doubled`,
      );
      assert.ok(
        Math.abs(tableBox.x - (outer.x + 1)) < 2,
        `${kind}/${width}: table left edge must meet card border`,
      );
      assert.ok(
        Math.abs(tableBox.x + tableBox.width - (outer.x + outer.width - 1)) < 2,
        `${kind}/${width}: table right edge must meet card border`,
      );
      assert.ok(
        await table.evaluate((e) => e.scrollWidth > e.clientWidth),
        `${kind}/${width}: table must retain horizontal scrolling`,
      );
      const operation = row.locator(".t-table__cell--fixed-right").last();
      const before = await operation.boundingBox();
      const idBefore = await row.locator("td").first().boundingBox();
      await table.evaluate((e) => {
        e.scrollLeft = e.scrollWidth;
      });
      await page.waitForFunction(
        () =>
          document.querySelector(".resource-card .t-table__content")
            .scrollLeft > 0,
      );
      const after = await operation.boundingBox(),
        idAfter = await row.locator("td").first().boundingBox();
      assert.ok(
        Math.abs(before.x - after.x) < 2,
        `${kind}/${width}: right operations must remain fixed`,
      );
      assert.ok(
        idAfter.x < idBefore.x - 100,
        `${kind}/${width}: content columns must actually scroll`,
      );
      assert.equal(
        await page.evaluate(
          () => document.documentElement.scrollWidth > innerWidth,
        ),
        false,
      );
      assert.deepEqual(errors, []);
      await page.close();
    }
  }
  console.log(
    "Publication list geometry: plan/batch at 1440/1024, wider search, flush card/table, no extra divider, horizontal scroll and fixed operations PASS",
  );
} finally {
  await browser.close();
}
