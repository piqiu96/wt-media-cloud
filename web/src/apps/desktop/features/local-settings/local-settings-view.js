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
