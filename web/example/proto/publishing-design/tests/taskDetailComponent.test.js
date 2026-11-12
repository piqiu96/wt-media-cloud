import { it, expect } from "vitest";
import { createSSRApp, h } from "vue";
import { renderToString } from "@vue/server-renderer";
import TaskDetail from "../TaskDetail.vue";
import { seed, states } from "../model.js";
for (const status of states)
  it(`renders readonly ${status} in approved section order`, async () => {
    const db = seed();
    const task = { ...db.tasks[0], status, attemptHistory: [], log: [] };
    const app = createSSRApp(TaskDetail, { task, db });
    app.component("t-tag", {
      setup(_, { slots }) {
        return () => h("span", slots.default?.());
      },
    });
    app.component("t-button", {
      setup(_, { slots }) {
        return () => h("button", slots.default?.());
      },
    });
    const html = await renderToString(app);
    expect(html.indexOf("任务摘要")).toBeLessThan(html.indexOf(">发布内容<"));
    expect(html.indexOf(">发布内容<")).toBeLessThan(html.indexOf(">执行历史<"));
    expect(html).toContain(
      status === "待执行" ? "任务尚未执行" : "暂无已保存的执行记录",
    );
    expect(html).not.toMatch(
      /<input|<textarea|保存并关闭|当前执行进度|安全提示|查看原始日志/,
    );
    if (status === "执行失败") expect(html).toContain("发布失败");
  });

async function renderDetail(overrides = {}) {
  const db = seed();
  const task = { ...db.tasks[0], ...overrides };
  const app = createSSRApp(TaskDetail, { task, db });
  for (const name of ["t-tag", "t-button"])
    app.component(name, {
      setup:
        (_, { slots }) =>
        () =>
          h("span", slots.default?.()),
    });
  return renderToString(app);
}
it("places identity and status in the summary instead of the title", async () => {
  const html = await renderDetail();
  const header = html.match(/<header[^>]*>([\s\S]*?)<\/header>/)[1];
  expect(header).not.toContain("任务 #");
  expect(header).not.toContain("待执行");
  const summary = html.match(/<dl class="v4-summary"[^>]*>([\s\S]*?)<\/dl>/)[1];
  expect(summary).toContain("任务 ID");
  expect(summary).toContain("任务状态");
  expect(summary).toContain("1001");
});
it("shows an unknown result at confirmation without marking publication complete", async () => {
  const html = await renderDetail({ status: "结果待确认" });
  expect(html).toContain('aria-label="发布进度"');
  const progress = html.match(/<ol[^>]*aria-label="发布进度"[\s\S]*?<\/ol>/)[0];
  const steps = progress.match(/<li\b[\s\S]*?<\/li>/g);
  expect(steps[2]).toContain('aria-current="step"');
  expect(steps[2]).toContain("确认结果");
  expect(steps[3]).not.toContain("is-complete");
});
it("does not invent completed steps for a task cancelled before execution", async () => {
  const html = await renderDetail({
    status: "已取消",
    attempts: 0,
    attemptHistory: [],
  });
  const progress = html.match(/<ol[^>]*aria-label="发布进度"[\s\S]*?<\/ol>/)[0];
  expect(progress).not.toContain("is-complete");
  expect(progress).toContain("已取消");
});
it("handles malformed historical dates without showing Invalid Date", async () => {
  const html = await renderDetail({ updatedAt: "not-a-date" });
  expect(html).not.toContain("Invalid Date");
  expect(html).toContain("尚无记录");
});
it("links the existing source asset and displays the task ID without a hash", async () => {
  const html = await renderDetail();
  expect(html).toContain('aria-label="查看来源成片 128"');
  expect(html).not.toContain("#1001");
  expect(html).not.toContain("#128");
});
it("does not expose a misleading source link when the associated asset is missing", async () => {
  const html = await renderDetail({ assetId: 99999 });
  expect(html).not.toContain('aria-label="查看来源成片 99999"');
  expect(html).toContain("成片记录缺失");
});
