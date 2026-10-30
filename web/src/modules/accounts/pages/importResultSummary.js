export function importResultSummary(counts = {}, running = false) {
  if (running) return { title: '正在接入账号', tone: 'info', description: '当前项完成后可暂停，已完成项会保留。' }
  if (counts.uncertain || counts.running) return { title: '部分结果待确认', tone: 'warning', description: '请先按行核实实际结果，再继续接入。' }
  if (counts.pending) return { title: '本轮接入已暂停', tone: 'warning', description: '已完成项保留，可继续接入待处理账号。' }
  if (counts.total && counts.failed === counts.total) return { title: '本轮接入未完成', tone: 'danger', description: '请按行查看原因，处理后再重试。' }
  if (counts.failed) return { title: '接入完成，部分账号需处理', tone: 'warning', description: '成功项已保存，请按行处理未完成账号。' }
  if (!counts.succeeded) return { title: '本轮未接入账号', tone: 'info', description: '没有成功接入的账号，可查看逐项结果。' }
  return { title: '本轮接入完成', tone: 'success', description: '成功项已保存，可查看账号详情。' }
}
