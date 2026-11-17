import { describe, it, expect } from "vitest";
import { seed, canRetry, declarationOptions } from "../model.js";
import {
  taskPresentation,
  canEditTask,
  canCancelTask,
  canReleaseTask,
} from "../taskExperience.js";
import { queueTask, setResult } from "../relationships.js";
function fixture(status = "待执行") {
  const db = seed();
  db.tasks = [];
  const a = db.assets[0],
    account = db.accounts[0];
  const t = {
    id: 9001,
    platform: account.platform,
    accountId: account.id,
    assetId: a.id,
    category: a.category,
    title: "有效标题",
    description: "",
    cover: a.cover,
    declaration: declarationOptions(account.platform)[0].value,
    status,
    result: "未提交",
    attempts: 0,
  };
  db.tasks.push(t);
  return { db, t };
}
describe("task experience safety", () => {
  it("requires actual preflight instead of pending label", () => {
    const { db, t } = fixture();
    t.title = "";
    expect(taskPresentation(t, db).disabled).toBe(true);
    t.title = "有效标题";
    expect(taskPresentation(t, db).label).toBe("执行");
    expect(taskPresentation(t, db).disabled).toBe(false);
    db.accounts[0].loginValid = false;
    expect(taskPresentation(t, db).disabled).toBe(true);
  });
  it.each(["结果待确认", "平台审核中", "已发布", "执行中", "等待人工"])(
    "%s cannot edit, cancel, release or queue again",
    (status) => {
      const { db, t } = fixture(status);
      t.result = status === "结果待确认" ? "未知" : "平台已接收";
      expect(canEditTask(t)).toBe(false);
      expect(canCancelTask(t)).toBe(false);
      expect(canReleaseTask(t)).toBe(false);
      expect(queueTask(db, t, { retry: true })).toBe(false);
    },
  );
  it("does not retry while the old attempt is still running", () => {
    const { db, t } = fixture("执行失败");
    t.result = "确认未发布";
    t.attemptHistory = [{ number: 1, status: "执行失败" }];
    expect(canRetry(t)).toBe(false);
    expect(queueTask(db, t, { retry: true })).toBe(false);
    t.attemptHistory[0].ended = "2026-11-11T00:00:00Z";
    expect(canRetry(t)).toBe(true);
    t.claimed = true;
    expect(canRetry(t)).toBe(false);
  });
  it("safe retry appends an attempt and retains old content evidence", () => {
    const { db, t } = fixture();
    queueTask(db, t);
    setResult(t, "执行失败", { evidence: "明确未发布" });
    const old = JSON.stringify(t.attemptHistory[0]);
    t.title = "修复后的标题";
    t.version = 2;
    t.snapshot = { title: t.title };
    expect(queueTask(db, t, { retry: true })).toBe(true);
    expect(t.attemptHistory).toHaveLength(2);
    expect(JSON.stringify(t.attemptHistory[0])).toBe(old);
    expect(t.attemptHistory[1].snapshot.title).toBe("修复后的标题");
  });
  it("cancelled task stays terminal until explicit release and new creation", () => {
    const { db, t } = fixture("已取消");
    expect(canEditTask(t)).toBe(false);
    expect(queueTask(db, t)).toBe(false);
    expect(canReleaseTask(t)).toBe(true);
    t.result = "未知";
    expect(canReleaseTask(t)).toBe(false);
  });
  it("missing execution-end evidence does not authorize retry", () => {
    const { t } = fixture("执行失败");
    t.result = "确认未发布";
    expect(canRetry(t)).toBe(false);
    t.executionEndedAt = "2026-11-11T00:00:00Z";
    expect(canRetry(t)).toBe(true);
    t.platformId = "existing-platform-draft";
    expect(canRetry(t)).toBe(false);
  });
  it("claiming blocks pending or queued edits and cancellation", () => {
    const { t } = fixture("排队中");
    t.claimed = true;
    expect(canEditTask(t)).toBe(false);
    expect(canCancelTask(t)).toBe(false);
  });
  it("reviewing can supplement a public result without counting as public success", () => {
    const { db, t } = fixture("平台审核中");
    const p = taskPresentation(t, db);
    expect(p.action).toBe("supplement");
    expect(p.description).toContain("不计入已发布目标");
  });
});
