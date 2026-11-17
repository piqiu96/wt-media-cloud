import { it, expect } from "vitest";
import { defaultPublishingResources } from "../quickPublishSelection.js";

function fixture() {
  return {
    tasks: [],
    accounts: [
      { id: 1, platform: "B站", categories: ["游戏"], enabled: true },
      { id: 2, platform: "小红书", categories: ["美食"], enabled: true },
      { id: 3, platform: "小红书", categories: ["游戏"], enabled: false },
      {
        id: 4,
        platform: "小红书",
        categories: ["游戏"],
        enabled: true,
        loginValid: false,
      },
      {
        id: 5,
        platform: "小红书",
        categories: ["游戏"],
        enabled: true,
        dailyCapacity: 0,
      },
      { id: 6, platform: "小红书", categories: ["游戏"], enabled: true },
      { id: 7, platform: "小红书", categories: ["游戏"], enabled: true },
    ],
    assets: [
      { id: 10, category: "美食" },
      { id: 11, category: "游戏", ready: false },
      { id: 12, category: "游戏", fileAvailable: false },
      { id: 13, category: "游戏" },
      { id: 14, category: "游戏" },
    ],
  };
}
it("defaults to a usable account matching both platform and category", () => {
  const db = fixture();
  expect(defaultPublishingResources(db, "小红书", "游戏").account.id).toBe(6);
  expect(defaultPublishingResources(db, "小红书", "游戏", 7).account.id).toBe(
    7,
  );
  expect(defaultPublishingResources(db, "小红书", "游戏", 1).account.id).toBe(
    6,
  );
});
it("defaults to a ready same-category asset without a same-platform reservation", () => {
  const db = fixture();
  db.tasks.push({ platform: "小红书", assetId: 13, status: "已取消" });
  expect(defaultPublishingResources(db, "小红书", "游戏").asset.id).toBe(14);
  db.tasks[0].status = "已放弃";
  expect(defaultPublishingResources(db, "小红书", "游戏").asset.id).toBe(13);
  db.tasks[0].platform = "B站";
  db.tasks[0].status = "执行中";
  expect(defaultPublishingResources(db, "小红书", "游戏").asset.id).toBe(13);
});
it("does not invent a selection without a matching account or free asset", () => {
  const db = fixture();
  expect(defaultPublishingResources(db, "小红书", "其他")).toEqual({
    account: null,
    asset: null,
  });
  expect(defaultPublishingResources(db, "", "游戏")).toEqual({
    account: null,
    asset: null,
  });
  db.assets = [];
  expect(defaultPublishingResources(db, "小红书", "游戏")).toMatchObject({
    account: { id: 6 },
    asset: null,
  });
});
