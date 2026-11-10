// Offline publishing prototype. These rules do not invoke platform services.
export const platforms = ["小红书", "B站", "百家号"];
export const states = [
  "待执行",
  "排队中",
  "执行中",
  "等待人工",
  "结果待确认",
  "平台审核中",
  "已发布",
  "执行失败",
  "已取消",
  "已放弃",
];
export const categories = ["三角洲行动", "火影忍者", "通用"];
export const threshold = 0.1;
export const occupied = (task) => task.status !== "已放弃";
export const canRetry = (task) =>
  task.status === "执行失败" && task.result === "确认未发布";
export function nextBatch(tasks) {
  if (
    tasks.some((t) =>
      ["待执行", "执行中", "排队中", "等待人工", "结果待确认"].includes(
        t.status,
      ),
    )
  )
    return "等待核实";
  const known = tasks.filter((t) =>
    ["已发布", "平台审核中", "执行失败"].includes(t.status),
  );
  if (!known.length) return "等待跳过确认";
  return known.filter(canRetry).length / known.length > threshold
    ? "等待继续确认"
    : "自动继续";
}
export function stats(tasks, target = 0) {
  const published = tasks.filter((t) => t.status === "已发布").length;
  return {
    total: tasks.length,
    published,
    reviewing: tasks.filter((t) => t.status === "平台审核中").length,
    failed: tasks.filter(canRetry).length,
    pending: tasks.filter((t) =>
      ["待执行", "排队中", "执行中"].includes(t.status),
    ).length,
    human: tasks.filter((t) => t.status === "等待人工").length,
    unknown: tasks.filter((t) => t.status === "结果待确认").length,
    finished: tasks.filter((t) =>
      ["已发布", "执行失败", "已取消", "已放弃"].includes(t.status),
    ).length,
    gap: Math.max(0, target - published),
    rate: target ? Math.round((published / target) * 100) : null,
  };
}
export function defaults(platform, asset) {
  return {
    title: asset?.title || "",
    description: platform === "百家号" ? asset?.title || "" : "",
    tags: platform === "B站" ? asset?.category || "" : "",
    partition: platform === "B站" ? "游戏" : "",
    declaration: "",
    visibility: "公开",
    schedule: "",
    watermark: false,
    mount: false,
    cover: asset?.cover || "",
    coverMode: "现成封面",
    templateId: null,
    templateDocument: null,
  };
}
export function validate(row, db, ignoreId) {
  const errors = [],
    a = db.accounts.find((a) => a.id === row.accountId),
    v = db.assets.find((a) => a.id === row.assetId);
  if (!a || !a.enabled || a.platform !== row.platform)
    errors.push("请选择可用的同平台账号");
  if (!v) errors.push("请选择成片");
  if (v?.ready === false)
    errors.push(v.unavailableReason || "成片尚未生成完成");
  if (v?.fileAvailable === false) errors.push("成片文件不可取得");
  if (a && v && !a.categories.includes(v.category))
    errors.push("账号与成片类目不匹配");
  if (
    v &&
    db.tasks.some(
      (t) =>
        t.id !== ignoreId &&
        t.platform === row.platform &&
        t.assetId === v.id &&
        occupied(t),
    )
  )
    errors.push("成片已被同平台任务占用");
  const title = row.platform === "百家号" ? row.description : row.title;
  const limit =
    row.platform === "小红书" ? 20 : row.platform === "B站" ? 80 : 50;
  if (!title?.trim())
    errors.push(row.platform === "百家号" ? "请填写作品描述" : "请填写标题");
  if (Array.from(title || "").length > limit)
    errors.push(`文案超过原型长度检查 ${limit}；百家号计数折算待核验`);
  if (!row.cover) errors.push("请选择封面");
  if (
    row.platform === "B站" &&
    row.tags?.split(/[,，、\s]+/).filter(Boolean).length > 10
  )
    errors.push("B站标签最多 10 个");
  if (!row.declaration) errors.push("请按内容确认创作声明");
  else if (
    !declarationOptions(row.platform).some(
      (option) => option.value === row.declaration,
    )
  )
    errors.push("请重新确认当前平台的创作声明");
  if (
    row.platform !== "百家号" &&
    Array.from(row.description || "").length >
      (row.platform === "小红书" ? 1000 : 2000)
  )
    errors.push("正文／简介超过平台长度限制");
  if (
    row.platform === "百家号" &&
    row.tags?.split(/[,，、\s]+/).filter(Boolean).length > 10
  )
    errors.push("百家号话题最多 10 个");
  if (row.platform === "B站" && (!row.tags?.trim() || !row.partition))
    errors.push("请补齐分区和标签");
  if (
    row.schedule &&
    (!Number.isFinite(Date.parse(row.schedule)) ||
      Date.parse(row.schedule) <= Date.now())
  )
    errors.push("发布时间应晚于当前时间");
  return errors;
}
export function picture(n, text = "游戏时刻") {
  return (
    "data:image/svg+xml;charset=utf-8," +
    encodeURIComponent(
      `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="900"><defs><linearGradient id="g" x2="1" y2="1"><stop stop-color="${["#152e4d", "#263359", "#0c4c4e"][n % 3]}"/><stop offset="1" stop-color="${["#538d98", "#8a6b98", "#4e9082"][n % 3]}"/></linearGradient></defs><path fill="url(#g)" d="M0 0h1200v900H0z"/><circle cx="1000" cy="80" r="360" fill="#ffffff" opacity=".08"/><path d="M0 700L460 220l740 680H0" fill="#0b1c35" opacity=".5"/><text x="80" y="700" fill="white" font-family="sans-serif" font-size="70" font-weight="bold">${text}</text><text x="85" y="770" fill="#c5d8e8" font-family="sans-serif" font-size="24">WT MEDIA · CONTENT STUDIO</text></svg>`,
    )
  );
}
export function seed() {
  const assets = Array.from({ length: 12 }, (_, i) => ({
    id: 128 + i,
    title:
      [
        "三角洲实战精彩片段",
        "火影忍者连招教学",
        "三角洲新手进阶指南",
        "火影忍者对决时刻",
      ][i % 4] + (i > 3 ? " " + (i + 1) : ""),
    category: i % 2 ? "火影忍者" : "三角洲行动",
    quality: i % 3 ? "普通" : "优质",
    duration: "00:49",
    cover: picture(i, ["实战记录", "连招教学", "新手进阶"][i % 3]),
  }));
  const accounts = platforms.flatMap((platform, p) =>
    Array.from({ length: 3 }, (_, i) => ({
      id: p * 10 + i + 1,
      name: [
        ["战术笔记", "忍者日记", "游戏记录"],
        ["游戏魔王嘟嘟", "实战研究所", "每日游戏"],
        ["游戏主播小美", "游戏档案", "实战频道"],
      ][p][i],
      platform,
      quality: i === 0 ? "优质" : i === 1 ? "普通" : "新号",
      categories: i === 2 ? categories.slice(0, 2) : [categories[i % 2]],
      enabled: i !== 2,
      window: 41 + p * 10 + i,
    })),
  );
  const tasks = states.map((status, i) => ({
    id: 1001 + i,
    platform: platforms[i % 3],
    accountId: accounts.filter((a) => a.platform === platforms[i % 3])[0].id,
    assetId: 128 + (i % 6) * 2,
    category: "三角洲行动",
    ...defaults(platforms[i % 3], assets[(i % 6) * 2]),
    declaration: "内容无需标注",
    status,
    result:
      status === "已发布"
        ? "已公开"
        : status === "平台审核中"
          ? "平台已接收"
          : status === "执行失败"
            ? "确认未发布"
            : status === "结果待确认"
              ? "未知"
              : "未提交",
    batchId: 201 + Math.floor(i / 4),
    attempts: ["待执行", "已取消", "已放弃"].includes(status) ? 0 : 1,
    log: [],
    evidence: "",
    created: "2026-11-10 10:30",
  }));
  const batches = [
    {
      id: 201,
      name: "小红书 · 首轮内容",
      platform: "小红书",
      control: "待执行",
    },
    { id: 202, name: "B站 · 游戏精选", platform: "B站", control: "已暂停" },
    {
      id: 203,
      name: "百家号 · 日常发布",
      platform: "百家号",
      control: "待执行",
    },
  ];
  // Keep each sample batch homogeneous; states remain varied for review.
  tasks.forEach((t) => {
    const b = batches.find((b) => b.id === t.batchId);
    t.platform = b.platform;
    t.accountId = accounts.find(
      (a) =>
        a.platform === b.platform &&
        a.categories.includes(t.category) &&
        a.enabled,
    ).id;
    t.tags = b.platform === "B站" ? "三角洲行动" : "";
    t.partition = b.platform === "B站" ? "游戏" : "";
    t.declaration = declarationOptions(b.platform)[0].value;
  });
  return {
    assets: [
      ...assets,
      {
        ...assets[0],
        id: 140,
        title: "三角洲新战术 · 生成中",
        ready: false,
        unavailableReason: "成片仍在生成",
      },
      {
        ...assets[1],
        id: 141,
        title: "火影对决 · 文件待恢复",
        fileAvailable: false,
      },
    ],
    accounts,
    tasks,
    batches,
    plans: [
      {
        id: 301,
        name: "游戏内容日常运营",
        control: "规划中",
        targets: [
          { platform: "小红书", category: "三角洲行动", count: 8 },
          { platform: "B站", category: "三角洲行动", count: 6 },
          { platform: "百家号", category: "三角洲行动", count: 4 },
        ],
        items: [201, 202, 203],
        skipped: [],
        position: 0,
      },
    ],
    images: assets.slice(0, 8).map((a, i) => ({
      id: 401 + i,
      name: a.title,
      category: a.category,
      uses: ["现成封面", "底图"],
      enabled: true,
      src: a.cover,
    })),
    templates: [],
  };
}

// Inventory eligibility is independent of editable publishing content.
export function resourceStatus(db, asset, platform, accountIds = null) {
  if (asset.ready === false)
    return asset.unavailableReason || "成片尚未生成完成";
  if (asset.fileAvailable === false) return "成片文件不可取得";
  if (
    db.tasks.some(
      (t) => t.platform === platform && t.assetId === asset.id && occupied(t),
    )
  )
    return "同平台已占用";
  const candidates = db.accounts.filter(
    (a) =>
      a.enabled &&
      a.platform === platform &&
      a.categories.includes(asset.category) &&
      (!accountIds || accountIds.includes(a.id)),
  );
  return candidates.length ? "" : "没有匹配账号";
}
export function resourceInventory(
  db,
  selectedPlatforms,
  selectedCategories,
  accountIds = null,
) {
  return selectedPlatforms.flatMap((platform) =>
    selectedCategories.map((category) => {
      const assets = db.assets.filter((v) => v.category === category);
      const accounts = db.accounts.filter(
        (a) =>
          a.enabled &&
          a.platform === platform &&
          a.categories.includes(category) &&
          (!accountIds || accountIds.includes(a.id)),
      );
      return {
        key: `${platform}:${category}`,
        platform,
        category,
        accounts: accounts.length,
        available: assets.filter(
          (v) => !resourceStatus(db, v, platform, accountIds),
        ).length,
        occupied: assets.filter((v) =>
          db.tasks.some(
            (t) => t.platform === platform && t.assetId === v.id && occupied(t),
          ),
        ).length,
        unavailable: assets.filter(
          (v) =>
            resourceStatus(db, v, platform, accountIds) &&
            !db.tasks.some(
              (t) =>
                t.platform === platform && t.assetId === v.id && occupied(t),
            ),
        ).length,
      };
    }),
  );
}
export function suggestDistribution(
  db,
  selectedPlatforms,
  assetIds,
  accountIds,
  previous = [],
  limits = {},
) {
  const load = new Map();
  const scarcity = (assetId) => {
    const video = db.assets.find((v) => v.id === assetId);
    return video
      ? db.accounts.filter(
          (a) =>
            accountIds.includes(a.id) &&
            a.enabled &&
            a.categories.includes(video.category),
        ).length
      : Infinity;
  };
  const orderedAssets = [...assetIds].sort((a, b) => scarcity(a) - scarcity(b));
  return selectedPlatforms.flatMap((platform) =>
    orderedAssets.flatMap((assetId) => {
      const video = db.assets.find((v) => v.id === assetId);
      if (!video || resourceStatus(db, video, platform, accountIds)) return [];
      const hasCapacity = (accountId) =>
        !Number(limits[accountId]) ||
        (load.get(accountId) || 0) < Number(limits[accountId]);
      const prior = previous.find(
        (r) =>
          r.platform === platform &&
          r.assetId === assetId &&
          accountIds.includes(r.accountId) &&
          hasCapacity(r.accountId),
      );
      const candidates = db.accounts.filter(
        (a) =>
          accountIds.includes(a.id) &&
          a.platform === platform &&
          a.categories.includes(video.category) &&
          a.enabled &&
          hasCapacity(a.id),
      );
      if (!candidates.length) return [];
      const qualityOrder =
        video.quality === "优质"
          ? ["优质", "普通", "新号"]
          : ["普通", "新号", "优质"];
      candidates.sort(
        (a, b) =>
          qualityOrder.indexOf(a.quality) - qualityOrder.indexOf(b.quality) ||
          accountIds.indexOf(a.id) - accountIds.indexOf(b.id),
      );
      const chosen = prior
        ? db.accounts.find((a) => a.id === prior.accountId)
        : candidates[0];
      load.set(chosen.id, (load.get(chosen.id) || 0) + 1);
      return prior
        ? [prior]
        : [
            {
              key: `${platform}:${assetId}`,
              platform,
              category: video.category,
              assetId,
              accountId: chosen.id,
              ...defaults(platform, video),
              overrides: [],
              modified: false,
            },
          ];
    }),
  );
}
export function mergeContentDefaults(
  rows,
  platform,
  values,
  overwrite = false,
) {
  return rows.map((row) => {
    if (row.platform !== platform) return row;
    const next = { ...row, overrides: [...(row.overrides || [])] };
    for (const [key, value] of Object.entries(values)) {
      if (!overwrite && next.overrides.includes(key)) continue;
      next[key] = value;
      if (overwrite) next.overrides = next.overrides.filter((k) => k !== key);
    }
    return next;
  });
}
export function declarationOptions(platform) {
  return (
    platform === "B站"
      ? [
          "内容无需标注",
          "含AI生成内容",
          "含虚构演绎内容",
          "内容含营销信息",
          "个人观点，仅供参考",
          "内容为转载",
        ]
      : platform === "小红书"
        ? [
            "不添加特殊声明（原型设置）",
            "虚构演绎仅供娱乐",
            "笔记含AI合成内容",
            "内容包含营销广告",
          ]
        : ["无需声明"]
  ).map((value) => ({ label: value, value }));
}
