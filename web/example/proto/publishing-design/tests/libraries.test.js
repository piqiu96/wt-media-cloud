import { it, expect } from "vitest";
import {
  defaultDocument,
  validateDocument,
  coverSignature,
} from "../covers.js";
import {
  imageReferences,
  deleteImage,
  templateReferences,
  chooseBackground,
} from "../library.js";
import { seed } from "../model.js";
import { platformInventory } from "../inventory.js";
it("D14 rejects missing image, unsafe geometry and unsupported font before export", () => {
  const d = defaultDocument();
  expect(() => validateDocument(d)).toThrow("底图");
  d.image = "data:image/png;base64,test";
  expect(validateDocument(d)).toBe(true);
  d.layers[0].x = 0;
  expect(() => validateDocument(d)).toThrow("安全区");
  d.layers[0].x = 96;
  d.layers[0].fontFamily = "missing-test-font";
  expect(() => validateDocument(d)).toThrow("字体缺失");
});
it("D14 alternate ratio document/font changes invalidate cover signatures", () => {
  const r = { templateDocuments: { "16:9": defaultDocument(1600, 900, "a") } };
  const old = coverSignature(r);
  r.templateDocuments["16:9"].layers[0].fontFamily = "serif";
  expect(coverSignature(r)).not.toBe(old);
});
it("D14 image references include template snapshots and alternate ratio snapshots", () => {
  const db = seed(),
    im = db.images[0];
  db.templates = [];
  db.tasks = [
    {
      id: 1,
      title: "历史",
      templateDocuments: { "16:9": { image: im.src } },
      templateSnapshots: { "16:9": { id: 8, version: 1 } },
    },
  ];
  expect(imageReferences(db, im)).toHaveLength(1);
  expect(templateReferences(db, { id: 8 })).toHaveLength(1);
  expect(() => deleteImage(db, im)).toThrow("只能停用");
  expect(db.images).toContain(im);
});
it("random background chooses game first, enabled generic fallback, then repair", () => {
  const db = {
    images: [
      { id: 1, enabled: true, category: "通用", uses: ["底图"] },
      {
        id: 2,
        enabled: true,
        category: "三角洲行动",
        uses: ["现成封面", "底图"],
      },
    ],
  };
  expect(chooseBackground(db, "三角洲行动", () => 0).id).toBe(2);
  db.images[1].enabled = false;
  expect(chooseBackground(db, "三角洲行动", () => 0).id).toBe(1);
  db.images[0].enabled = false;
  expect(chooseBackground(db, "三角洲行动")).toBeNull();
});
it("D01 shared account capacity is deducted and counted once across categories", () => {
  const db = seed();
  db.accounts = db.accounts
    .filter((a) => a.platform === "百家号" && a.enabled)
    .slice(0, 1);
  db.accounts[0].dailyCapacity = 6;
  db.accounts[0].categories = ["三角洲行动", "火影忍者"];
  db.tasks = [
    {
      platform: "百家号",
      accountId: db.accounts[0].id,
      status: "平台审核中",
      schedule: "2026-11-10T10:00:00+08:00",
    },
  ];
  const r = platformInventory(db, "2026-11-10T01:00:00Z").find(
    (r) => r.platform === "百家号",
  );
  expect(r.capacity).toBe(5);
  expect(r.published).toBe(0);
});

it("D01 category estimates respect eligibility while joint capacity deduplicates shared quota", () => {
  const db = seed();
  db.tasks = [];
  db.assets = [1, 2, 3, 4].flatMap((id) => [
    { id, category: "三角洲行动" },
    { id: id + 10, category: "火影忍者" },
  ]);
  db.accounts = [
    {
      id: 1,
      enabled: true,
      platform: "百家号",
      dailyCapacity: 3,
      categories: ["三角洲行动", "火影忍者"],
    },
    {
      id: 2,
      enabled: true,
      loginValid: false,
      platform: "百家号",
      dailyCapacity: 20,
      categories: ["三角洲行动"],
    },
  ];
  const r = platformInventory(db).find((r) => r.platform === "百家号");
  expect(r.available).toBe(1);
  expect(r.invalid).toBe(1);
  expect(r.byCategory.map((c) => c.estimate)).toEqual([3, 3]);
  expect(r.estimate).toBe(3);
  db.accounts[0].categories = ["不存在类目"];
  expect(
    platformInventory(db).find((r) => r.platform === "百家号").estimate,
  ).toBe(0);
});
