// Additive, fictional fixtures for repeatable browser acceptance. Never clears user records.
import {
  nextId,
  confirmPlan,
  dispatchPlan,
  pausePlan,
  syncPlan,
  createPublication,
  clone,
  claimTask,
  setResult,
} from "./relationships.js";
import { rowsFor, runBatchCommand, startReviewScope } from "./batchLifecycle.js";
import { reconcileExecutionGroup } from "./batchExecutionSettings.js";
import { preparationRecord } from "./preparationPreview.js";
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

// CHG108 review rows are fictional and exercise the same batch commands as the preview.
export function addExecutionReviewScenarios(db) {
  const platform = "百家号";
  const category = "三角洲行动";
  const sourceAsset = db.assets.find(asset => asset.category === category && asset.cover);
  if (!sourceAsset) return;
  const definitions = [
    { key: "start", name: "启动检查 · 保留目标缺口", target: 4, count: 3 },
    { key: "operator", name: "普通运营 · 已保存未开始", owner: "lin", count: 1 },
    { key: "running", name: "分组执行 · 并发 5、可用 Profile 1", count: 3 },
    { key: "interval", name: "组间等待 · 审核尚未公开", count: 3 },
    { key: "paused", name: "异常暂停 · 保留人工现场", count: 3, groupPolicy: "异常暂停" },
    { key: "finish", name: "明确失败 · 可结束批次", count: 1 },
    { key: "stopped", name: "终止收尾 · 原结果尚未知", count: 3 },
    { key: "public", name: "可信公开 · 30 目标／28 任务／24 公开", count: 28, target: 30 },
  ];
  let next = nextId(db);
  for (const scenario of definitions) {
    if (db.batches.some(batch => batch.executionReview === true && batch.previewSample === scenario.key)) continue;
    const actor = scenario.owner === "lin" ? { id: "lin", name: "林予安" } : { id: "chen", name: "陈小雨" };
    const batchId = next++;
    const accounts = Array.from({ length: 2 }, (_, index) => ({
      id: next++, name: `演示发布账号 ${index + 1}`, platform, categories: [category],
      ownerId: actor.id, enabled: true, loginValid: true, platformUid: `uid-${batchId}-${index}`,
      profileId: `profile-${batchId}-${scenario.key === "running" ? 0 : index}`,
    }));
    db.accounts.push(...accounts);
    const rows = Array.from({ length: scenario.count }, (_, index) => {
      const asset = { ...sourceAsset, id: next++, title: `演示成片 ${index + 1}`, category, ready: true,
        fileAvailable: !(scenario.key === "start" && index === 2) };
      db.assets.push(asset);
      return initCandidate({
        ...defaults(platform, asset), key: `preview-${batchId}-${index + 1}`,
        accountId: accounts[(scenario.key === "paused" ? 0 : index) % accounts.length].id,
        assetId: asset.id, category, platform, description: `演示发布文案 ${index + 1}`,
        declaration: declarationOptions(platform)[0].value,
        contentApproved: true, coverApproved: true,
      });
    });
    const batch = preparationRecord({
      name: scenario.name, platforms: [platform], categories: [category],
      targets: [{ category, count: scenario.target || scenario.count }], rows, step: 3,
      executionSettings: { concurrency: 5, groupInterval: 120, groupPolicy: scenario.groupPolicy || "自动继续", exceptionLimit: 3 },
    }, { id: batchId, actor, executionReview: true });
    batch.previewSample = scenario.key;
    db.batches.push(batch);
    if (["start", "operator"].includes(scenario.key)) continue;
    const scope = startReviewScope(db, batch);
    runBatchCommand(db, batch, "execute", { scope: scope.signature, partial: !!scenario.target });
    next = nextId(db);
    const tasks = rowsFor(db, batch);
    const first = tasks[0];
    claimTask(db, first);
    if (scenario.key === "running") continue;
    if (scenario.key === "interval") {
      setResult(first, "平台审核中", { evidence: "演示平台已接收，尚未公开" });
      claimTask(db, tasks[1]);
      setResult(tasks[1], "平台审核中", { evidence: "演示平台已接收，尚未公开" });
    } else if (scenario.key === "paused") {
      setResult(first, "等待人工", { evidence: "演示原账号验证，保留原浏览器现场" });
      first.suspendedAt = new Date().toISOString();
    } else if (scenario.key === "stopped") {
      setResult(first, "结果待确认", { evidence: "演示提交响应丢失，须核实原稿件" });
      runBatchCommand(db, batch, "stop", { reason: "演示人工终止后续发布" });
    } else if (scenario.key === "finish") {
      setResult(first, "执行失败", { evidence: "演示已明确拒绝，原尝试安全结束且未发布" });
    } else {
      // Fixed evidence fixtures demonstrate the read-back projection only.
      for (const [index, task] of tasks.entries()) {
        task.executionGroup = Math.floor(index / 2) + 1;
        if (!task.attemptHistory.length)
          task.attemptHistory.push({ number: 1, started: new Date().toISOString(), snapshot: task.snapshot });
        task.attempts = 1;
        if (index < 24) {
          const workId = next++;
          db.works ||= [];
          db.works.push({ id: workId, platform, platformId: `demo-${workId}`, sourceTaskIds: [task.id] });
          Object.assign(task, { workId, platformId: `demo-${workId}` });
          setResult(task, "已发布", { evidence: "演示原尝试已关联匿名可播放作品" });
        } else setResult(task, "平台审核中", { evidence: "演示平台已接收，尚未公开" });
      }
      batch.executionGroup = null;
      batch.endedAt = new Date().toISOString();
    }
    reconcileExecutionGroup(db, batch, Date.now(), { dispatch: false });
  }
}

// CHG109 uses the existing plan and batch commands. Only these fixture outcomes are simulated.
export function addPlanReviewScenarios(db) {
  const platform = "百家号";
  const category = "三角洲行动";
  const sourceAsset = db.assets.find(asset => asset.category === category && asset.cover);
  if (!sourceAsset) return;
  const definitions = [
    { id: "PREPARE", name: "首启前 · 尚有批次待配置" },
    { id: "READY", name: "首启前 · 全部批次已配置" },
    { id: "DUPLICATE", name: "首启前 · 同平台成片重复" },
    { id: "10", name: "失败率 10% · 已自动进入下一批", results: [...Array(9).fill("已发布"), "执行失败"] },
    { id: "20", name: "失败率 20% · 等待确认下一批", results: [...Array(8).fill("已发布"), "执行失败", "执行失败"] },
    { id: "ZERO", name: "零分母 · 等待确认跳过", results: ["已取消"] },
    { id: "PENDING", name: "人工现场与结果未知 · 暂缓衔接", results: ["等待人工", "结果待确认"] },
    { id: "PAUSED", name: "人工暂停 · 当前批次保留", results: ["已发布"], paused: true },
    { id: "CHECK", name: "自动核验已停止 · 待人工核实", results: ["平台审核中"], checkStopped: true },
    { id: "PUBLIC", name: "平台已接收 · 计入衔接、未公开", results: ["平台审核中"], finishNext: true },
    { id: "FINISH", name: "明确未公开 · 按缺口确认结束", results: ["执行失败"], final: true },
    { id: "OPERATOR", name: "普通运营 · 本人已配置计划", owner: "lin" },
  ];
  for (const scenario of definitions) {
    const id = "PL-109-" + scenario.id;
    if (db.plans.some(plan => plan.id === id)) continue;
    let sequence = nextId(db);
    const owner = scenario.owner === "lin" ? { id: "lin", name: "林予安" } : { id: "chen", name: "陈小雨" };
    const counts = [scenario.results?.length || 1, ...(scenario.final ? [] : [1])];
    const plan = confirmPlan(db, {
      id, name: scenario.name, confirmKey: id, platformOrder: [platform],
      targets: [{ platform, category, count: counts.reduce((left, right) => left + right) }],
      splits: counts.map((count, index) => ({ id: id + "-B" + (index + 1),
        name: index ? "批次 02 · 下一批" : "批次 01 · 首批", platform, targets: [{ category, count }] })),
    });
    const time = new Date().toISOString();
    Object.assign(plan, { planReview: true, ownerId: owner.id, ownerName: owner.name, createdBy: owner.id,
      created: time, updated: time, dateStart: time.slice(0, 10), dateEnd: time.slice(0, 10), note: "原型演示数据" });
    const account = { id: sequence++, name: "演示百家号", platform, categories: [category], ownerId: owner.id,
      enabled: true, loginValid: true, platformUid: "demo-uid-" + id, profileId: "demo-profile-" + id };
    db.accounts.push(account);
    let firstAsset;
    for (const [index, item] of plan.items.entries()) {
      const batch = db.batches.find(row => row.id === item.id);
      Object.assign(batch, { planReview: true, preparationOnly: true, executionReview: true, ownerId: owner.id, created: time,
        executionSettings: { concurrency: 5, groupInterval: 0, groupPolicy: "自动继续", exceptionLimit: 5 } });
      if (scenario.id === "PREPARE" && index === 1) continue;
      batch.phase = "内容预检通过";
      batch.candidates = Array.from({ length: counts[index] }, (_, rowIndex) => {
        const asset = scenario.id === "DUPLICATE" && index ? firstAsset : { ...sourceAsset, id: sequence++, category,
          ready: true, fileAvailable: true, title: `演示成片 ${index + 1}-${rowIndex + 1}` };
        if (!index && !rowIndex) firstAsset = asset;
        if (!db.assets.some(row => row.id === asset.id)) db.assets.push(asset);
        return initCandidate({ ...defaults(platform, asset, db), platform, category, accountId: account.id, assetId: asset.id,
          key: `${item.id}-row-${rowIndex + 1}`, declaration: declarationOptions(platform)[0].value,
          contentApproved: true, coverApproved: true });
      });
    }
    if (!scenario.results) continue;
    dispatchPlan(db, plan);
    if (scenario.paused) pausePlan(plan);
    const batch = db.batches.find(row => row.id === plan.items[0].id);
    for (const [index, task] of rowsFor(db, batch).entries()) {
      const status = scenario.results[index];
      task.attempts = 1;
      task.attemptHistory = [{ number: 1, started: time, snapshot: task.snapshot }];
      if (status === "已发布") {
        const workId = nextId(db);
        db.works ||= [];
        db.works.push({ id: workId, platform, platformId: "demo-" + workId, sourceTaskIds: [task.id] });
        Object.assign(task, { workId, platformId: "demo-" + workId });
      }
      if (status === "执行失败" || scenario.checkStopped)
        task.executorEvidence = { attempt: 1, confirmedUnpublished: true };
      if (status === "等待人工") task.suspendedAt = time;
      if (scenario.checkStopped) task.autoCheckStoppedAt = time;
      setResult(task, status, { evidence: status === "已发布" ? "演示：原尝试已确认公开作品"
        : status === "平台审核中" ? "演示：平台接收回执，尚未公开" : "演示：" + status });
    }
    reconcileExecutionGroup(db, batch, Date.now(), { dispatch: false });
    syncPlan(db, plan);
    if (scenario.finishNext) {
      const nextBatch = db.batches.find(row => row.id === plan.items[1].id);
      for (const task of rowsFor(db, nextBatch)) setResult(task, "执行失败", { evidence: "演示：明确未公开" });
      reconcileExecutionGroup(db, nextBatch, Date.now(), { dispatch: false });
      syncPlan(db, plan);
    }
  }
}
