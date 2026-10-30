const COLUMNS = ['名称', '协议', '地址', '端口', '用户名', '密码', '地区', '供应商', '最大窗口数', '备注']
export const MAX_IMPORT_ROWS = 1000
const MAX_BYTES = 5 * 1024 * 1024

export const PROXY_IMPORT_TEMPLATE = `${COLUMNS.join(',')}\r\n示例线路,socks5,203.0.113.128,1080,,,上海,星云网络,3,日常线路\r\n`

export function parseCsvRecords(text) {
  const rows = []
  let row = []
  let cell = ''
  let quoted = false
  const source = String(text || '').replace(/^\uFEFF/, '')
  for (let index = 0; index < source.length; index += 1) {
    const char = source[index]
    if (char === '"') {
      if (quoted && source[index + 1] === '"') { cell += '"'; index += 1 }
      else quoted = !quoted
    } else if (char === ',' && !quoted) {
      row.push(cell); cell = ''
    } else if ((char === '\n' || char === '\r') && !quoted) {
      row.push(cell); rows.push(row); row = []; cell = ''
      if (char === '\r' && source[index + 1] === '\n') index += 1
    } else cell += char
  }
  if (quoted) throw new Error('CSV 引号未闭合，请修正文件后重试。')
  if (cell || row.length) { row.push(cell); rows.push(row) }
  return rows
}

function normalizedHeader(value) { return String(value || '').trim().replace(/^\uFEFF/, '') }

export function parseImportTable(records) {
  const rows = records.map((cells, index) => ({ cells, sourceLine: index + 1 }))
    .filter(row => row.cells.some(cell => String(cell ?? '').trim()))
  if (!rows.length) throw new Error('文件没有可导入的代理内容。')
  const header = rows[0].cells.map(normalizedHeader)
  if (header.length === 1 && !COLUMNS.includes(header[0])) {
    if (rows.length > MAX_IMPORT_ROWS) throw new Error(`每次最多导入 ${MAX_IMPORT_ROWS} 行。`)
    return {
      lines: rows.map(row => String(row.cells[0] ?? '').trim()),
      metadata: rows.map(row => ({ sourceLine: row.sourceLine })),
    }
  }
  if (rows.length > MAX_IMPORT_ROWS + 1) throw new Error(`每次最多导入 ${MAX_IMPORT_ROWS} 行。`)
  if (!COLUMNS.every(column => header.includes(column))) {
    throw new Error(`表格列不完整，请使用下载模板：${COLUMNS.join('、')}。`)
  }
  const positions = Object.fromEntries(COLUMNS.map(column => [column, header.indexOf(column)]))
  const metadata = []
  const lines = []
  for (const row of rows.slice(1)) {
    const cells = row.cells
    const value = column => String(cells[positions[column]] ?? '').trim()
    const protocol = value('协议') || 'http'
    const host = value('地址')
    const port = value('端口')
    const username = value('用户名')
    const password = value('密码')
    const authority = username || password ? `${encodeURIComponent(username)}:${encodeURIComponent(password)}@` : ''
    const normalizedHost = host.includes(':') && !host.startsWith('[') ? `[${host}]` : host
    lines.push(`${protocol}://${authority}${normalizedHost}:${port}`)
    const rawQuota = value('最大窗口数')
    metadata.push({
      name: value('名称'), region: value('地区'), supplier: value('供应商'),
      max_profiles: rawQuota ? Number(rawQuota) : 0, remark: value('备注'),
      sourceLine: row.sourceLine,
      error: rawQuota && (!/^\d+$/.test(rawQuota) || Number(rawQuota) < 1 || Number(rawQuota) > 1000)
        ? '最大窗口数须为 1–1000 的整数。' : '',
    })
  }
  if (!lines.length) throw new Error('表格只有表头，没有代理内容。')
  return { lines, metadata }
}

export async function readProxyImportFile(file) {
  if (file.size > MAX_BYTES) throw new Error('文件不能超过 5 MB。')
  const name = String(file.name || '').toLowerCase()
  let parsed
  if (name.endsWith('.txt')) {
    const rows = (await file.text()).split(/\r?\n/)
      .map((line, index) => ({ line: line.trim(), sourceLine: index + 1 }))
      .filter(row => row.line)
    if (rows.length > MAX_IMPORT_ROWS) throw new Error(`每次最多导入 ${MAX_IMPORT_ROWS} 行。`)
    parsed = { lines: rows.map(row => row.line), metadata: rows.map(row => ({ sourceLine: row.sourceLine })) }
  } else if (name.endsWith('.csv')) {
    parsed = parseImportTable(parseCsvRecords(await file.text()))
  } else if (name.endsWith('.xlsx')) {
    const { readSheet } = await import('read-excel-file/browser')
    const rows = await readSheet(file)
    parsed = parseImportTable(rows)
  } else throw new Error('请选择 .xlsx、.csv 或 .txt 文件。')
  if (!parsed.lines.length) throw new Error('文件没有可导入的代理内容。')
  return { ...parsed, name: file.name }
}
