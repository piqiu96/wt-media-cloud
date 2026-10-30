import { describe, expect, it } from 'vitest'
import { parseCsvRecords, parseImportTable, PROXY_IMPORT_TEMPLATE, readProxyImportFile } from './importFile.js'

describe('proxy import files', () => {
  it('preserves quoted commas and credentials while mapping template metadata', () => {
    const text = PROXY_IMPORT_TEMPLATE.replace('日常线路', '日常,发布')
    const quoted = text.replace('日常,发布', '"日常,发布"')
    const parsed = parseImportTable(parseCsvRecords(quoted))
    expect(parsed.lines).toEqual(['socks5://203.0.113.128:1080'])
    expect(parsed.metadata[0]).toMatchObject({ name: '示例线路', region: '上海', supplier: '星云网络', max_profiles: 3, remark: '日常,发布' })
  })

  it('rejects a bad quota instead of silently applying a default', () => {
    const parsed = parseImportTable(parseCsvRecords(PROXY_IMPORT_TEMPLATE.replace(',3,', ',abc,')))
    expect(parsed.metadata[0].error).toContain('最大窗口数')
  })

  it('accepts one-column address CSV without requiring the table template', () => {
    expect(parseImportTable(parseCsvRecords('host.example:1080\r\nother.example:9090')).lines).toEqual(['host.example:1080', 'other.example:9090'])
  })

  it('keeps original line numbers when text and CSV files contain blank rows', async () => {
    const text = await readProxyImportFile({ name: 'lines.txt', size: 40, text: async () => '\nfirst.example:1080\n\nsecond.example:9090\n' })
    const csv = parseImportTable(parseCsvRecords('\nfirst.example:1080\n\nsecond.example:9090\n'))
    expect(text.metadata.map(row => row.sourceLine)).toEqual([2, 4])
    expect(csv.metadata.map(row => row.sourceLine)).toEqual([2, 4])
  })
})
