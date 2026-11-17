// Local prototype scheduling only; no external executor or browser calls.
export function taskInterval(value=0) {
  const seconds=Number(value);
  if(!Number.isSafeInteger(seconds)||seconds<0) throw Error('任务间隔须为非负整数秒');
  return seconds;
}
export function batchTaskCanStart(db,batch,task,now) {
  if(!batch || !Object.hasOwn(batch.rules||{},'taskIntervalSeconds')) return true;
  const interval=taskInterval(batch.rules.taskIntervalSeconds);
  if(interval===0) return true;
  const rows=db.tasks.filter(t=>t.batchId===batch.id);
  if(rows.some(t=>t.id!==task.id&&(['执行中','等待人工','结果待确认'].includes(t.status)||(t.status==='执行失败'&&t.result!=='确认未发布')))) return false;
  if(rows.find(t=>t.status==='排队中'&&!t.claimed)?.id!==task.id) return false;
  const ended=Math.max(0,...rows.filter(t=>t.id!==task.id).map(t=>Date.parse(t.executionEndedAt||t.attemptHistory?.at(-1)?.ended||'')||0));
  return now>=ended+interval*1000;
}
