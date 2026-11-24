// Local prototype scheduling only; no external executor or browser calls.
export function taskInterval(value=0) {
  const seconds=Number(value);
  if(!Number.isSafeInteger(seconds)||seconds<0) throw Error('任务间隔须为非负整数秒');
  return seconds;
}
export function batchTaskCanStart(db,batch,task,now) {
  if (batch?.executionReview) {
    reconcileExecutionGroup(db, batch, now);
    return batch.control === "执行中" && batch.executionGroup?.number === task.executionGroup &&
      batch.executionGroup?.taskIds.includes(task.id);
  }
  if(!batch || !Object.hasOwn(batch.rules||{},'taskIntervalSeconds')) return true;
  const interval=taskInterval(batch.rules.taskIntervalSeconds);
  if(interval===0) return true;
  const rows=db.tasks.filter(t=>t.batchId===batch.id);
  if(rows.some(t=>t.id!==task.id&&(['执行中','等待人工','结果待确认'].includes(t.status)||(t.status==='执行失败'&&t.result!=='确认未发布')))) return false;
  if(rows.find(t=>t.status==='排队中'&&!t.claimed)?.id!==task.id) return false;
  const ended=Math.max(0,...rows.filter(t=>t.id!==task.id).map(t=>Date.parse(t.executionEndedAt||t.attemptHistory?.at(-1)?.ended||'')||0));
  return now>=ended+interval*1000;
}

const occupiedScene = ({ status }) => ["执行中", "等待人工", "结果待确认"].includes(status);
const automaticEnd = (task) => {
  if (task.status === "等待人工") return Date.parse(task.suspendedAt || "") || 0;
  if (occupiedScene(task) || task.status === "排队中") return 0;
  return Date.parse(task.executionEndedAt || task.attemptHistory?.at(-1)?.ended || "") || 0;
};
const exception = (task) => ["执行失败", "等待人工", "结果待确认"].includes(task.status);
const tasksFor = (db, batch) => db.tasks.filter(task => task.batchId === batch.id);
const accountFor = (db, task) => db.accounts.find(account => account.id === task.accountId);
export const profileOf = account => account?.profileId ?? account?.window;
export const uidOf = (account) => `${account?.platform}:${account?.platformUid ?? account?.id}`;

export function executionExceptions(db, batch) {
  return tasksFor(db, batch).filter(exception).map(task => ({
    task, key: JSON.stringify([task.id, task.status, task.attempts, task.version, task.evidence]),
  }));
}

export function executionPause(batch, reason, now = Date.now()) {
  batch.control = "已暂停";
  batch.pause = { id: (batch.lastPauseId || 0) + 1, reason, at: new Date(now).toISOString() };
  batch.lastPauseId = batch.pause.id;
}

export function groupCandidates(db, batch, now) {
  const scenes = db.tasks.filter(occupiedScene);
  const profiles = new Set(scenes.map(task => profileOf(accountFor(db, task))));
  const uids = new Set(scenes.map(task => uidOf(accountFor(db, task))));
  const capacity = Math.max(0, (db.exampleSceneCapacity || 5) - scenes.filter(task => accountFor(db, task)?.ownerId === batch.ownerId).length);
  const effectiveSize = Math.min(batch.executionSettings.concurrency, capacity);
  const waiting = tasksFor(db, batch).filter(task => task.status === "排队中" && (!task.schedule || Date.parse(task.schedule) <= now));
  const results = [];
  for (const task of waiting) {
    if (results.length >= effectiveSize) break;
    const account = accountFor(db, task);
    const profile = profileOf(account);
    const uid = uidOf(account);
    if (!account?.enabled || account.loginValid === false || account.ownerId !== batch.ownerId || !profile || profiles.has(profile) || uids.has(uid)) continue;
    results.push(task);
    profiles.add(profile);
    uids.add(uid);
  }
  return {
    results,
    reason: results.length || !tasksFor(db, batch).some(task => task.status === "排队中") ? "" :
      capacity === 0 ? "可保留的浏览器现场已满，等待现有任务处理" :
      waiting.length ? "等待可用账号或浏览器 Profile" : "等待就绪任务",
  };
}

// Extend the existing queue/claim demo with a fixed group and a group-end clock.
export function reconcileExecutionGroup(db, batch, now = Date.now(), { dispatch = true } = {}) {
  if (!batch?.executionReview || !batch.firstStartedAt || batch.stoppedAt || batch.endedAt) return;
  const tasks = tasksFor(db, batch);
  const group = batch.executionGroup;
  let justEnded = false;
  if (group && !group.endedAt) {
    const times = group.taskIds.map(id => {
      const row = tasks.find(task => task.id === id && task.executionGroup === group.number);
      return row ? automaticEnd(row) : group.startedAt;
    });
    if (times.every(Boolean)) {
      group.endedAt = Math.max(...times);
      group.nextAt = group.endedAt + batch.executionSettings.groupInterval * 1000;
      justEnded = true;
    }
  }
  if (batch.control !== "执行中" || !tasks.some(task => task.status === "排队中")) return;
  const exceptions = executionExceptions(db, batch);
  const unacknowledged = exceptions.filter(item => !batch.acknowledgedExceptions?.includes(item.key));
  if (unacknowledged.length && exceptions.length >= batch.executionSettings.exceptionLimit) {
    executionPause(batch, "异常任务达到上限", now);
    return;
  }
  if (justEnded && batch.executionSettings.groupPolicy === "异常暂停" &&
      unacknowledged.some(item => group.taskIds.includes(item.task.id))) {
    executionPause(batch, "本组处理结束，存在待处理异常", now);
    return;
  }
  if (!dispatch || (group && (!group.endedAt || now < group.nextAt))) return;
  const { results } = groupCandidates(db, batch, now);
  if (!results.length) return;
  const number = (group?.number || 0) + 1;
  for (const task of results) task.executionGroup = number;
  batch.executionGroup = { number, taskIds: results.map(task => task.id), startedAt: now };
}
