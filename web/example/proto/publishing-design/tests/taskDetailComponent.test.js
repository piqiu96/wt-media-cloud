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
    expect(html).toContain("任务尚未执行");
    expect(html).not.toMatch(
      /<input|<textarea|保存并关闭|当前执行进度|安全提示|查看原始日志/,
    );
    if (status === "执行失败") expect(html).toContain("发布失败");
  });
