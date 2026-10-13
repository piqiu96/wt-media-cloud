import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./MyOutputsPage.vue', import.meta.url), 'utf8')

// CHG-20261013-084 T-04：我的成片页（列表/详情/预览/归档/去发布入口）。
// 与本仓其余列表页同一量法：钉住**会静默失效**的接线，和会把话说满的文案。
describe('my outputs page', () => {
  // 和合成任务页同一条教训：筛了但参数没上车，那条筛选就是摆设。
  it('loads the list through the server-side publish-status filter it renders', () => {
    expect(source).toContain('client.listComposeOutputs(params)')
    expect(source).toContain('if (filters.value.publish_status) params.publish_status = filters.value.publish_status')
    expect(source).toContain('v-model="filters.publish_status"')
  })

  it('renders the three publish statuses the contract names', () => {
    expect(source).toContain("pending_publish: '待发布'")
    expect(source).toContain("published: '已发布'")
    expect(source).toContain("archived: '已归档'")
  })

  // 归档只对待发布的行出现——非 pending_publish 的行连按钮都没有，而不是点了再被拒。
  it('offers archive only to pending outputs and asks before sending it', () => {
    expect(source).toContain("row.publish_status === 'pending_publish'")
    expect(source).toContain('client.archiveComposeOutput(')
    expect(source).toContain('DialogPlugin.confirm')
  })

  // 预览地址是短时签名（与素材 video-url 同边界）：每次打开抽屉现取，不缓存。
  it('fetches a fresh signed preview address when the drawer opens', () => {
    expect(source).toContain('client.getComposeOutputVideoUrl(')
    // 详情整体按 id 重取（列表行是加载那一刻的副本），预览地址跟着这次打开走。
    expect(source).toContain('client.getComposeOutput(')
  })

  // 第五章的「去发布」只把成片带入发布流程：不建 publication、不写发布事实，
  // 也不允许出现宣布翻转状态的调用名。
  it('hands the output to the publish flow without recording any publication', () => {
    expect(source).toContain("router.push({ name: 'Publish', query: { output_id")
    expect(source).not.toContain('publishComposeOutput')
    expect(source).not.toContain("publish_status = 'published'")
  })

  it('starts the first output download and opens the shared download centre', () => {
	  expect(source).toContain('await client.createOutputDownload(row.id)')
	  expect(source).toContain('downloadCentre.open()')
	  expect(source).toContain('feedback.toast.info(')
	  expect(source).toContain('@click="download(row)"')
	})

  // 操作列宽按待发布行最多的按钮清单推导（下载 loading 态多一个 spinner），不再写死 290。
  it('derives the op column width from the four pending-publish actions', () => {
    expect(source).toContain("opColumnWidth(['下载', '详情', '去发布', '归档'], { loading: true })")
    expect(source).not.toContain("width: 290, fixed: 'right'")
  })

  it('keeps its single status filter out of the equal-width grid with actions last', () => {
    const bar = source.slice(source.indexOf('class="filter-bar"'), source.indexOf('table-scroll-wrap'))
    expect(bar).not.toContain('<t-space')
    expect(bar.indexOf('wt-filter-actions')).toBeGreaterThan(bar.lastIndexOf('wt-filter-field'))
  })
})
