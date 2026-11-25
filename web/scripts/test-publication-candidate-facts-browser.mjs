// Current catalog and selected-cover facts only; all HTTP is intercepted, never real publication E2E.
import assert from "node:assert/strict";
import { chromium } from "playwright";
const browser = await chromium.launch({
  channel: process.env.WT_MEDIA_BROWSER_CHANNEL || "chrome",
  headless: true,
});
const base = process.env.WT_MEDIA_BROWSER_BASE_URL || "http://127.0.0.1:5173";
const longName="复制成片长名称用于验证安装版发布内容列在真实文件名下仍保持两行展示".repeat(3);
const coverPath = "/api/v1/file-resources/" + "a".repeat(64),
  thumbPath = "/api/v1/file-resources/" + "c".repeat(64);
try {
  for (const [scene, message] of [
    ["disabled", "账号当前不可用"],
    ["occupied", "该成片已被当前平台的任务占用"],
    ["unknown", "视频文件不可用"],
    ["missing", "资源待核实"],
    ["wrong-category", "成片类目不匹配"],
    ["wrong-team", "成片团队不匹配"],
    ["cover-missing", null],
    ["cover-known", null],
  ]) {
    const page = await browser.newPage({
        viewport: { width: Number(process.env.WT_MEDIA_VIEWPORT_WIDTH)||1440, height: 1000 },
      }),
      errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    const row = {
      row_key: "r1",
      category_id: "4",
      media_account_id: "11",
      composite_output_id: "41",
      input_revision: "1",
      field_revisions: {},
      content: {
        description: "完整确认的发布文案",
        cover_file_id: "31",
        portrait_cover_file_id: "32",
        declaration: "original",
        confirmed: true,
      },
    };
    const account = {
      id: "11",
      name: "本人账号",
      platform: "baidu",
      eligible: scene !== "disabled",
      reason: scene === "disabled" ? "账号当前不可用" : "",
      category_ids: ["4"],
    };
    const output = {
      composite_output_id: "41",
      name: longName,
      category_id: scene === "wrong-category" ? "5" : "4",
      team_id: scene === "wrong-team" ? "8" : "7",
      occupied: scene === "occupied",
      platforms: ["baidu"],
      video: { status: scene === "unknown" ? "unknown" : "ready" },
      thumbnail: { file_id: "91", resource_path: thumbPath },
    };
    const resources =
      scene === "cover-known"
        ? [
            {
              row_key: "r1",
              cover_resource_path: coverPath,
              portrait_cover_resource_path:
                "/api/v1/file-resources/" + "b".repeat(64),
            },
          ]
        : [];
    const batch = {
      id: "2",
      revision: "1",
      name: "候选事实核对",
      operator_id: "9",
      platform: "baidu",
      allowed_actions: ["edit"],
      execution_status: "not_started",
      configuration_status: "ready",
      targets: [{ category_id: "4", target_count: 1 }],
      config: {
        accounts: [{ media_account_id: "11", limit: 1 }],
        output_ids: ["41"],
        manual_pairs: [],
        candidates: [row],
      },
      readiness: {
        ready: true,
        issues: [],
        shortfalls: [],
        cover_resources: resources,
      },
      execution_setting: {
        concurrency_limit: 1,
        group_interval_seconds: 0,
        between_group_policy: "auto_continue",
        exception_task_limit: 1,
      },
    };
    await page.route("**/api/v1/**", async (route) => {
      const request = route.request(),
        p = new URL(request.url()).pathname;
      const send = (data) =>
        route.fulfill({
          json: {
            errcode: 0,
            message: "success",
            data,
            logid: "candidate-facts",
          },
        });
      if (p === "/api/v1/auth/me")
        return send({
          id: "9",
          username: "operator",
          role: "operator",
          team_id: "7",
          status: "enabled",
          category_ids: ["4"],
        });
      if (p === "/api/v1/categories")
        return send([{ id: "4", name: "三角洲行动", enabled: true }]);
      if (p === "/api/v1/publication-batch/2") return send(batch);
      if (p === "/api/v1/publication/account")
        return send({
          items: scene === "missing" ? [] : [account],
          total: scene === "missing" ? 0 : 1,
        });
      if (p === "/api/v1/publication/output")
        return send({
          items: scene === "missing" ? [] : [output],
          total: scene === "missing" ? 0 : 1,
        });
      if (p === "/api/v1/publication-batch/validate")
        return send({
          ready: true,
          issues: [],
          shortfalls: [],
          cover_resources: resources,
        });
      assert.equal(
        request.method(),
        "GET",
        "no save or execution command in facts checks",
      );
      return send({ items: [], total: 0 });
    });
    await page.goto(base + "/publish-batches?view=edit&id=2");
    await page.getByRole("button", { name: "下一步", exact: true }).click();
    for (const sectionName of ["内容配置", "发布内容"]) {
      const section = page.getByRole("region", {
        name: sectionName,
        exact: true,
      });
      const candidate = section.locator("tbody tr");
      await candidate.waitFor();
      if(scene!=="missing"){
        const title=candidate.locator('button.resource-name-link > span.batch-content-asset');
        assert.equal(await title.count(),1,"clamp must be applied to the inner span, not the native button");
        assert.equal(await title.textContent(),longName);assert.equal(await title.getAttribute('title'),longName);
        const geometry=await title.evaluate(element=>({height:element.getBoundingClientRect().height,line:parseFloat(getComputedStyle(element).lineHeight)}));
        assert.ok(geometry.height<=2*geometry.line+1,"inner resource title must remain within two visible lines");
      }

      if (message) {
        assert.equal(
          await candidate.getByText("已就绪", { exact: true }).count(),
          0,
        );
        await candidate.getByText(message, { exact: true }).waitFor();
      }
      if (scene === "cover-missing") {
        assert.equal(
          await candidate.getByAltText("发布封面", { exact: true }).count(),
          0,
        );
        await candidate
          .getByText("发布封面引用待核实", { exact: true })
          .waitFor();
      }
      if (scene === "cover-known") {
        const image = candidate.getByAltText("发布封面", { exact: true });
        assert.ok((await image.getAttribute("src")).endsWith(coverPath));
        assert.notEqual(await image.getAttribute("src"), thumbPath);
      }
      if (sectionName === "内容配置")
        await page.getByRole("button", { name: "下一步", exact: true }).click();
    }
    assert.deepEqual(errors, []);
    await page.close();
  }
  console.log(
    "Candidate facts: full confirmed content never masks unavailable catalog; content/review both reject disabled/occupied/unknown/missing/wrong scope; selected-cover references never substitute video thumbnails PASS",
  );
} finally {
  await browser.close();
}
