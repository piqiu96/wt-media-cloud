import { readFileSync } from 'node:fs'
import { createSSRApp, h } from 'vue'
import { routerKey } from 'vue-router'
import { renderToString } from 'vue/server-renderer'
import { describe, expect, it, vi } from 'vitest'
import ContentPoolSourceDrawer from './ContentPoolSourceDrawer.vue'

// 模板里的 <t-drawer> 被 TDesign 解析器编译成对 tdesign-vue-next 的直接导入，
// app.component 注册不到它；而真实的 Drawer 在 SSR 下不产出任何节点（等挂载才渲染），
// 渲染结果会是空串。这里只把 Drawer 换成同名插槽转发的壳，其余组件照真实的走。
vi.mock('tdesign-vue-next', async (importOriginal) => {
  const actual = await importOriginal()
  const { h: create } = await import('vue')
  return {
    ...actual,
    Drawer: {
      name: 'TDesignDrawerStub',
      props: { visible: Boolean, header: String, size: String, closeBtn: Boolean },
      setup(props, { slots }) {
        return () => (props.visible ? create('section', [create('h3', props.header), slots.default?.(), slots.footer?.()]) : null)
      },
    },
  }
})

const source = readFileSync(new URL('./ContentPoolSourceDrawer.vue', import.meta.url), 'utf8')

// 页脚只在对应 id 存在时才给按钮：字符串断言看不出「人工来源会不会长出任务跳转」，
// 这里把组件真渲染一遍，顺带守住模板本身能编译。
async function render(props) {
  const app = createSSRApp({ render: () => h(ContentPoolSourceDrawer, props) })
  app.provide(routerKey, { push: () => {} })
  return renderToString(app)
}

// 内容池「来源」列的只读关联弹窗：列表只给策略名，策略与任务的关键字段在这里补齐。
// 任务由抽屉自己取（来源行只有 id 与名称），取不到就降级，不伪造字段。
describe('content pool source drawer', () => {
  it('opens as a read-only related drawer with an explicit footer', () => {
    expect(source).toContain('visible: { type: Boolean, default: false }')
    expect(source).toContain('source: { type: Object, default: null }')
    expect(source).toContain("emit('update:visible'")
    expect(source).toContain('header="来源详情"')
    expect(source).toContain('size="min(92vw, 560px)"')
    expect(source).toContain(':close-btn="true"')
    expect(source).toContain('<template #footer>')
    expect(source).not.toContain('>关闭</t-button>')
    expect(source).not.toContain('取消')
  })

  it('fetches the task itself and ignores a stale response', () => {
    expect(source).toContain('const taskID = Number(props.source?.crawl_task_id || 0)')
    expect(source).toContain('await client.getTask(taskID)')
    expect(source).toContain('if (seq !== requestSeq) return')
    expect(source).toContain('if (visible) loadTask()')
  })

  it('shows the strategy facts from the source row and the task snapshot', () => {
    expect(source).toContain("source.strategy_name || (source.strategy_id ? `策略 ${source.strategy_id}` : '-')")
    expect(source).toContain("if (snapshot().strategy_type === 'author') return '作者'")
    expect(source).toContain("if (snapshot().strategy_type === 'keyword') return '关键词'")
    expect(source).toContain("keywords.join('、') || snap.author || '-'")
    expect(source).toContain("snapshot().schedule || '-'")
    for (const label of ['策略名称', '类型', '关键词 / 作者', '执行周期']) {
      expect(source, label).toContain(`<dt>${label}</dt>`)
    }
  })

  it('shows the task facts with the crawl-task trigger wording', () => {
    expect(source).toContain("if (task.value.task_type === 'retry_failed_task') return '重试'")
    expect(source).toContain("return task.value.schedule_key ? '定时' : '手动'")
    for (const label of ['任务 ID', '触发方式', '状态', '创建时间', '完成时间', '结果概况']) {
      expect(source, label).toContain(`<dt>${label}</dt>`)
    }
    expect(source).toContain('发现 ${stats.found || 0} · 新增 ${stats.added || 0} · 待审核 ${stats.pending || 0}')
  })

  it('degrades to an error and a retry instead of faking task fields', () => {
    expect(source).toContain('task.value = null')
    expect(source).toContain("error.value = e?.message || '读取任务详情失败'")
    expect(source).toContain('<FeedbackAlert v-if="error" theme="error" :message="error" />')
    expect(source).toContain('@click="loadTask">重试</t-button>')
    expect(source).toContain('v-else-if="task"')
    // 加载中只说加载中，不先画一排 0。
    expect(source).toContain('任务详情加载中…')
  })

  it('links to the strategy and the task page only when the ids exist', () => {
    expect(source).toContain("router.push({ path: '/discovery-strategies', query: { strategy_id: props.source.strategy_id } })")
    expect(source).toContain("router.push({ path: '/crawl-tasks', query: { task_id: props.source.crawl_task_id } })")
    expect(source).toContain('v-if="source.strategy_id" theme="primary"')
    expect(source).toContain('v-if="source.crawl_task_id" class="wt-secondary-button" variant="outline"')
    // 关弹窗走 v-model，宿主列表不换路由，筛选与分页都留着。
    expect(source).toContain("emit('update:visible', false)")
  })

  it('renders only the strategy footer for a source that has no task id', async () => {
    const html = await render({ visible: true, source: { strategy_id: 38, strategy_name: '团队1热点', crawl_task_id: null } })
    expect(html).toContain('来源详情')
    expect(html).toContain('团队1热点')
    expect(html).toContain('前往策略页面')
    expect(html).not.toContain('前往任务页面')
    // 没有任务 id 就不去取任务，也不先画一排未知字段。
    expect(html).not.toContain('<dt>任务 ID</dt>')
    expect(html).not.toContain('任务详情加载中')
  })
})
