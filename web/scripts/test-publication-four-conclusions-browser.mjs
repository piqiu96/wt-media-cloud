// Production owner/manager dialog and HTTP payload checks, using controlled original attempts.
import assert from "node:assert/strict";
import { chromium } from "playwright";
const browser = await chromium.launch({
  channel: process.env.WT_MEDIA_BROWSER_CHANNEL || "chrome",
  headless: true,
});
const base = process.env.WT_MEDIA_BROWSER_BASE_URL || "http://127.0.0.1:5173";
try {
  for (const [role, conclusion] of [
    ["operator", "published"],
    ["operator", "reviewing"],
    ["operator", "failed"],
    ["operator", "uncertain"],
    ["admin", "published"],
  ]) {
    const page = await browser.newPage({
        viewport: { width: 1440, height: 1000 },
      }),
      errors = [],
      posts = [];
    page.on("pageerror", (e) => errors.push(e.message));
    let expectedConclusion = conclusion;
    let current = {
      id: "21",
      batch_id: "1",
      revision: "7",
      operator_id: "9",
      category_id: "1",
      media_account_id: "11",
      composite_output_id: "41",
      platform: "baidu",
      platform_account_uid: "123",
      current_attempt_id: "70",
      platform_submission_id: "18001",
      status: "reviewing",
      allowed_actions: ["resolve"],
      content: { description: "四结论原内容" },
      resources: { account_name: "本人账号", output_name: "本人视频" },
    };
    const attempt = {
      id: "70",
      publication_id: "21",
      executor_user_id: "9",
      status: "completed",
      result_hash: "hash",
      submit_started_at: "2026-11-25T03:00:00Z",
      input_snapshot: { platform_uid: "123" },
      evidence: {
        status: "reviewing",
        platform_work_id: "18001",
        evidence: {
          scene_closed: true,
          platform_uid: "123",
          receipt_id: "18001",
        },
      },
    };
    await page.route("**/api/v1/**", async (route) => {
      const r = route.request(),
        p = new URL(r.url()).pathname;
      const send = (data) =>
        route.fulfill({
          json: {
            errcode: 0,
            message: "success",
            data,
            logid: "four-conclusions",
          },
        });
      if (p === "/api/v1/auth/me")
        return send({
          id: role === "admin" ? "99" : "9",
          role,
          team_id: "7",
          status: "enabled",
          username: role,
          category_ids: ["1"],
        });
      if (p === "/api/v1/categories")
        return send([{ id: "1", name: "游戏", enabled: true }]);
      if (p === "/api/v1/publication/21/resolve") {
        const input = r.postDataJSON();
        posts.push(input);
        assert.equal(input.attempt_id, "70");
        assert.equal(input.expected_revision, current.revision);
        assert.equal(input.conclusion, expectedConclusion);
        assert.ok(input.evidence.basis.length >= 10);
        assert.equal(input.evidence.platform_uid, "123");
        if (input.conclusion === "published") {
          assert.equal(input.published_at, "2026-11-25T04:00:00.000Z");
          assert.equal(input.platform_submission_id, "18001");
          assert.equal(input.platform_work_id, "70001");
          assert.notEqual(input.platform_work_id, input.platform_submission_id);
          assert.equal(input.evidence.publicly_accessible, true);
        }
        if (input.conclusion === "reviewing") {
          assert.equal(input.platform_submission_id, "18001");
          assert.equal(input.evidence.scene_closed, true);
          assert.equal(input.platform_work_id, undefined);
        }
        if (input.conclusion === "failed") {
          assert.equal(input.evidence.scene_closed, true);
          assert.equal(input.evidence.no_publishable_work, true);
          assert.equal(input.platform_submission_id, undefined);
        }
        if (input.conclusion === "uncertain") {
          assert.equal(input.platform_submission_id, undefined);
          assert.equal(input.evidence.publicly_accessible, false);
        }
        current = {
          ...current,
          status: input.conclusion,
          revision: String(Number(current.revision) + 1),
        };
        return send(current);
      }
      if (p === "/api/v1/publication/21") return send(current);
      if (p === "/api/v1/publication/21/attempt") return send([attempt]);
      if (p === "/api/v1/publication")
        return send({ items: [current], total: 1 });
      if (r.method() !== "GET") throw Error("Unexpected write " + p);
      return send({ items: [], total: 0 });
    });
    await page.goto(base + "/publish?publication_id=21&from_batch=1");
    await page
      .getByRole("heading", { name: "发布任务详情", exact: true })
      .waitFor();
    await page.getByRole("button", { name: "补录结果", exact: true }).click();
    const dialog = page.getByRole("dialog", {
      name: role === "admin" ? "补录公开作品" : "核实发布结果",
    });
    await dialog.waitFor();
    if (role === "operator") {
      await dialog.locator('[aria-label="核实结论"]').click();
      await page
        .locator(".t-select-option")
        .getByText("已公开", { exact: true })
        .waitFor();
      for (const label of ["已公开", "仍在审核", "确认未发布", "仍无法判断"])
        assert.equal(
          await page
            .locator(".t-select-option")
            .getByText(label, { exact: true })
            .count(),
          1,
        );
      const label = {
        published: "已公开",
        reviewing: "仍在审核",
        failed: "确认未发布",
        uncertain: "仍无法判断",
      }[conclusion];
      await page
        .locator(".t-select-option")
        .getByText(label, { exact: true })
        .click();
    } else {
      assert.equal(
        await dialog.locator('[aria-label="核实结论"] input').isDisabled(),
        true,
      );
    }
    const input = (label) =>
      dialog
        .locator(`[aria-label="${label}"] input, input[aria-label="${label}"]`)
        .first();
    await input("核实原因").fill("已核对原稿件");
    await dialog
      .locator(
        '[aria-label="核实依据"] textarea,textarea[aria-label="核实依据"]',
      )
      .fill("逐项核对原账号、指定稿件与实际视频内容，保留原尝试证据");
    if (conclusion === "published") {
      assert.equal(await input("投稿回执 ID").inputValue(), "18001");
      assert.equal(await input("公开作品 ID").inputValue(), "");
      await input("公开作品 ID").fill("70001");
      await input("公开作品链接").fill("https://example.com/work/70001");
      await dialog
        .locator(
          '[aria-label="平台实际发布时间"] input,input[aria-label="平台实际发布时间"]',
        )
        .first()
        .fill("2026-11-25 12:00:00");
      await dialog
        .locator(
          '[aria-label="平台实际发布时间"] input,input[aria-label="平台实际发布时间"]',
        )
        .first()
        .press("Enter");
      await dialog.getByText("核实原因", { exact: false }).first().click();
    }
    if (role === "operator" && conclusion !== "uncertain")
      await dialog
        .getByText("已核实原执行进程与浏览器已安全结束", { exact: true })
        .click();
    if (["published", "reviewing"].includes(conclusion))
      await dialog
        .getByText("已核实指定稿件与原账号、视频及发布内容一致", {
          exact: true,
        })
        .click();
    if (conclusion === "failed") {
      await dialog
        .getByRole("button", { name: "保存核实结果", exact: true })
        .click();
      assert.equal(posts.length, 0);
      await dialog
        .getByText("请确认原账号没有可继续发布的稿件。", { exact: true })
        .waitFor();
      await dialog
        .getByText("已核实原账号没有可继续发布的稿件", { exact: true })
        .click();
    }
    if (conclusion === "published")
      await dialog
        .getByText("已核实公开作品无需登录即可访问", { exact: true })
        .click();
    await dialog
      .getByRole("button", {
        name: role === "admin" ? "保存补录" : "保存核实结果",
        exact: true,
      })
      .click();
    await dialog.waitFor({ state: "hidden" });
    assert.equal(posts.length, 1);
    assert.equal(current.current_attempt_id, "70");
    assert.equal(current.platform_submission_id, "18001");
    if (conclusion === "uncertain") {
      expectedConclusion = "failed";
      await page.getByRole("button", { name: "核实结果", exact: true }).click();
      const again = page.getByRole("dialog", { name: "核实发布结果" });
      await again.waitFor();
      await again.locator('[aria-label="核实结论"]').click();
      await page
        .locator(".t-select-option")
        .getByText("确认未发布", { exact: true })
        .click();
      await again
        .locator('[aria-label="核实原因"] input,input[aria-label="核实原因"]')
        .first()
        .fill("再次核实明确拒发");
      await again
        .locator(
          '[aria-label="核实依据"] textarea,textarea[aria-label="核实依据"]',
        )
        .fill("再次检查原账号指定稿件和原执行现场，确认未公开且无可继续稿件");
      await again
        .getByText("已核实原执行进程与浏览器已安全结束", { exact: true })
        .click();
      await again
        .getByText("已核实原账号没有可继续发布的稿件", { exact: true })
        .click();
      await again
        .getByRole("button", { name: "保存核实结果", exact: true })
        .click();
      await again.waitFor({ state: "hidden" });
      assert.equal(posts.length, 2);
      assert.equal(posts[1].attempt_id, "70");
      assert.equal(posts[1].expected_revision, "8");
      assert.equal(current.platform_submission_id, "18001");
    }
    assert.deepEqual(errors, []);
    await page.close();
  }
  console.log(
    "Four-conclusion production dialog: owner published/reviewing/failed/uncertain, failed evidence negative, receipt distinct from public work, manager public-only and original attempt/revision payload PASS",
  );
} finally {
  await browser.close();
}
