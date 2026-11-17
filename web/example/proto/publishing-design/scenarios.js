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

// Additive task fixtures. Explicitly fictional, independent accounts avoid cross-scenario blocking.
export function addTaskReviewScenario(db) {
  let seq = nextId(db) + 100;
  const statuses = [
    "待执行",
    "执行中",
    "等待人工",
    "结果待确认",
    "平台审核中",
    "执行失败",
    "已发布",
    "已取消",
  ];
  const tasks = statuses.map((status, index) => {
    const a = {
      id: seq++,
      title: `体验验收成片 ${index + 1}`,
      category: "三角洲行动",
      quality: "优质",
      ready: true,
      fileAvailable: true,
      cover: backgroundPicture(index),
      duration: "00:49",
    };
    const account = {
      id: seq++,
      name: `体验验收账号 ${index + 1}`,
      platform: "B站",
      categories: [a.category],
      enabled: true,
      loginValid: true,
      window: `演示 ${index + 1}`,
    };
    db.assets.push(a);
    db.accounts.push(account);
    const content = {
      platform: "B站",
      accountId: account.id,
      assetId: a.id,
      category: a.category,
      ...defaults("B站", a),
      title: `体验验收 · ${status === "执行失败" ? "确认失败" : status}`,
      declaration: "内容无需标注",
      version: 1,
    };
    const attempted = !["待执行", "已取消"].includes(status),
      now = new Date().toISOString();
    const t = {
      ...content,
      id: seq++,
      batchId: null,
      status,
      result:
        status === "结果待确认"
          ? "未知"
          : status === "平台审核中"
            ? "平台已接收"
            : status === "已发布"
              ? "已公开"
              : status === "执行失败"
                ? "确认未发布"
                : "未提交",
      attempts: attempted ? 1 : 0,
      claimed: ["执行中", "等待人工", "结果待确认"].includes(status),
      snapshot: clone(content),
      created: now,
      log: [],
      attemptHistory: [],
      evidence: "",
    };
    if (attempted) {
      t.attemptHistory = [
        {
          number: 1,
          version: 1,
          started: now,
          status,
          snapshot: clone(content),
          steps: [
            { time: now, status: "执行中", evidence: "本地演示：读取资源完成" },
          ],
        },
      ];
      t.log = [
        { time: now, text: "本地演示：账号与资源检查完成" },
        {
          time: now,
          text:
            status === "执行中" ? "上传视频，等待进度回传" : `进入${status}`,
        },
      ];
    }
    if (status === "等待人工")
      t.evidence = "本地演示：登录验证需要人工完成，上传现场已保留";
    if (["结果待确认", "平台审核中", "已发布"].includes(status))
      t.submittedAt = now;
    if (status === "结果待确认")
      t.evidence = "本地演示：点击提交后连接中断，尚未核实平台稿件";
    if (["平台审核中", "已发布", "执行失败"].includes(status))
      t.attemptHistory[0].ended = now;
    if (status === "平台审核中")
      t.evidence = "本地演示：稿件已接收，等待平台审核";
    if (status === "已发布") {
      t.publicAt = now;
      t.evidence = "本地演示：公开发布结果样例，未关联真实作品链接";
    }
    if (status === "执行失败") {
      t.failedStep = "上传视频";
      t.evidence = "本地演示：上传阶段明确失败，原执行已结束且确认未发布";
      t.attemptHistory[0].receipt = t.evidence;
      t.attemptHistory[0].errorCode = "DEMO_UPLOAD_FAILED";
    }
    if (status === "已取消")
      t.evidence = "本地演示：运营取消排期，保留当前平台的成片占用";
    return t;
  });
  db.tasks.push(...tasks);
  return tasks;
}
