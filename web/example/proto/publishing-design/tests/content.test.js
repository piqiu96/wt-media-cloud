import { it, expect } from "vitest";
import {
  requestSuggestion,
  receiveSuggestion,
  adopt,
  editField,
  demoValues,
  initCandidate,
} from "../content.js";
import { coverSignature } from "../covers.js";
import { platformInventory, businessDate } from "../inventory.js";
import { seed } from "../model.js";
it("D13 one response contains title, short title, topics, tags; explicit adoption", () => {
  const r = initCandidate({ assetId: 128, category: "三角洲行动" }),
    req = requestSuggestion(r);
  receiveSuggestion(r, req, demoValues(r));
  expect(r.title).toBeUndefined();
  for (const f of ["title", "shortTitle", "topics", "tags"])
    expect(adopt(r, f)).toBe(true);
  expect(r.shortTitle).not.toBe(r.title);
});
it("D13 manual edits during generation invalidate only changed field", () => {
  const r = initCandidate({ title: "原稿" }),
    req = requestSuggestion(r);
  editField(r, "title", "人工");
  receiveSuggestion(r, req, {
    title: "迟到",
    shortTitle: "短标题",
    topics: "#游戏",
    tags: "游戏",
  });
  expect(adopt(r, "title")).toBe(false);
  expect(r.title).toBe("人工");
  expect(adopt(r, "shortTitle")).toBe(true);
});
it("D13 out-of-order response cannot supersede a newer request", () => {
  const r = initCandidate({}),
    old = requestSuggestion(r, ["title"]),
    newer = requestSuggestion(r, ["title"]);
  receiveSuggestion(r, newer, { title: "新" });
  receiveSuggestion(r, old, { title: "旧" });
  adopt(r, "title");
  expect(r.title).toBe("新");
});
it("D13 changing input invalidates old results, single-field regeneration preserves others", () => {
  const r = initCandidate({ assetId: 1, tags: "已采用" }),
    req = requestSuggestion(r);
  r.assetId = 2;
  receiveSuggestion(r, req, { title: "旧" });
  expect(adopt(r, "title")).toBe(false);
  const one = requestSuggestion(r, ["title"]);
  receiveSuggestion(r, one, { title: "新" });
  adopt(r, "title");
  expect(r.tags).toBe("已采用");
});
it("D14 changing short title invalidates actual preview and approval", () => {
  const r = initCandidate({
    shortTitle: "旧",
    coverApproved: true,
    coverVariants: { source: "old" },
    templateDocument: { layers: [] },
  });
  const sig = coverSignature(r);
  editField(r, "shortTitle", "新");
  expect(coverSignature(r)).not.toBe(sig);
  expect(r.coverApproved).toBe(false);
  expect(r.coverVariants).toEqual({});
});
it("repeating bulk adoption keeps adopted fields and their review state stable", () => {
  const r = initCandidate({}),
    req = requestSuggestion(r, ["title"]);
  receiveSuggestion(r, req, { title: "已采用标题" });
  expect(adopt(r, "title")).toBe(true);
  const version = r.fieldVersions.title;
  r.contentApproved = true;
  expect(adopt(r, "title")).toBe(false);
  expect(r.ai.suggestions.title.status).toBe("已采用");
  expect(r.fieldVersions.title).toBe(version);
  expect(r.contentApproved).toBe(true);
});
it("D01 Shanghai publication day uses public timestamp, unknown shared capacity stays unknown", () => {
  const db = seed();
  db.tasks = [
    {
      platform: "B站",
      accountId: 11,
      status: "已发布",
      created: "2026-11-08",
      publicAt: "2026-11-09T16:30:00Z",
    },
  ];
  expect(businessDate(db.tasks[0].publicAt)).toBe("2026-11-10");
  const inv = platformInventory(db, "2026-11-10T01:00:00Z").find(
    (r) => r.platform === "B站",
  );
  expect(inv.published).toBe(1);
  expect(inv.publishAccounts).toBe(1);
  expect(inv.capacity).toBeNull();
});
