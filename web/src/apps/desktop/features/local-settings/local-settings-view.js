// What the 本机设置 page derives from the answers it was given.
//
// Pure, and separate from the `.vue` file for the same reason
// `local-agent-status.js` is: this is the part with rules in it, and a rule that
// lives in a component cannot be asserted without mounting one. `vitest` imports
// these functions directly; the page is only the rendering of what they return.
//
// ## The one rule that decides most of this file
//
// **A number that could not be measured is not shown as zero.** The Rust side
// already refuses to answer with a `0` it did not measure (AC-06), so what is
// left here is the opposite mistake: rendering a *real* zero as if it were a
// fact of the same kind as a non-zero. 「没有需要清理的文件」and 「已释放 4 KB」are
// different sentences because they are different outcomes, and the report is
// what tells them apart — see `describeCleanup`.

/** Bytes as a person reads them. Binary units, because that is what a file manager shows. */
export function formatBytes(bytes) {
  // `null` and `undefined` are refused before `Number()` sees them, because
  // `Number(null)` is `0` — and 「0 B」 is the exact sentence AC-06 exists to
  // prevent for a measurement that was never taken. An honest zero still
  // renders as `0 B`; a missing answer does not.
  if (bytes === null || bytes === undefined || bytes === "") {
    return "未知";
  }
  const value = Number(bytes);
  if (!Number.isFinite(value) || value < 0) {
    return "未知";
  }
  if (value < 1024) {
    return `${value} B`;
  }
  const units = ["KB", "MB", "GB", "TB"];
  let scaled = value / 1024;
  let unit = 0;
  while (scaled >= 1024 && unit < units.length - 1) {
    scaled /= 1024;
    unit += 1;
  }
  // One decimal below 10, none above: 「1.5 MB」 is informative, 「1.0 MB」 is
  // noise, and 「1024.0 MB」 is a lie about precision.
  const rounded = scaled >= 10 ? Math.round(scaled).toString() : scaled.toFixed(1);
  return `${rounded} ${units[unit]}`;
}

/**
 * Seconds since the epoch as this machine's local time, or 「未知」.
 *
 * Local, not UTC: the user's ruling is that both sides keep the machine's own
 * timezone, and the Rust side deliberately sends a number rather than a
 * formatted string so that this is the one place the conversion happens.
 */
export function formatModified(seconds) {
  if (seconds === null || seconds === undefined) {
    return "未知";
  }
  const value = Number(seconds);
  if (!Number.isFinite(value) || value <= 0) {
    return "未知";
  }
  const date = new Date(value * 1000);
  if (Number.isNaN(date.getTime())) {
    return "未知";
  }
  const pad = (number) => String(number).padStart(2, "0");
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ` +
    `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
  );
}

/** What a log file's `kind` means to a person. */
const KIND_LABELS = Object.freeze({
  live: "正在写入",
  archive: "已归档",
  other: "无法识别",
});

export function logKindLabel(kind) {
  return KIND_LABELS[kind] ?? KIND_LABELS.other;
}

/** What one cleanup report says happened. */
export function describeCleanup(report) {
  const freed = formatBytes(report.freedBytes);
  const removed = report.filesRemoved;
  const failures = report.failures.length;

  // The order matters. A cleanup that failed *and* removed nothing is not a
  // cleanup that had nothing to do, and the two are indistinguishable from
  // `freed_bytes` alone — which is exactly why the report carries the counts.
  if (failures > 0 && removed === 0) {
    return {
      theme: "error",
      text: `清理没有完成：${failures} 项无法删除，没有文件被移除`,
      detail: report.failures.map((failure) => `${failure.path}：${failure.reason}`).join("\n"),
    };
  }
  if (failures > 0) {
    return {
      theme: "warning",
      text: `清理部分完成：删除 ${removed} 个文件（${freed}），${failures} 项无法删除`,
      detail: report.failures.map((failure) => `${failure.path}：${failure.reason}`).join("\n"),
    };
  }
  if (removed === 0) {
    return {
      theme: "info",
      text: "没有需要清理的文件",
      detail: report.kept.length
        ? `保留了 ${report.kept.length} 项：${report.kept.map((item) => item.name).join("、")}`
        : "",
    };
  }
  return {
    theme: "success",
    // The directory is named because 「删错了目录」 is a mistake a count hides.
    text: `已释放 ${freed}（${removed} 个文件），目录：${report.directory}`,
    detail: report.kept.length
      ? `保留了 ${report.kept.length} 项：${report.kept.map((item) => item.name).join("、")}`
      : "",
  };
}

/** What one diagnostic export's report says. */
export function describeDiagnostic(report) {
  const lines = [
    `已导出 ${formatBytes(report.bytes)}`,
    `文件：${report.path}`,
    `校验值：${report.sha256}`,
    `其中 ${report.entries.length} 个文件，另有 ${report.omitted.length} 个未收录`,
    `本机执行服务：${report.agentState === "answered" ? "已应答" : "不可达"}`,
  ];
  return {
    theme: "success",
    text: `诊断包已生成：${report.path}`,
    detail: lines.join("\n"),
    // Named so the page can show it apart from the rest: an export that could not
    // reach the Agent still produces a bundle, and a person attaching it should
    // know which one they have.
    agentUnreachable: report.agentState !== "answered",
  };
}

/**
 * What the push note says, from the **Agent's own answer** rather than from the
 * fact that the command returned.
 *
 * The answer is a read-back: the Agent reports the directory *it* has, which can
 * differ from what Desktop just stored — the Agent may have been down when the
 * choice was made, or its own store changed elsewhere. Saying 「已推送」 off a
 * successful return would report the request instead of the result, and the only
 * directory a download will actually use is the Agent's.
 */
export function describePush(facts) {
  if (!facts.saveDir) {
    return {
      theme: "warning",
      text: "本机执行服务没有记下这个位置。它下次启动时仍会读到这个位置，可以稍后再试一次。",
    };
  }
  if (!facts.writable) {
    return {
      theme: "warning",
      text: `本机执行服务现在会写到 ${facts.saveDir}，但它报告这个位置目前写不进去。素材会下载失败，直到它可写为止。`,
    };
  }
  return {
    theme: "info",
    text: `本机执行服务现在会写到 ${facts.saveDir}（可用 ${formatBytes(facts.freeBytes)}）。`,
  };
}

/**
 * Saving a new location does not move anything by itself, so the page asks what
 * moving the old files would involve and puts the answer in front of the person.
 * These are that dialog's rules.
 *
 * ## Nothing is selected to begin with
 *
 * The default for every file is 保留. Moving and deleting both act on files the
 * person did not create through this page, and a dialog whose default is 「搬运」
 * turns one button press into a bulk file operation nobody asked for. The cost of
 * the safe default is a few clicks; the cost of the other one is somebody's
 * files. 「存储在别处的文件」 is exactly the kind of thing that is cheaper to
 * re-download than to recover.
 */
export const FILE_CHOICES = Object.freeze({
  move: "move",
  delete: "delete",
  keep: "keep",
});

const CHOICE_LABELS = Object.freeze({
  move: "搬运",
  delete: "删除",
  keep: "保留",
});

export function choiceLabel(choice) {
  return CHOICE_LABELS[choice] ?? CHOICE_LABELS.keep;
}

/**
 * What each state a scanned file can be in is called.
 *
 * `tone` is `ResourceStatusBadge`'s, and 「已在当前位置」 is `success` rather than
 * `neutral` on purpose: it is the one state that needs nothing done to it, and a
 * neutral badge next to two warnings reads as a third problem.
 */
const FILE_STATUS = Object.freeze({
  movable: { label: "待搬运", tone: "warning" },
  already_there: { label: "已在当前位置", tone: "success" },
  missing: { label: "已不存在", tone: "neutral" },
});

/** What a `kept` reason token means. The Rust side sends tokens, not prose. */
const KEPT_REASON_LABELS = Object.freeze({
  same_directory: "已经在目标位置",
  target_exists: "目标位置已有同名文件",
  not_a_regular_file: "不是常规文件",
  symlink: "是符号链接",
});

export function keptReasonLabel(reason) {
  return KEPT_REASON_LABELS[reason] ?? reason ?? "";
}

/** The files a plan found nowhere, when at least one directory could not be listed. */
function missingStatusLabel(plan) {
  // 「已不存在」 is a claim about the file; it needs every known location to have
  // been read. With one directory unreadable the honest sentence is the weaker
  // one, because the file may be sitting in the directory nobody could open —
  // and a person told 「已删除」 re-downloads 230 MB they still have.
  return plan.unreadable.length ? { label: "未找到", tone: "neutral" } : FILE_STATUS.missing;
}

/**
 * The dialog's rows: every file the plan mentions, with the state it is in and
 * the choice that currently applies to it.
 *
 * Only 待搬运 rows carry a choice. A file already in the chosen directory has
 * nothing to decide, and a file that is not there cannot be acted on — giving
 * either of them buttons would be offering an action whose only outcome is a
 * report saying it did nothing.
 */
export function migrationRows(plan, choices = {}) {
  const missing = missingStatusLabel(plan);
  const rows = [
    ...plan.movable.map((file) => ({ file, status: "movable", selectable: true })),
    ...plan.alreadyThere.map((file) => ({ file, status: "already_there", selectable: false })),
    ...plan.missing.map((file) => ({ file, status: "missing", selectable: false, display: missing })),
  ];

  return rows.map(({ file, status, selectable, display }) => {
    const state = display ?? FILE_STATUS[status];
    return {
      name: file.name,
      directory: file.directory,
      // A size that was not read is 未知, never 「0 B」 (AC-06) — see `formatBytes`.
      bytesText: formatBytes(file.bytes),
      status,
      statusLabel: state.label,
      statusTone: state.tone,
      selectable,
      choice: selectable ? choices[file.name] ?? FILE_CHOICES.keep : FILE_CHOICES.keep,
    };
  });
}

/** The names marked for one action, in the order the rows are shown. */
export function chosenNames(rows, action) {
  return rows.filter((row) => row.selectable && row.choice === action).map((row) => row.name);
}

/**
 * What the dialog says before anything is touched: where the files would go, how
 * much that needs, and how much room there is.
 *
 * The sizes are a **lower bound** whenever one of them could not be read — which
 * is why the sentence says 「至少」. A total that quietly left out the unmeasured
 * file would be a number that gets a space check past a full disk.
 */
export function describeMigrationPlan(plan) {
  // The plan's own total, not one added up here: the side that walked the
  // directories counted once, and a second sum in the renderer is a second
  // definition of 「需要多少空间」 that can disagree with the one on the wire.
  const needed = plan.neededBytes;
  const unmeasured = plan.movable.filter((file) => !Number.isFinite(file.bytes)).length;
  const atLeast = unmeasured > 0 ? "至少 " : "";
  const shortfall = needed > plan.freeBytes;

  const lines = [
    `目标位置：${plan.to}`,
    `待搬运 ${plan.movable.length} 个文件，需要${atLeast}${formatBytes(needed)}；目标位置可用的空间：${formatBytes(plan.freeBytes)}`,
  ];
  if (plan.alreadyThere.length) {
    lines.push(`已在目标位置：${plan.alreadyThere.length} 个文件`);
  }
  if (plan.missing.length) {
    lines.push(
      plan.unreadable.length
        ? `在查得到的目录里没找到：${plan.missing.map((file) => file.name).join("、")}`
        : `已不在任何已知的保存位置：${plan.missing.map((file) => file.name).join("、")}`
    );
  }
  if (unmeasured > 0) {
    lines.push(`有 ${unmeasured} 个文件读不到体积，上面的需要量不含它们`);
  }
  // Named, and named loudly: this is the state that makes every 「没找到」 below it
  // less than a fact.
  if (plan.unreadable.length) {
    lines.push(
      `有 ${plan.unreadable.length} 个已知位置读不了，那里的文件没有查过：` +
        plan.unreadable.map((item) => `${item.directory}（${item.reason}）`).join("；")
    );
  }

  return {
    // A shortfall is information, not a refusal: the files still have to be moved
    // off a disk that is being replaced, and the person may be about to free the
    // space or to unplug something. The list below stays usable either way.
    theme: shortfall || plan.unreadable.length ? "warning" : "info",
    text: plan.movable.length
      ? `要把 ${plan.movable.length} 个文件搬到 ${plan.to}`
      : "没有需要搬运的文件",
    detail: lines.join("\n"),
    shortfall,
  };
}

/**
 * What one move or delete says happened.
 *
 * The order matters for the same reason it does in `describeCleanup`: an action
 * that failed *and* did nothing is not an action that had nothing to do, and the
 * two are indistinguishable from the done-count alone.
 */
export function describeMigration(report, action = FILE_CHOICES.move) {
  const removing = action === FILE_CHOICES.delete;
  const verb = removing ? "删除" : "搬运";
  const done = removing ? report.deleted : report.moved;
  const failures = report.failures.length;
  const lines = [];

  if (report.kept.length) {
    lines.push(
      `保留了 ${report.kept.length} 项：` +
        report.kept.map((item) => `${item.name}（${keptReasonLabel(item.reason)}）`).join("、")
    );
  }
  if (report.missing.length) {
    lines.push(`名单里有 ${report.missing.length} 个文件没找到：${report.missing.join("、")}`);
  }
  if (failures) {
    lines.push(report.failures.map((failure) => `${failure.name}：${failure.reason}`).join("\n"));
  }
  const detail = lines.join("\n");

  if (failures && done.length === 0) {
    return { theme: "error", text: `${verb}没有完成：${failures} 项失败，没有文件被${verb}`, detail };
  }
  if (failures) {
    return {
      theme: "warning",
      text: `${verb}部分完成：${done.length} 个文件已${verb}，${failures} 项失败`,
      detail,
    };
  }
  if (done.length === 0) {
    return { theme: "info", text: `没有文件需要${verb}`, detail };
  }
  return {
    theme: "success",
    // The destination is named for a move and not for a delete: a deletion has no
    // destination, and naming the chosen directory next to it would read as 「these
    // were deleted from there」, which is not what the report says.
    text: removing
      ? `已删除 ${done.length} 个文件`
      : `已搬到 ${report.directory}：${done.length} 个文件`,
    detail,
  };
}

/**
 * The whole page's view state.
 *
 * `settings` and `usage` are separate answers from separate commands, and either
 * may be missing on a first render — so each is described independently and the
 * page shows whichever arrived.
 */
export function createLocalSettingsPage({ settings = null, usage = null } = {}) {
  return {
    availableText: usage ? formatBytes(usage.availableBytes) : "",
    cacheText: usage ? formatBytes(usage.cacheBytes) : "",
    // 「未设置」 rather than a guessed default: the default a task would use is
    // the task's decision, and this page cannot truthfully state it.
    saveDirText: settings ? (settings.saveDir ?? "未设置") : "",
    saveDirSet: Boolean(settings?.saveDir),
    fileText: settings?.file ?? "",
    fileSet: Boolean(settings?.file),
    logTrees: (usage?.logs ?? []).map((tree) => ({
      source: tree.source,
      directory: tree.directory,
      bytesText: formatBytes(tree.bytes),
      filesText: `${tree.files} 个文件`,
      // 「空」 rather than 「0 B」: the two are the same measurement but only one of
      // them reads as a state rather than as a size.
      isEmpty: tree.files === 0,
    })),
  };
}
