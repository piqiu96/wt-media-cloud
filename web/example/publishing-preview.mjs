import { readFile, writeFile, rename, mkdir } from "node:fs/promises";
import path from "node:path";
import os from "node:os";
import { gunzip } from "node:zlib";
import { promisify } from "node:util";
const unzip = promisify(gunzip);
import { unpackSnapshot } from "./proto/publishing-design/snapshotTransport.js";
import { createHash } from "node:crypto";
import {
  cancelPublication,
  verifyResult,
  processHuman,
} from "./proto/publishing-design/taskActions.js";
import { canRetry } from "./proto/publishing-design/model.js";
import { reconcileExecutionGroup } from "./proto/publishing-design/batchExecutionSettings.js";
import { queueTask } from "./proto/publishing-design/relationships.js";
import { planAction, protectPlanSnapshot } from "./proto/publishing-design/planBridge.js";
import {
  batchState,
  executionChecks,
  runBatchCommand,
  protectBatchSnapshot,
  migrateBatchLifecycle,
} from "./proto/publishing-design/batchLifecycle.js";
const clone = (x) => JSON.parse(JSON.stringify(x));
const fingerprint = (value) =>
  "sha256:" + createHash("sha256").update(value).digest("hex");
export function createBatchStore({ file, check } = {}) {
  let state = null,
    queue = Promise.resolve(),
    initialization;
  async function read() {
    initialization ||= (async () => {
      if (file) {
        try {
          state = JSON.parse(await readFile(file, "utf8"));
        } catch (e) {
          if (e.code !== "ENOENT") throw e;
        }
      }
      const previous = JSON.stringify(state);
      if (state?.data) migrateBatchLifecycle(state.data);
      for (const [key, value] of Object.entries(state?.receipts || {}))
        if (!value.startsWith("sha256:"))
          state.receipts[key] = fingerprint(value);
      // Persist a legacy migration before returning it to any client. Otherwise
      // a preview restart regenerates its freeze timestamp at the same revision.
      if (file && state && JSON.stringify(state) !== previous) {
        await mkdir(path.dirname(file), { recursive: true });
        await writeFile(file + ".tmp", JSON.stringify(state));
        await rename(file + ".tmp", file);
      }
    })().catch((error) => {
      initialization = undefined;
      throw error;
    });
    await initialization;
    return state;
  }
  function transact(work) {
    const run = queue
      .catch(() => {})
      .then(async () => {
        await read();
        const next = clone(state);
        const result = await work(next);
        if (file) {
          await mkdir(path.dirname(file), { recursive: true });
          await writeFile(file + ".tmp", JSON.stringify(result));
          await rename(file + ".tmp", file);
        }
        state = result;
        return clone(state);
      });
    queue = run;
    return run;
  }
  return {
    get: async () => {
      await queue.catch(() => {});
      return clone(await read());
    },
    bootstrap: (data) =>
      transact((current) => {
        if (current) return current;
        const db = clone(data);
        if (!db?.batches || !db.tasks) throw Error("演示数据格式不完整");
        migrateBatchLifecycle(db);
        return { revision: 1, data: db };
      }),
    save: (data, revision) =>
      transact((current) => {
        if (!current || revision !== current.revision)
          throw Error("409：其他窗口已更新，请刷新页面后重试");
        const db = clone(data);
        for (const b of db.batches) {
          const old = current.data.batches.find((x) => x.id === b.id);
          if (old && !old.firstStartedAt && b.firstStartedAt) {
            const tasks = db.tasks.filter(
              (t) => t.batchId === b.id && t.status === "排队中",
            );
            if (!tasks.length) throw Error("首次执行缺少可执行任务");
            const pending = current.data.tasks.filter(
              (t) => t.batchId === b.id && t.status === "待执行" && !t.claimed,
            );
            if (tasks.length !== pending.length)
              throw Error("部分执行必须通过批次执行接口明确确认");
            const parent = db.plans?.find((p) =>
              p.items?.some((i) => i.kind === "batch" && i.id === b.id),
            );
            if (
              parent &&
              (parent.items[parent.position]?.id !== b.id ||
                !parent.lockedAt ||
                !(
                  (parent.owner === "plan" && parent.control === "执行中") ||
                  (parent.owner === "manual" && parent.control === "已暂停")
                ))
            )
              throw Error("计划尚未轮到此批或缺少有效调度权");
            for (const t of tasks) {
              const errors = executionChecks(t, db);
              if (errors.length) throw Error(errors.join("；"));
            }
          }
        }
        protectPlanSnapshot(current.data, db);
        protectBatchSnapshot(current.data, db);
        migrateBatchLifecycle(db);
        return {
          revision: revision + 1,
          data: db,
          receipts: current.receipts || {},
        };
      }),
    command: ({ batchId, action, input = {}, key, revision }) =>
      transact((current) => {
        if (!current) throw Error("尚未初始化");
        const signature = fingerprint(
          JSON.stringify({ batchId, action, input }),
        );
        if (key && current.receipts?.[key]) {
          if (current.receipts[key] !== signature)
            throw Error("请求标识已用于其他操作");
          return current;
        }
        if (revision !== current.revision)
          throw Error("409：其他窗口已更新，请刷新页面后重试");
        const db = clone(current.data),
          b = db.batches.find((b) => b.id === batchId);
        const beforeState = b ? batchState(db, b) : null,
          logLength = b?.log?.length || 0;
        if (action === "planOperation") {
          planAction(db, input.planId, input.action, input.value);
        } else if (action === "taskOperation") {
          const t = db.tasks.find(
            (t) => t.id === input.taskId && t.batchId === batchId,
          );
          if (!b || !t) throw Error("任务不存在");
          if (
            t.status !== input.expectedStatus ||
            (t.attempts || 0) !== input.expectedAttempts
          )
            throw Error("任务已变化，请重新打开");
          if (input.action === "cancel") cancelPublication(t, input.reason);
          else if (input.action === "human") {
            if (b.executionReview && input.human === "validated") {
              if (b.control !== "执行中" || b.stoppedAt || b.endedAt || !t.suspendedAt)
                throw Error("批次或原现场当前不可继续，保留原尝试");
              const account = db.accounts.find(item => item.id === t.accountId);
              const active = db.tasks.filter(item => item.status === "执行中");
              if (active.filter(item => item.batchId === b.id).length >= b.executionSettings.concurrency ||
                  active.filter(item => db.accounts.find(row => row.id === item.accountId)?.ownerId === account?.ownerId).length >= 5)
                throw Error("自动执行名额已满，原现场仍保留");
            }
            processHuman(t, input.human);
            if (b.executionReview && t.status === "执行中") delete t.suspendedAt;
          }
          else if (["verify", "supplement"].includes(input.action)) {
            if (input.action === "verify" && t.status === "执行失败") {
              if (canRetry({ ...t, batchStopped: false }))
                throw Error("已确认未发布，请使用重试或放弃任务");
              t.status = "结果待确认";
              if (!t.attemptHistory?.length && t.attempts > 0)
                t.attemptHistory = [
                  { number: t.attempts, source: "历史尝试编号，原日志缺失" },
                ];
            }
            if (
              input.action === "verify" &&
              !["已发布", "平台审核中", "执行失败", "结果待确认"].includes(input.status)
            )
              throw Error("请选择有效的核实结论");
            if (
              input.status === "已发布" &&
              (!input.publicAt ||
                !Number.isFinite(Date.parse(input.publicAt)) ||
                Date.parse(input.publicAt) > Date.now())
            )
              throw Error("请填写有效的实际发布时间");
            if (input.action === "supplement") {
              if (
                !["平台审核中", "待补链接"].includes(t.status) ||
                input.status !== "已发布"
              )
                throw Error("仅允许补录待公开结果任务");
              t.status = "结果待确认";
              if (!t.attemptHistory?.length && t.attempts > 0)
                t.attemptHistory = [
                  { number: t.attempts, source: "历史尝试编号，原日志缺失" },
                ];
              input.attempt = t.attemptHistory?.at(-1)?.number;
            }
            verifyResult(db, t, input, "本地演示用户");
            if (input.status === "已发布")
              t.publicAt = new Date(input.publicAt).toISOString();
          } else if (input.action === "start") {
            if (!b.firstStartedAt || b.stoppedAt || t.status !== "待执行")
              throw Error("当前不可执行任务");
            const errors = executionChecks(t, db);
            if (errors.length) throw Error(errors.join("；"));
            if (!queueTask(db, t)) throw Error("任务已变化");
          } else if (input.action === "retry") {
            if (b.stoppedAt || !canRetry(t))
              throw Error("停止发布或原任务未发布证据不足，禁止重试");
            const errors = executionChecks(t, db);
            if (errors.length) throw Error(errors.join("；"));
            if (!queueTask(db, t, { retry: true })) throw Error("任务已变化");
          } else throw Error("不支持的任务操作");
          if (b.executionReview) reconcileExecutionGroup(db, b, Date.now(), { dispatch: false });
          b.updatedAt = new Date().toISOString();
          b.version = (b.version || 0) + 1;
          b.log ||= [];
          b.log.push({
            time: b.updatedAt,
            actor: "本地演示用户",
            type: "任务处理",
            text: `T-${t.id}：${{ cancel: "取消任务", human: "人工处理", verify: "核实结果", supplement: "补录公开结果", start: "执行任务", retry: "重试任务" }[input.action]} → ${t.status}`,
            result: "成功",
          });
        } else runBatchCommand(db, b, action, input, check);
        migrateBatchLifecycle(db);
        for (const entry of b?.log?.slice(logLength) || [])
          Object.assign(entry, { beforeState, afterState: batchState(db, b) });
        return {
          revision: current.revision + 1,
          data: db,
          receipts: {
            ...current.receipts,
            ...(key ? { [key]: signature } : {}),
          },
        };
      }),
  };
}
export function createPublishingMiddleware({ webRoot }) {
  const stores = new Map();
  return async (req, res, next) => {
    const url = new URL(req.url, "http://localhost");
    if (!url.pathname.startsWith("/__example/publishing/")) return next();
    res.setHeader("Content-Type", "application/json; charset=utf-8");
    res.setHeader("Cache-Control", "no-store");
    try {
      const host = req.headers.host || "";
      if (!/^(127\.0\.0\.1|localhost|\[::1\])(?::\d+)?$/.test(host))
        throw Error("仅允许本机原型预览");
      if (req.headers.origin && new URL(req.headers.origin).host !== host)
        throw Error("拒绝跨站写入");
      const hash = createHash("sha256")
        .update(webRoot + host)
        .digest("hex")
        .slice(0, 16);
      if (!stores.has(hash))
        stores.set(
          hash,
          createBatchStore({
            file: path.join(os.tmpdir(), "wt-media-example", hash + ".json"),
          }),
        );
      const store = stores.get(hash);
      let body = {};
      if (req.method !== "GET") {
        const encoding = req.headers["content-encoding"];
        if (encoding && encoding !== "identity" && encoding !== "gzip")
          throw Error("不支持的内容编码");
        const chunks = [];
        let length = 0;
        for await (const part of req) {
          length += part.length;
          if (length > 40 * 1024 * 1024)
            throw Error("保存内容超过预览传输上限");
          chunks.push(part);
        }
        const encoded = Buffer.concat(chunks);
        // The expanded limit is bounded independently to reject compressed bombs.
        const raw = encoding === "gzip"
          ? await unzip(encoded, { maxOutputLength: 256 * 1024 * 1024 })
          : encoded;
        body = unpackSnapshot(JSON.parse(raw.toString("utf8") || "{}"));
      }
      const route = url.pathname.split("/").at(-1);
      let result;
      if (req.method === "GET" && route === "snapshot")
        result = await store.get();
      else if (req.method === "POST" && route === "bootstrap")
        result = await store.bootstrap(body.data);
      else if (req.method === "PUT" && route === "snapshot") {
        const saved = await store.save(body.data, body.revision);
        result = { revision: saved.revision };
      }
      else if (req.method === "POST" && route === "command")
        result = await store.command(body);
      else {
        res.statusCode = 405;
        res.end(JSON.stringify({ error: "不支持的原型操作" }));
        return;
      }
      res.end(JSON.stringify(result));
    } catch (e) {
      res.statusCode = e.message.startsWith("409") ? 409 : 422;
      res.end(JSON.stringify({ error: e.message }));
    }
  };
}
