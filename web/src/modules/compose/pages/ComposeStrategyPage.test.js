import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./ComposeStrategyPage.vue', import.meta.url), 'utf8')

/**
 * 修改是「整份替换 + expected_version 防覆盖」（合同 PUT 信封）。
 *
 * 期望版本必须在**打开编辑**那一刻从行里取（row.current_version），不是提交那一刻再查
 * —— 表格里的行在对话框开着的时候可能已被别人改过，提交时再取读到的是新版本，
 * 防覆盖就形同虚设。
 */
describe('compose strategy page update semantics', () => {
  it('captures expected_version when the edit dialog opens', () => {
    expect(source).toContain('expected_version: row.current_version')
  })

  it('sends the update through the frozen envelope', () => {
    expect(source).toContain('await client.updateStrategy(editingID.value, form.value.expected_version, payload)')
    // 创建没有期望版本一说；创建路径不得捎带 expected_version。
    expect(source).toContain('await client.createStrategy(payload)')
    expect(source).toMatch(/createStrategy\(payload\)/)
  })

  // 服务端 DisallowUnknownFields + additionalProperties: false。payload 必须逐字段
  // 显式构造；`...form.value` 一散就会把 expected_version、模板选择器的中间态等
  // 非合同字段一起发出去，换来一个具名的 15201。
  it('builds the payload field by field instead of spreading the form', () => {
    expect(source).not.toMatch(/\.\.\.form\.value/)
    expect(source).not.toMatch(/\.\.\.row\b/)
  })
})

/**
 * 普通运营只读：列表照看，创建/编辑/复制不出现。判据来自 /auth/me 的 role，
 * 与服务端 visibleStrategy 的角色分支是同一套角色词（senior_operator / admin）。
 */
describe('compose strategy page role handling', () => {
  it('derives manage rights from the session role', () => {
    expect(source).toContain("me.value?.role === 'admin' || me.value?.role === 'senior_operator'")
  })

  it('guards every write action behind canManage', () => {
    // 新增按钮与行内写操作都必须在 v-if="canManage" 的分支里。
    const guarded = source.match(/v-if="canManage"/g) || []
    expect(guarded.length).toBeGreaterThanOrEqual(2)
    const opBlock = source.slice(source.indexOf('#op="{ row }"'), source.indexOf('</t-space>'))
    expect(opBlock).toContain('@click="openEdit(row)">编辑</t-button>')
    expect(opBlock).toContain('@click="openCopy(row)">复制</t-button>')
    // 只读侧的入口不设防：详情与版本是普通运营的合法操作。
    expect(opBlock).toContain('@click="openDetail(row)">详情</t-button>')
    expect(opBlock).toContain('@click="openVersions(row)">版本</t-button>')
  })
})

/**
 * 复制走服务端的 copy 端点（产生独立策略、版本回到 1），不是「把表单填好让用户再存一次」
 * —— 后者会把复制变成一次普通创建，丢掉「源策略与副本同版本起点」这个服务端事实。
 * 请求体只允许 {name}。
 */
describe('compose strategy page copy', () => {
  it('copies through the endpoint with only a name', () => {
    expect(source).toContain('await client.copyStrategy(copyTarget.value.id, copyName.value.trim())')
    expect(source).not.toMatch(/copyStrategy\([^)]*,\s*\{/)
  })
})

/** 校验与操作结果按反馈规范给出：字段错误靠近字段，操作结果用 toast。 */
describe('compose strategy page feedback', () => {
  it('shows field errors next to their fields', () => {
    expect(source).toContain("import FormError from '../../../shared/ui/feedback/FormError.vue'")
    expect(source).toContain('<FormError id="strategy-name-error" :message="fieldErrors.name" />')
    expect(source).toContain('<FormError id="strategy-game-error" :message="fieldErrors.game" />')
    expect(source).toContain('<FormError id="strategy-content-type-error" :message="fieldErrors.contentType" />')
    expect(source).toContain('<FormError id="strategy-ratio-error" :message="fieldErrors.ratio" />')
  })

  // 409（15204）是「别人已改，请重读」：具名处理后重拉列表，让用户拿新版本重开，
  // 而不是把「请求失败」四个字原样丢出去。
  it('names the version conflict and reloads the list for a fresh read', () => {
    expect(source).toContain('e.errcode === 15204')
    expect(source).toContain('e.errcode === 15201')
  })
})

/**
 * 表单的可选项来自模板目录接口（Cloud 维护的版本化目录），不是页面里写死的
 * basic_cut / 16:9。目录将来加模板或加比例，这页不用改一个字。
 */
describe('compose strategy page template catalog', () => {
  it('drives ratio and override options from the catalog', () => {
    expect(source).toContain('selectedTemplate')
    expect(source).toContain('selectedTemplate?.video_ratios')
    expect(source).toContain('selectedTemplate?.allowed_overrides')
    expect(source).not.toContain("'basic_cut'")
    expect(source).not.toContain("'16:9'")
    expect(source).not.toContain("'9:16'")
  })

  // 字幕/BGM/片尾在 1.0 目录里没有资源合同，服务端只接受 false：开关必须呈禁用态
  // 并说明原因，且 payload 里显式发 false。
  it('renders the asset-less switches disabled and always sends false', () => {
    expect(source).toContain('subtitle: false, bgm: false, outro: false')
    expect(source).toMatch(/<t-switch[^>]*:disabled="true"[^>]*\/>\s*<!--[^>]*-->/)
    expect(source).toContain('当前模板版本未提供')
  })
})

/**
 * 版本历史是只读的服务端事实（不可变快照、新→旧由服务端排序）。页面原样渲染，
 * 不在客户端重排 —— 重排一次，顺序就成了页面的说法而不是合同的说法。
 */
describe('compose strategy page versions', () => {
  it('renders versions as the endpoint returns them', () => {
    expect(source).toContain('await client.listVersions(row.id)')
    expect(source).not.toContain('.sort(')
  })
})
