import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import MyOutputsPage from './MyOutputsPage.vue'

const source = readFileSync(new URL('./MyOutputsPage.vue', import.meta.url), 'utf8')
const listMarkup = source.slice(source.indexOf('<ResourcePageHeader'), source.indexOf('<t-drawer'))
const rowActions = listMarkup.slice(listMarkup.indexOf('<template #op='), listMarkup.indexOf('</template>', listMarkup.indexOf('<template #op=')))

describe('output management page', () => {
  it('compiles as the shared Cloud and Desktop page', () => {
    expect(MyOutputsPage).toBeTruthy()
  })
  it('keeps the operating filters and selected batch download on the existing table', () => {
    for (const name of ['search', 'trigger', 'download', 'created_range']) {
      expect(source).toContain(`v-model="filters.${name}"`)
    }
    expect(source).toContain('综合搜索')
    expect(source).not.toContain('v-model="filters.archived"')
    expect(source).not.toContain('v-model="filters.output"')
    expect(source).not.toContain('v-model="filters.source"')
    expect(source).toContain('v-model:selected-row-keys="selectedRowKeys"')
    expect(source).toContain('已选 {{ selectedRowKeys.length }} 项')
    expect(source).toContain('批量下载')
    expect(source).not.toContain('取消选择</t-button>')
    expect(source).toContain("checkProps: ({ row }) => ({ disabled: !canDownload(row) })")
    expect(source).toContain('selectedRowKeys.value = selectedRowKeys.value.filter')
    expect(source).toContain('已提交 ${submitted} 个下载任务')
    expect(source).toContain('下载任务提交失败')
  })

  it('sends search and trigger filters to the backend and keeps only the local download overlay', () => {
    expect(source).toContain('params.search = filters.value.search.trim()')
    expect(source).toContain('params.trigger_type = filters.value.trigger')
    expect(source).not.toContain('matchesOutputSearch')
    expect(source).not.toContain('searchedMaterials')
    expect(source).toContain("!filters.value.download || downloadFor(row).key === filters.value.download")
  })

  it('shows asset, source, strategy, generation and real download facts in the list', () => {
    for (const key of ['output', 'material_id', 'strategy', 'trigger', 'download_status', 'created_at']) {
      expect(source).toContain(`<template #${key}=`)
    }
    expect(source).toContain('titleFor(row)')
    expect(source).toContain('generationLabel(triggerFor(row))')
    expect(source).toContain('downloadFor(row).label')
    expect(source).toContain('filteredRows.length')
  })

  it('uses the existing outlined row actions and keeps local opening inside file information', () => {
    expect(rowActions).toContain('详情')
    expect(rowActions).toContain('下载')
    expect(rowActions).toContain('class="wt-secondary-button" variant="outline"')
    expect(rowActions).not.toContain('打开文件')
    expect(rowActions).not.toContain('打开文件夹')
    expect(rowActions).not.toContain('去发布')
    expect(rowActions).not.toContain('归档')
    expect(source).toContain('@open-folder="openLocalFolder(detail.output)"')
    expect(source).not.toContain('@open-file=')
    expect(source).not.toContain('openSavedFile')
  })

  it('opens a fresh preview using the existing video link and associated entity drawers', () => {
    expect(source).toContain('client.getComposeOutputVideoUrl(row.id)')
    expect(source).toContain('client.getComposeOutput(row.id)')
    expect(source.match(/publicObjectReadUrl\(link\?\.url\)/g)).toHaveLength(2)
    expect(source).toContain('MaterialDetailDrawer')
    expect(source).toContain('ComposeStrategySummary')
    expect(source).toContain('@open-task="openTask"')
    expect(source).not.toContain('router.push')
  })
})
