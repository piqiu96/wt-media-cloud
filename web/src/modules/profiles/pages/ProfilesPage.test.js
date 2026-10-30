import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ProfilesPage.vue', import.meta.url), 'utf8')

describe('browser window resource page', () => {
  it('composes the shared resource presentation primitives', () => {
    expect(source).toContain('ResourcePageHeader')
    expect(source).toContain('ResourceCard')
    expect(source).toContain('ResourceStatGrid')
    expect(source).toContain('ResourceStatusBadge')
    expect(source).toContain('OperationProgressDialog')
    expect(source).toContain('wt-resource-table')
  })

  it('limits local capability to operators on desktop and keeps every other view read-only', () => {
    expect(source).toContain('const isOwnDesktop = computed(() => isDesktopClient.value && (isOperator.value || (isSenior.value && !viewingMember.value)))')
    expect(source).toContain('? ["详情", "打开", "编辑", "处理异常", "运营备注"]')
    expect(source).toContain(': ["详情", "运营备注"]')
    expect(source).toContain('if (isOwnDesktop.value) list.push({ colKey: "row-select", type: "multiple"')
    expect(source).toContain('disabled: ({ row }) => !canSelectWindow(row)')
    expect(source).toContain('if (isAdmin.value) list.push({ colKey: "team", title: "运营团队", width: 120 })')
    expect(source).toContain('<ResourceStatusBadge\n                v-else')
    expect(source).toContain('if (!isOwnDesktop.value) return')
  })

  it('sends scoped filters to the backend and pages the returned set', () => {
    expect(source).toContain('bindingClient.listProfiles(filters)')
    expect(source).toContain('if (isSenior.value) filters.user_id = memberFilter.value || selfId.value')
    expect(source).not.toContain('local_status')
    expect(source).not.toContain('business_status')
    expect(source).not.toContain('proxy_protocol')
    expect(source).toContain('const pagedProfiles = computed(')
    expect(source).toContain('<t-pagination')
  })

  it('keeps filter labels visible and separates draft inputs from applied filters', () => {
    expect(source).toContain('>搜索</span>')
    expect(source).toContain('>授权运营</span>')
    expect(source).toContain('>代理方式</span>')
    expect(source).toContain('>业务状态</span>')
    expect(source).toContain('>运行状态</span>')
    expect(source).toContain('@click="submitSearch"')
    expect(source).toContain('@click="resetFilters"')
    expect(source).toContain('const filterDraft = ref(emptyFilters())')
    expect(source).toContain('const appliedFilters = ref(emptyFilters())')
  })

  it('keeps the operation column pinned and width from the shared measure', () => {
    expect(source).toContain('{ colKey: "op", title: "操作", width: opColumnWidth(rowActionLabels.value, { variant: "text", loading: true, primary: isOwnDesktop.value }), fixed: "right" }')
  })

  it('places window status before running state and labels deletion at both entry and confirmation', () => {
    const columns = source.slice(source.indexOf('const columns = computed('), source.indexOf('const tableScroll = computed('))
    expect(columns.indexOf('colKey: "window_state"')).toBeLessThan(columns.indexOf('colKey: "running"'))
    const filters = source.slice(source.indexOf('<div class="filter-bar">'), source.indexOf('class="wt-resource-table"'))
    expect(filters.indexOf('>窗口状态</span>')).toBeLessThan(filters.indexOf('>运行状态</span>'))
    expect(source).toContain('>删除</t-button>')
    expect(source).not.toContain('>删除记录</t-button>')
    expect(source).toContain('class="is-danger"')
  })

  it('keeps historical missing windows actionable in sync without reporting them as new issues', () => {
    expect(source).toContain('const syncHistoricalMissingProfiles = computed(')
    expect(source).toContain("{ key: \"history\", label: \"历史待处理\"")
    expect(source).toContain('本次没有新增异常。')
    expect(source).toContain('另有 {{ syncHistoricalMissingProfiles.length }} 个历史未处理的缺失窗口。')
    expect(source).toContain('v-if="!syncHasChanges && syncHistoricalMissingProfiles.length"')
    expect(source).toContain('@click="syncDeleteAction(row)">删除</t-button>')
  })

  it('requires a fresh running-state read before disabling a window', () => {
    const body = source.slice(source.indexOf('async function requestEnabledChange('))
    const disable = body.slice(0, body.indexOf('enabledTarget.value = profile'))
    expect(disable).toContain('refreshRunningStates({ silent: false })')
    expect(disable).toContain('state === true')
    expect(disable).toContain('state === undefined')
    expect(disable).toContain('profile.local_state !== "active"')
  })

  it('enables batch actions only for a consistent running state', () => {
    expect(source).toContain('const batchOpenReady = computed(() => selectedProfiles.value.length > 0 && selectedProfiles.value.every((profile) => rowOperable(profile) && runningState(profile) === false))')
    expect(source).toContain('const batchCloseReady = computed(() => selectedProfiles.value.length > 0 && selectedProfiles.value.every((profile) => rowOperable(profile) && runningState(profile) === true))')
    expect(source).toContain('>批量设置代理</t-button>')
    expect(source).toContain('>批量打开</t-button>')
    expect(source).toContain('>批量关闭</t-button>')
  })

  it('defaults every discovered window to unclaimed and disables existing owners', () => {
    expect(source).toContain('syncSelectedAdded.value = []')
    expect(source).toContain(':disabled="Boolean(row.ownership_conflict)"')
    expect(source).toContain('const syncSelectableAdded = computed(() => syncAddedProfiles.value.filter((profile) => !profile.ownership_conflict))')
    expect(source).toContain('勾选后建立关联，已归属其他运营的窗口不可选择。')
  })

  it('accepts one user-scoped sync without a separate proxy scan stage', () => {
    expect(source).toContain('confirm: (id, selection) => bindingClient.confirm(id, selection)')
    expect(source).not.toContain('proxyClient.confirmLocalScan')
    expect(source).not.toContain('proxyClient.previewLocalScan')
  })

  it('never retries a create that may already have happened', () => {
    const body = source.slice(source.indexOf('async function submitCreate('))
    expect(body).toContain('if (!snapshot) {')
    expect(body).toContain('item.status = "unconfirmed"')
    expect(body).toContain('请先同步窗口确认，不要重复创建')
    expect(source).toContain('const createUncertain = ref(false)')
  })

  it('drives proxy changes through Cloud intent plus the Desktop Agent bridge', () => {
    expect(source).toContain('proxyClient.operate(')
    expect(source).toContain('localAgent.profileProxyOperation(')
    expect(source).toContain('proxyClient.finishOperation(')
    expect(source).not.toContain('proxyClient.assign(')
    expect(source).not.toContain('proxyClient.unbind(')
  })

  it('uses real Agent and Cloud results instead of prototype timers or mock data', () => {
    expect(source).not.toContain('setTimeout')
    expect(source).not.toContain('setInterval')
    expect(source).not.toContain('mockProfiles')
    expect(source).not.toContain('Math.random')
  })

  it('automatically reads once on entering the owner view, without extra scans on metadata reload or member change handlers', () => {
    const watcher = source.slice(source.indexOf('watch(isOwnDesktop'), source.indexOf('onMounted('))
    expect(watcher).toContain('if (value) refreshRunningStates({ silent: false })')
    expect(watcher).toContain('runningReader.invalidate()')
    const reload = source.slice(source.indexOf('async function reloadAll()'), source.indexOf('function pruneSelection()'))
    expect(reload).not.toContain('refreshRunningStates')
    const memberChange = source.slice(source.indexOf('function onSeniorMemberChange()'), source.indexOf('function onAdminTeamChange()'))
    expect(memberChange).not.toContain('refreshRunningStates')
    const sync = source.slice(source.indexOf('const syncWorkflow ='), source.indexOf('function toggleSyncAdded'))
    expect(sync).toContain('runningReader.read({ notifyError: false })')
    expect(sync).not.toContain('localAgent.profileScan()')
  })

  it('puts the only running-state refresh in the page actions and disables unknown row actions', () => {
    const header = source.slice(source.indexOf('<ResourcePageHeader'), source.indexOf('</ResourcePageHeader>'))
    expect(header).toContain('>刷新运行状态</t-button>')
    expect(header).toContain('@click="refreshRunningStates({ announce: true })"')
    expect(source).toContain('{ colKey: "running", title: "运行状态", width: 100 }')
    expect(source).not.toContain('<template #runningHeader>')
    expect(source).toContain(':disabled="rowOperable(row) === false || runningState(row) === undefined || runningLoading"')
    const readFailure = source.slice(source.indexOf('const runningReader ='), source.indexOf('function applyRunningSnapshot('))
    expect(readFailure).toContain('runningStates.value = {}')
    expect(readFailure).toContain('feedback.toast.error(')
    expect(readFailure).not.toContain('FeedbackAlert')
  })

  it('uses a compact dialog for a missing window and hides the empty account table', () => {
    const dialog = source.slice(source.indexOf('<ResourceOverlay v-model:visible="missingVisible"'), source.indexOf('<ResourceOverlay v-model:visible="mismatchVisible"'))
    expect(dialog).toContain('kind="dialog" :width="640"')
    expect(dialog).toContain('class="missing-summary-grid"')
    expect(dialog).toContain('class="missing-account-list"')
    expect(dialog).not.toContain('<t-table')
    expect(dialog).toContain('v-if="missingAccounts.length"')
    expect(dialog).toContain('v-else class="missing-empty-accounts"')
    expect(dialog).not.toContain('重要提示')
  })

  it('keeps the read-only detail drawer footerless and the progress dialog reachable', () => {
    expect(source).toContain('v-model:visible="detailVisible" title="窗口详情" :footer="false" kind="drawer" :width="800"')
    expect(source).toContain(':footer="true"')
    expect(source).toContain('@retry="retryFailed"')
    expect(source).toContain('@verify="verifyPending"')
    expect(source).toContain('@done="closeProgress"')
  })
})
