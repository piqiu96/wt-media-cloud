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
    expect(source).toContain('if (isOwnDesktop.value) list.push({ colKey: "row-select", type: "multiple", width: 48 })')
    expect(source).toContain('if (isAdmin.value) list.push({ colKey: "team", title: "运营团队", width: 120 })')
    expect(source).toContain('<ResourceStatusBadge\n                v-else')
    expect(source).toContain('if (!isOwnDesktop.value) return')
  })

  it('sends scoped filters to the backend and pages the returned set', () => {
    expect(source).toContain('bindingClient.listProfiles(filters)')
    expect(source).toContain('if (isSenior.value) filters.user_id = memberFilter.value || selfId.value')
    expect(source).not.toContain('local_status: statusFilter.value')
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
    expect(source).toContain('{ colKey: "op", title: "操作", width: opColumnWidth(rowActionLabels.value, { variant: "text", loading: true }), fixed: "right" }')
  })

  it('requires a fresh running-state read before disabling a window', () => {
    const body = source.slice(source.indexOf('async function requestBusinessChange('))
    const disable = body.slice(0, body.indexOf('businessTarget.value = profile'))
    expect(disable).toContain('refreshRunningStates({ silent: false })')
    expect(disable).toContain('state === true')
    expect(disable).toContain('state === undefined')
    expect(disable).toContain('profile.local_status !== "active"')
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
    expect(source).toContain('本机发现但 Cloud 尚无关联的窗口。勾选后才会建立关联；不勾选不会认领，其他运营已登记的窗口不可选择。')
  })

  it('accepts a scan through two explicit stages instead of one combined action', () => {
    expect(source).toContain('await bindingClient.confirm(syncScan.value.id, syncSelectedAdded.value)')
    expect(source).toContain('await proxyClient.confirmLocalScan(syncScan.value.id, syncSelectedUnknown.value)')
    expect(source).toContain('async function retryProxyStage()')
    expect(source).toContain('窗口变化已接受，但代理关系核对未完成。重新同步会以本机实际状态继续核对。')
  })

  it('never retries a create that may already have happened', () => {
    const body = source.slice(source.indexOf('async function submitCreate('))
    expect(body).toContain('if (!snapshot) {')
    expect(body).toContain('item.status = "unconfirmed"')
    expect(body).toContain('请先同步窗口确认，不要重复创建')
    expect(source).toContain('const createUncertain = ref(false)')
  })

  it('drives proxy changes through the server-side scan endpoints', () => {
    expect(source).toContain('proxyClient.previewLocalScan(')
    expect(source).toContain('proxyClient.clearProfileProxy(')
    expect(source).toContain('proxyClient.release(')
    expect(source).toContain('proxyClient.unbind(')
  })

  it('uses real Agent and Cloud results instead of prototype timers or mock data', () => {
    expect(source).not.toContain('setTimeout')
    expect(source).not.toContain('setInterval')
    expect(source).not.toContain('mockProfiles')
    expect(source).not.toContain('Math.random')
  })

  it('keeps the read-only detail drawer footerless and the progress dialog reachable', () => {
    expect(source).toContain('v-model:visible="detailVisible" title="窗口详情" :footer="false" kind="drawer" :width="800"')
    expect(source).toContain(':footer="syncDone || syncHasChanges"')
    expect(source).toContain('@retry="retryFailed"')
    expect(source).toContain('@verify="verifyPending"')
    expect(source).toContain('@done="closeProgress"')
  })
})
