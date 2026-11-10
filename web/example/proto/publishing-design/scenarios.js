// Additive, fictional fixtures for repeatable browser acceptance. Never clears user records.
import {
  nextId,
  confirmPlan,
  createPublication,
  clone,
} from "./relationships.js";
import { initCandidate } from "./content.js";
import { defaults, declarationOptions, backgroundPicture } from "./model.js";
export function addScenario(db, kind) {
  let seq = nextId(db) + 100;
  const category = "三角洲行动",
    platform = "百家号";
  const counts = kind === "30/28" ? [30, 1] : [10, 1];
  const accountIds = Array.from({ length: 3 }, (_, i) => {
    const id = seq++;
    db.accounts.push({
      id,
      name: "验收演示账号 " + (i + 1),
      platform,
      categories: [category, "火影忍者"],
      enabled: true,
      quality: i ? "普通" : "优质",
      window: "仅演示",
      dailyCapacity: null,
    });
    return id;
  });
  const n = kind === "30/28" ? 29 : 11;
  const assets = Array.from({ length: n }, (_, i) => {
    const a = {
      id: seq++,
      title: "验收演示成片 " + (i + 1),
      category: kind === "30/28" && i >= 20 && i < 28 ? "火影忍者" : category,
      quality: i % 3 ? "普通" : "优质",
      ready: true,
      fileAvailable: true,
      cover: backgroundPicture(i),
      duration: "01:20",
    };
    db.assets.push(a);
    return a;
  });
  const firstTargets =
    kind === "30/28"
      ? [
          { category, count: 20 },
          { category: "火影忍者", count: 10 },
        ]
      : [{ category, count: 10 }];
  const d = {
    name: "验收演示 · " + kind + " · " + new Date().toLocaleTimeString("zh-CN"),
    confirmKey: crypto.randomUUID(),
    platformOrder: [platform],
    targets: firstTargets.map((t) => ({
      ...t,
      platform,
      count: t.count + (t.category === category ? 1 : 0),
    })),
    splits: [
      { name: "百度 A · " + kind, platform, targets: firstTargets },
      { name: "百度 B · 下一批", platform, targets: [{ category, count: 1 }] },
    ],
  };
  const p = confirmPlan(db, d);
  const make = (b, selected, first = true) =>
    createPublication(db, {
      platform,
      name: b.name,
      batchId: b.id,
      planId: p.id,
      partial: kind === "30/28",
      rows: selected.map((a, i) => ({
        platform,
        assetId: a.id,
        category: a.category,
        accountId: accountIds[i % 3],
        ...defaults(platform, a),
        declaration: declarationOptions(platform)[0].value,
        demoOutcome:
          (first && kind === "10%" && i === 0) ||
          (first && kind === "20%" && i < 2)
            ? "执行失败"
            : first && kind === "未知" && i === 0
              ? "结果待确认"
              : "已发布",
      })),
    });
  if (kind === "30/28") {
    const b = db.batches.find((b) => b.id === p.items[0].id);
    b.candidates = assets.slice(0, -1).map((a, i) =>
      initCandidate({
        key: platform + ":" + a.id,
        platform,
        assetId: a.id,
        category: a.category,
        accountId: accountIds[i % 3],
        ...defaults(platform, a),
        declaration: declarationOptions(platform)[0].value,
      }),
    );
    b.phase = "待内容配置";
    b.draft = {
      step: 2,
      assets: assets.slice(0, -1).map((a) => a.id),
      accountOrder: accountIds,
      creationKey: crypto.randomUUID(),
    };
  } else
    make(
      db.batches.find((b) => b.id === p.items[0].id),
      assets.slice(0, -1),
    );
  make(
    db.batches.find((b) => b.id === p.items[1].id),
    assets.slice(-1),
    false,
  );
  return p;
}
