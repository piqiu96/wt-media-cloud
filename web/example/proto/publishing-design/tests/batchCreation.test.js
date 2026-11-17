import { describe, it, expect } from "vitest";
import { seed, suggestDistribution } from "../model.js";
describe("batch resource matching", () => {
  it("keeps specialist accounts for their category when a shared account is listed first", () => {
    const db = seed();
    db.tasks = [];
    db.accounts = [
      {
        id: 1,
        enabled: true,
        platform: "小红书",
        quality: "普通",
        categories: ["三角洲行动", "火影忍者"],
      },
      {
        id: 2,
        enabled: true,
        platform: "小红书",
        quality: "普通",
        categories: ["三角洲行动"],
      },
    ];
    const rows = suggestDistribution(
      db,
      ["小红书"],
      [128, 129],
      [1, 2],
      [],
      { 1: 1, 2: 1 },
      ["三角洲行动", "火影忍者"],
    );
    expect(rows).toHaveLength(2);
    expect(rows.find((r) => r.assetId === 128).accountId).toBe(2);
    expect(rows.find((r) => r.assetId === 129).accountId).toBe(1);
  });
  it("uses category priority for shared limited capacity", () => {
    const db = seed();
    db.tasks = [];
    db.accounts = [
      {
        id: 1,
        enabled: true,
        platform: "小红书",
        quality: "优质",
        categories: ["三角洲行动", "火影忍者"],
      },
    ];
    const rows = suggestDistribution(
      db,
      ["小红书"],
      [128, 129],
      [1],
      [],
      { 1: 1 },
      ["火影忍者", "三角洲行动"],
    );
    expect(rows.map((r) => r.assetId)).toEqual([129]);
  });
});

import { createSSRApp, h } from "vue";
import { renderToString } from "@vue/server-renderer";
import PublishComposer from "../PublishComposer.vue";
import BatchContentWorkspace from "../BatchContentWorkspace.vue";
function appFor(component, props) {
  const app = createSSRApp(component, props);
  for (const name of [
    "t-input-number",
    "t-tabs",
    "t-tab-panel",
    "t-tooltip",
    "t-drawer",
    "t-dialog",
    "t-tag",
    "t-button",
    "t-popup",
    "t-checkbox",
    "t-checkbox-group",
    "t-input",
    "t-alert",
    "t-select",
    "t-textarea",
    "t-radio",
    "t-radio-group",
    "t-radio-button",
  ])
    app.component(name, {
      props: ["visible"],
      setup:
        (props, { slots }) =>
        () =>
          ["t-drawer", "t-dialog"].includes(name) && !props.visible
            ? null
            : h("div", slots.default?.()),
    });
  app.component("t-table", {
    props: ["data", "columns"],
    setup(props, { slots }) {
      return () => h("table", [
        h("thead", [h("tr", props.columns.map(column => h("th", column.title)))]),
        h("tbody", props.data.map(row => h("tr",
          props.columns.map(column => h("td", slots[column.colKey]?.({ row }))),
        ))),
      ]);
    },
  });
  return app;
}
it("opens the creation component with an empty or resumed draft without initialization errors", async () => {
  const db = seed();
  await expect(
    renderToString(appFor(PublishComposer, { db })),
  ).resolves.toBeTypeOf("string");
  db.publishDraft = {
    active: true,
    step: 2,
    platforms: ["B站"],
    categories: ["火影忍者"],
    rows: [],
    targets: [{ category: "火影忍者", count: 1 }],
  };
  await expect(
    renderToString(appFor(PublishComposer, { db })),
  ).resolves.toBeTypeOf("string");
});
it("shows adopted content and a task selector in the content workspace before invoking AI", async () => {
  const db = seed();
  db.tasks = [];
  const rows = suggestDistribution(db, ["B站"], [129], [12]);
  rows[0].title = "人工已确认标题";
  const html = await renderToString(
    appFor(BatchContentWorkspace, { db, rows }),
  );
  expect(html).toContain("人工已确认标题");
  expect(html).toContain("发布账号");
  expect(html).toContain("发布成片");
  expect(html).not.toContain("batch-content-selection");
  expect(html).toContain("配置状态");
  expect(html).toContain("待完善");
  expect(html).toContain("未选择创作声明");
  expect(html).not.toContain("CONTENT GENERATION");
});

describe('category account selection', () => {
  function scopedDb() {
    const db=seed();db.tasks=[];
    db.accounts=[{id:1,enabled:true,platform:'小红书',quality:'普通',categories:['三角洲行动','火影忍者']},{id:2,enabled:true,platform:'小红书',quality:'普通',categories:['三角洲行动']}];
    return db;
  }
  it('matches only accounts selected for each category',()=>{
    const rows=suggestDistribution(scopedDb(),['小红书'],[128,129],[1,2],[],{},['三角洲行动','火影忍者'],{'三角洲行动':[2],'火影忍者':[1]});
    expect(rows.find(r=>r.assetId===128).accountId).toBe(2);
    expect(rows.find(r=>r.assetId===129).accountId).toBe(1);
  });
  it('an explicitly empty category pool cannot borrow accounts from another category',()=>{
    const rows=suggestDistribution(scopedDb(),['小红书'],[128,129],[1],[],{},[],{'三角洲行动':[],'火影忍者':[1]});
    expect(rows.map(r=>r.assetId)).toEqual([129]);
  });
  it('shares one account limit across category selections',()=>{
    const rows=suggestDistribution(scopedDb(),['小红书'],[128,129],[1],[],{1:1},['三角洲行动','火影忍者'],{'三角洲行动':[1],'火影忍者':[1]});
    expect(rows).toHaveLength(1);
  });
});
