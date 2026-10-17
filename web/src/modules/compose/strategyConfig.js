// 表格、详情与任务视图共用模板名称的展示方式。

/** 合成模板：`名称 v版本`；认不出模板时退回编号，不假装知道它叫什么。 */
export function templateLabel(templates, id) {
  const template = (Array.isArray(templates) ? templates : []).find((item) => item.id === id)
  if (template) return `${template.name} v${template.version}`
  return id === undefined || id === null ? '—' : `模板 ${id}`
}

