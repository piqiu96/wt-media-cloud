import { describe, it, expect } from "vitest";
import {
  nextBatch,
  stats,
  validate,
  defaults,
  seed,
  canRetry,
} from "../model.js";
import { layoutText } from "../../../../src/shared/ui/media/textLayout.js";
describe("publishing safety and accounting", () => {
  it("unknown and human states block advancement and retry", () => {
    for (const status of ["结果待确认", "等待人工", "排队中", "执行中"]) {
      expect(nextBatch([{ status }])).toBe("等待核实");
      expect(canRetry({ status, result: "未知" })).toBe(false);
    }
  });
  it("10 percent is inclusive and cancellation is excluded", () => {
    const rows = Array.from({ length: 9 }, () => ({ status: "平台审核中" }));
    rows.push(
      { status: "执行失败", result: "确认未发布" },
      { status: "已取消" },
    );
    expect(nextBatch(rows)).toBe("自动继续");
    rows.push({ status: "执行失败", result: "确认未发布" });
    expect(nextBatch(rows)).toBe("等待继续确认");
    expect(nextBatch([{ status: "已取消" }])).toBe("等待跳过确认");
  });
  it("only public works count toward target and unique resources do not determine target", () => {
    expect(
      stats(
        [{ status: "已发布" }, { status: "平台审核中" }, { status: "已取消" }],
        3,
      ),
    ).toMatchObject({
      published: 1,
      reviewing: 1,
      gap: 2,
      rate: 33,
      finished: 2,
    });
  });
  it("cancelled tasks reserve video; abandonment releases only that reservation", () => {
    const db = seed();
    db.tasks = [{ id: 1, platform: "B站", assetId: 128, status: "已取消" }];
    const row = {
      platform: "B站",
      accountId: 11,
      assetId: 128,
      ...defaults("B站", db.assets[0]),
      declaration: "内容无需标注",
    };
    expect(validate(row, db)).toContain("成片已被同平台任务占用");
    db.tasks[0].status = "已放弃";
    expect(validate(row, db)).toEqual([]);
    row.accountId = 12;
    expect(validate(row, db)).toContain("账号与成片类目不匹配");
  });
});
describe("shared image text layout", () => {
  const measure = (s, size) => Array.from(s).length * size;
  it("wraps and shrinks without losing content", () => {
    const result = layoutText(
      {
        text: "abcdefgh",
        w: 40,
        h: 50,
        fontSize: 20,
        minFont: 10,
        maxLines: 2,
      },
      measure,
    );
    expect(result.overflow).toBe(false);
    expect(result.lines.join("")).toBe("abcdefgh");
    expect(result.size).toBeLessThan(20);
  });
  it("reports unreadable text instead of clipping", () => {
    const result = layoutText(
      {
        text: "abcdefgh",
        w: 10,
        h: 10,
        fontSize: 20,
        minFont: 10,
        maxLines: 1,
      },
      measure,
    );
    expect(result.overflow).toBe(true);
    expect(result.lines.join("")).toBe("abcdefgh");
  });
});

import {
  resourceStatus,
  resourceInventory,
  suggestDistribution,
  mergeContentDefaults,
} from "../model.js";
describe("multi-platform resource decisions", () => {
  it("counts platform opportunities separately and excludes incomplete files", () => {
    const db = seed();
    db.tasks = [
      { platform: "B站", assetId: 128, status: "已取消" },
      { platform: "小红书", assetId: 130, status: "已发布" },
    ];
    const inventory = resourceInventory(db, ["小红书", "B站"], ["三角洲行动"]);
    expect(inventory.map((r) => r.available)).toEqual([5, 5]);
    expect(inventory.map((r) => r.unavailable)).toEqual([1, 1]);
    expect(
      resourceStatus(
        db,
        db.assets.find((v) => v.id === 140),
        "小红书",
      ),
    ).toBe("成片仍在生成");
    const rows = suggestDistribution(
      db,
      ["小红书", "B站"],
      [128, 130, 132, 140],
      [1, 11],
    );
    expect(rows).toHaveLength(4);
    expect(new Set(rows.map((r) => r.assetId)).size).toBe(3);
    expect(
      rows.every(
        (r) =>
          !resourceStatus(
            db,
            db.assets.find((v) => v.id === r.assetId),
            r.platform,
          ),
      ),
    ).toBe(true);
  });
  it("matches category and enabled accounts; keeps manually changed candidate fields", () => {
    const db = seed();
    db.tasks = [];
    const rows = suggestDistribution(
      db,
      ["小红书", "B站"],
      [128, 129],
      [1, 2, 11],
    );
    expect(rows).toHaveLength(3);
    const custom = {
      ...rows[0],
      title: "保留标题",
      declaration: "单条声明",
      overrides: ["title", "declaration"],
    };
    const retained = suggestDistribution(
      db,
      ["小红书", "B站"],
      [128, 129],
      [1, 2, 11],
      [custom],
    );
    expect(retained.find((r) => r.key === custom.key)).toEqual(custom);
    const merged = mergeContentDefaults([custom], "小红书", {
      declaration: "平台声明",
      description: "新简介",
    });
    expect(merged[0]).toMatchObject({
      title: "保留标题",
      declaration: "单条声明",
      description: "新简介",
    });
    expect(
      mergeContentDefaults(
        [custom],
        "小红书",
        { declaration: "覆盖声明" },
        true,
      )[0].overrides,
    ).toEqual(["title"]);
  });
  it("revalidates eligibility and does not advance a batch with pending tasks", () => {
    const db = seed();
    db.tasks = [];
    const row = {
      platform: "小红书",
      accountId: 1,
      assetId: 140,
      ...defaults(
        "小红书",
        db.assets.find((v) => v.id === 140),
      ),
      declaration: "已确认",
    };
    expect(validate(row, db)).toContain("成片仍在生成");
    expect(nextBatch([{ status: "已发布" }, { status: "待执行" }])).toBe(
      "等待核实",
    );
    expect(resourceStatus(db, db.assets[0], "小红书", [])).toBe("没有匹配账号");
  });
});

it("respects per-account batch limits without creating a cartesian product", () => {
  const db = seed();
  db.tasks = [];
  const rows = suggestDistribution(
    db,
    ["小红书", "B站"],
    [128, 130, 132],
    [1, 11],
    [],
    { 1: 1, 11: 2 },
  );
  expect(rows.filter((r) => r.accountId === 1)).toHaveLength(1);
  expect(rows.filter((r) => r.accountId === 11)).toHaveLength(2);
  expect(new Set(rows.map((r) => `${r.platform}:${r.assetId}`)).size).toBe(3);
  const excluded = resourceInventory(db, ["小红书"], ["三角洲行动"], [])[0];
  expect(excluded.accounts).toBe(0);
  expect(excluded.available).toBe(0);
});
