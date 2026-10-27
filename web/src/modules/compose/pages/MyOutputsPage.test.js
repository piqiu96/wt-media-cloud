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

  it('sends search, trigger and operator filters to the backend and keeps only the local download overlay', () => {
    expect(source).toContain('params.search = filters.value.search.trim()')
    expect(source).toContain('params.trigger_type = filters.value.trigger')
    expect(source).toContain('v-model="filters.operator"')
    expect(source).toContain('operatorOptions.value.map((option) => ({ value: `op:${option.id}`, label: option.display_name }))')
    expect(source).not.toContain("value: 'mine'")
    expect(source).not.toContain("value: 'team'")
    expect(source).not.toContain("filters.value.operator = 'mine'")
    expect(source).not.toContain('params.operator_scope')
    expect(source).toContain('if ((isAdmin.value || isSenior.value) && namedOperator) params.operator_id = namedOperator')
    expect(source).toContain('const namedOperator = operatorIdFromOption(filters.value.operator)')
    // 清空下拉给的是 undefined，不是空串：解析和“选项是否还在”都必须走守卫。
    expect(source).toContain('missingOperatorOption(filters.value.operator, operatorFilterOptions.value)')
    expect(source).not.toContain('filters.value.operator.startsWith')
    expect(source).toContain('params.team_id = Number(filters.value.team_id)')
    expect(source).not.toContain(':clearable="!isSenior"')
    expect(source).not.toContain('matchesOutputSearch')
    expect(source).not.toContain('searchedMaterials')
    expect(source).toContain("!filters.value.download || downloadFor(row).key === filters.value.download")
  })

  it('shows asset, source, game, strategy, operator, generation and real download facts in the list', () => {
    for (const key of ['output', 'material_id', 'game', 'strategy', 'operator', 'trigger', 'download_status', 'created_at']) {
      expect(source).toContain(`<template #${key}=`)
    }
    expect(source).toContain('gameName(row.game_id)')
    expect(source).toContain("row.operator_name || '未记录'")
    expect(source).toContain('titleFor(row)')
    expect(source).toContain('triggerLabel(triggerFor(row))')
    expect(source).toContain(':options="TRIGGER_TYPE_OPTIONS"')
    expect(source).not.toContain('generationLabel')
    expect(source).not.toContain('const triggerOptions')
    expect(source).toContain('downloadFor(row).label')
    expect(source).toContain('filteredRows.length')
    // 游戏列紧跟来源素材，早于策略；筛选与列说的是同一个字段。
    expect(source.indexOf("{ colKey: 'game'")).toBeGreaterThan(source.indexOf("{ colKey: 'material_id'"))
    expect(source.indexOf("{ colKey: 'game'")).toBeLessThan(source.indexOf("{ colKey: 'strategy'"))
  })

  // 游戏筛选只列游戏管理启用项；普通运营再与本人负责游戏取交集（他不负责的游戏
  // 本来就没有成片可读）。名称字典另取全量，停用游戏的历史成片仍显示名字。
  it('filters by an enabled game and sends game_id to the server', () => {
    expect(source).toContain('v-model="filters.game_id"')
    expect(source).toContain(':options="gameFilterOptions"')
    expect(source).toContain('if (filters.value.game_id) params.game_id = filters.value.game_id')
    expect(source).toContain("game_id: ''")
    expect(source).toContain('const gameFilterOptions = computed')
    expect(source).toContain('isAdmin.value || isSenior.value || own.has(game.id)')
    expect(source).toContain("client.listGames({ scope: 'all' })")
    expect(source).toContain("client.listGames({ scope: 'all', status: 'enabled' })")
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
