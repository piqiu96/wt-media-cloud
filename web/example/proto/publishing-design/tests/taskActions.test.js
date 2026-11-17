import { describe, it, expect } from "vitest";
import {
  allowedActions,
  displayStatus,
  canEditTask,
} from "../taskExperience.js";
import {
  verifyResult,
  cancelPublication,
  processHuman,
  publicWork,
} from "../taskActions.js";
const task = (status = "结果待确认") => ({
  id: 1,
  status,
  platform: "B站",
  accountId: 1,
  attempts: 1,
  attemptHistory: [
    { number: 1, ended: "2026-11-11", snapshot: { title: "旧标题" } },
  ],
  result: "未知",
});
describe("V4 10×8 action matrix", () => {
  const matrix = {
    待执行: ["detail", "edit", "start", "cancel"],
    排队中: ["detail", "cancel"],
    执行中: ["detail"],
    等待人工: ["detail", "human"],
    结果待确认: ["detail", "verify"],
    平台审核中: ["detail"],
    执行失败: ["detail", "retry", "copy"],
    已发布: ["detail", "copy"],
    已取消: ["detail", "copy"],
    已放弃: ["detail", "copy"],
  };
  for (const [state, expected] of Object.entries(matrix))
    for (const action of [
      "detail",
      "edit",
      "start",
      "cancel",
      "human",
      "verify",
      "retry",
      "copy",
    ])
      it(`${state} ${action}`, () => {
        let t = task(state);
        t.claimed = false;
        t.result = "确认未发布";
        expect(allowedActions(t).includes(action)).toBe(
          expected.includes(action),
        );
      });
  it("keeps persisted enum and displays 发布失败", () =>
    expect(displayStatus(task("执行失败"))).toBe("发布失败"));
  it("queued task cannot edit", () =>
    expect(canEditTask(task("排队中"))).toBe(false));
});
describe("evidence-bound operations", () => {
  it("rejects profile URLs and foreign domains", () => {
    for (const url of [
      "https://space.bilibili.com/123",
      "https://evil.test/video/BV123",
      "https://www.bilibili.com/account",
    ])
      expect(() => publicWork("B站", url)).toThrow();
  });
  it("normalizes public identities", () =>
    expect(
      publicWork("B站", "https://www.bilibili.com/video/BV123/?spm_id_from=x")
        .id,
    ).toBe("BV123"));
  it("unknown result may keep optional note", () => {
    let t = task();
    verifyResult(
      { works: [] },
      t,
      { status: "结果待确认", attempt: 1 },
      "运营",
    );
    expect(t.status).toBe("结果待确认");
    expect(t.verifications).toHaveLength(1);
  });
  it("success needs public work and deduplicates association", () => {
    let db = { works: [] };
    for (let n = 0; n < 2; n++) {
      let t = task();
      verifyResult(
        db,
        t,
        {
          status: "已发布",
          url: "https://www.bilibili.com/video/BV123",
          publicAt: "2026-01-01T00:00:00Z",
          attempt: 1,
        },
        "运营",
      );
      expect(t.workId).toBe(db.works[0].id);
    }
    expect(db.works).toHaveLength(1);
  });
  it("failure cannot be authorized by a checkbox or note", () => {
    let t = task();
    expect(() =>
      verifyResult(
        {},
        t,
        {
          status: "执行失败",
          reason: "失败",
          evidence: "勾选确认",
          attempt: 1,
        },
        "运营",
      ),
    ).toThrow();
    expect(t.status).toBe("结果待确认");
  });
  it("rejects stale attempt and missing reviewer", () => {
    expect(() =>
      verifyResult({}, task(), { status: "结果待确认", attempt: 2 }, "运营"),
    ).toThrow();
    expect(() =>
      verifyResult({}, task(), { status: "结果待确认", attempt: 1 }, ""),
    ).toThrow();
  });
  it("review requires submission evidence", () =>
    expect(() =>
      verifyResult({}, task(), { status: "平台审核中", attempt: 1 }, "运营"),
    ).toThrow());
  it("cancellation never frees reservation or trusts safe checkbox", () => {
    let t = task("待执行");
    t.result = "未提交";
    cancelPublication(t, "重复配置");
    expect(t.status).toBe("已取消");
    expect(t.released).not.toBe(true);
    let active = task("执行中");
    expect(() => cancelPublication(active, "")).toThrow();
    expect(active.status).toBe("执行中");
  });
  it("human possible submission blocks any further upload", () => {
    let t = task("等待人工");
    processHuman(t, "submitted");
    expect(t.status).toBe("结果待确认");
    expect(t.attempts).toBe(1);
  });
  it("human validation without executor evidence cannot resume", () => {
    let t = task("等待人工");
    expect(() => processHuman(t, "validated")).toThrow();
    processHuman(t, "later");
    expect(t.status).toBe("等待人工");
  });
});
import { accountAvailability } from "../accountAvailability.js";
describe("account capacity uses observed data", () => {
  it("unknown capacity stays unknown", () =>
    expect(
      accountAvailability(
        { tasks: [] },
        { enabled: true, platform: "B站", categories: ["游戏"] },
        "B站",
        "游戏",
      ).remaining,
    ).toBeNull());
  it("zero quota and mismatched category cannot be chosen", () => {
    const a = {
      id: 1,
      enabled: true,
      platform: "B站",
      categories: ["游戏"],
      dailyCapacity: 0,
    };
    expect(
      accountAvailability({ tasks: [] }, a, "B站", "游戏").reason,
    ).toContain("额度");
    expect(
      accountAvailability({ tasks: [] }, a, "B站", "其他").reason,
    ).toContain("类目");
  });
  it("counts today once and excludes the current reservation", () => {
    const a = {
      id: 1,
      enabled: true,
      platform: "B站",
      categories: ["游戏"],
      dailyCapacity: 2,
    };
    const db = {
      tasks: [
        {
          id: 1,
          accountId: 1,
          status: "待执行",
          created: "2026-11-12T01:00:00Z",
        },
        {
          id: 2,
          accountId: 1,
          status: "平台审核中",
          submittedAt: "2026-11-12T02:00:00Z",
        },
      ],
    };
    expect(
      accountAvailability(db, a, "B站", "游戏", 1, "2026-11-12T03:00:00Z")
        .remaining,
    ).toBe(1);
  });
});
it("failure verification uses executor evidence tied to the current attempt", () => {
  const t = task();
  t.executorEvidence = { attempt: 1, confirmedUnpublished: true };
  verifyResult(
    {},
    t,
    {
      status: "执行失败",
      attempt: 1,
      reason: "平台拒绝",
      evidence: "拒绝回执",
    },
    "运营",
  );
  expect(t.status).toBe("执行失败");
  expect(t.retrySafe).toBe(true);
  expect(t.verifications[0].attempt).toBe(1);
});
it("human completed validation resumes original attempt only with executor evidence", () => {
  const t = task("等待人工");
  t.executorEvidence = { safeToResume: true, attempt: 1 };
  processHuman(t, "validated");
  expect(t.status).toBe("执行中");
  expect(t.attempts).toBe(1);
  expect(t.attemptHistory).toHaveLength(1);
});

it("executor stop evidence does not override unknown platform outcome", () => {
  const t = task("执行中");
  t.executorEvidence = { safeToCancel: true, attempt: 1 };
  expect(() => cancelPublication(t)).toThrow();
  t.result = "未提交";
  cancelPublication(t);
  expect(t.status).toBe("已取消");
});
