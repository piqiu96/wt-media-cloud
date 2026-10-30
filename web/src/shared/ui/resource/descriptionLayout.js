// 按实际内容容器与短字段密度排版；业务可用 full 显式指定满行。
export function descriptionLayout(fields, width) {
  const full = fields.map(field => (field.full === '' || field.full === true) || /名称|地址|备注|原因|说明|链接|密码|当前代理|托管代理/.test(String(field.label || '')))
  const columns = width >= 640 && full.filter(value => !value).length >= 4 ? 2 : 1
  return { columns, spans: full.map(value => value ? columns : 1) }
}
