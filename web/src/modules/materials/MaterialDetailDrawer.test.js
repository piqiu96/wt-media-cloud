import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('./MaterialDetailDrawer.vue', import.meta.url), 'utf8')
const template = source.slice(source.indexOf('<template>'))

/** 取出两段定界文本之间的内容；找不到就抛，别让断言在空串上静默通过。 */
function sliceBetween(text, start, end) {
  const from = text.indexOf(start)
  const to = text.indexOf(end, from + start.length)
  if (from === -1 || to === -1) throw new Error(`找不到切片：${start} … ${end}`)
  return text.slice(from, to)
}

// 走查修正（CHG-20260930-069）：作者主页、平台原视频页、文件大小与云端视频地址
// 都在详情里。行内只剩识别信息。
describe('material detail drawer', () => {
  // 走查反馈：详情里的素材 ID 与列表同形，不带 # 前缀。
  it('shows the material id as a plain number', () => {
    expect(source).toContain('<dd>{{ material.id }}</dd>')
    expect(source).not.toContain('#{{ material.id }}')
  })

  it('opens the author home and the source page from the material body', () => {
    expect(source).toContain('material.author_home_url')
    expect(source).toContain('material.source_url')
  })

  it('shows the file size through the shared formatter', () => {
    expect(source).toContain('formatBytes(material.video_size_bytes)')
    expect(source).toContain("from '../../shared/utils/units.js'")
  })

  // 云端视频地址只在就绪时去取：未就绪的素材没有地址，一次必然 409 的请求
  // 不该发出去。
  it('asks for the cloud video address only when the material is ready', () => {
    expect(source).toContain('material.video_status === \'ready\'')
    expect(source).toContain('getVideoUrl(')
  })

  it('renders the cover with a placeholder when it is missing or fails to load', () => {
    expect(source).toContain('MaterialCover')
  })

  // 走查反馈（CHG-20260930-069）：来源行采集时的内容池统计以**独立区块**展现，不混进
  // dl 的字段里——它们是采集时刻的快照，不是素材的属性。抖音接口不提供播放数
  // （statistics.play_count 恒为 0，已对全部来源行核对），区块只展示拿得到数的四项；
  // view_count 字段仍在契约里，只是不在这里渲染。
  it('shows the source content-pool statistics as their own section', () => {
    expect(source).toContain('material.like_count')
    expect(source).toContain('material.favorite_count')
    expect(source).toContain('material.comment_count')
    expect(source).toContain('material.share_count')
    for (const label of ['点赞', '收藏', '评论', '分享']) {
      expect(template).toContain(label)
    }
    // 断言只看模板：脚本注释里「抖音不提供播放数」是这条决定本身的记录。
    expect(template).not.toContain('播放')
    expect(template).not.toContain('material.view_count')
  })

  // 走查三轮（交互对齐）：抽屉按上下文提供动作——library 只有「加入我的素材」，
  // mine 只有「下载/重试」。走查四轮补上 library 的「已加入」分支：那时主操作是
  // 「去我的素材」，不再给一个必然幂等空转的「加入我的素材」。
  it('offers the actions of its context only', () => {
    expect(source).toContain("mode: { type: String, default: 'library' }")
    expect(source).toMatch(/v-if="mode === 'mine'"/)
    expect(source).toMatch(/mode === 'library' && !mine/)
    expect(source).toMatch(/mode === 'library' && mine/)
    // mine 上下文的主操作文案由状态驱动：failed 是「重试」，其余是「下载」。
    expect(source).toContain("$emit('download', material)\">{{ downloadActionLabel(material.video_status) }}")
  })

  // 走查四轮（2026-09-30 用户带设计图）：正文按「概览 / 文件信息 / 来源信息」分标签，
  // 而不是把十几行 dl 平铺——平铺的结局是「入库时间」和「72313 个赞」读成同一类事实。
  it('organises the body into 概览 / 文件信息 / 来源信息 tabs', () => {
    expect(template).toContain('<t-tabs')
    expect(template).toContain('<t-tab-panel')
    for (const label of ['概览', '文件信息', '来源信息']) {
      expect(template, `缺少 ${label} 标签`).toContain(`label="${label}"`)
    }
    // 基本信息留在概览；文件事实与来源事实各自归位。
    expect(template).toContain('基本信息')
    expect(template).toContain('来源内容池统计')
  })

  /**
   * 走查五轮（2026-09-30 用户走查）：「详情里的展示框效果远不如预期」。用户给的参照是
   * **内容池详情的框**（ContentPoolPage 的 .detail-panel 一族）：每一段信息是一张有边界的
   * 卡片，不是一个把十几行 dt/dd 平铺下去的长条。
   *
   * 这条断言量到 CSS，不只量类名——类名是零成本的，样式才是用户看得见的那一半。
   */
  it('frames every section as a bordered card, the way the content-pool detail does', () => {
    expect(template).toContain('detail-hero')
    expect(template).toContain('detail-card')
    // 设计图里的四个框 + 概览里的使用情况框；每一段的标题都挂在这张卡上。
    // 量 `class="detail-card__title"` 而不是裸类名：`template` 一路切到文件尾，裸类名
    // 会把 <style> 里那条定义也算进来，5 个框会数成 6。
    expect(template.match(/class="detail-card__title"/g) ?? []).toHaveLength(5)
    const style = source.slice(source.indexOf('<style'))
    expect(style, '类名写了但样式没定义就只是一句说法').toContain('.detail-card {')
    expect(style).toMatch(/\.detail-card \{[^}]*border: 1px solid var\(--wt-border\)/)
    expect(style).toContain('.detail-hero {')
    expect(style).toContain('.detail-source-line {')
  })

  // 设计图把「使用情况：成片数、发布数、最近时间、重复风险」放在概览的最上面，
  // 基本信息在它下面。两者都是卡片。
  it('opens the 概览 tab with the usage card, then 基本信息', () => {
    const overview = sliceBetween(template, 'value="overview"', 'value="file"')
    expect(overview).toContain('使用情况')
    expect(overview).toContain('usageFacts(material)')
    expect(overview.indexOf('使用情况')).toBeLessThan(overview.indexOf('基本信息'))
  })

  // 互动数据是来源行采集时的快照，跟着「来源」走：设计图把它画在来源信息里，
  // 而不是继续留在概览。
  it('keeps the source content-pool statistics inside the 来源信息 tab', () => {
    const sourceTab = sliceBetween(template, 'value="source"', '</t-tabs>')
    expect(sourceTab).toContain('来源内容池统计')
    expect(sourceTab).toContain('material.like_count')
    const overview = sliceBetween(template, 'value="overview"', 'value="file"')
    expect(overview).not.toContain('来源内容池统计')
  })

  // 规范 §7.2：两个状态维度并排，不能为了页面简单把「已暂停 + 可下载」压成一个词。
  // 顶部就说清「这条素材还能不能选」和「它的文件好了没有」。
  it('heads the drawer with both status dimensions', () => {
    const hero = sliceBetween(template, 'detail-hero', 'detail-tabs')
    expect(hero).toContain('v-if="material.status"')
    expect(hero).toContain('materialStatusLabel(material.status)')
    expect(hero).toContain('videoStatusLabel(material.video_status)')
    expect(hero).toContain('已加入我的素材')
  })

  // 规范 §6.3：详情不出现无意义的「确认 / 取消」，关闭走统一的关闭动作。
  //
  // 但这一条**证明不了**底下那对按钮不在——走查五轮就是这么被漏掉的：用户看到的
  // 「取消 / 确认」根本不是本组件写的，而是 TDesign Drawer 的 footer 默认值
  // （props.footer 默认 true，不给插槽就渲染 getDefaultFooter()）。源码字符串断言
  // 看不见一个由组件库注入的默认页脚，所以下面这条一直是绿的。
  it('closes with 关闭 instead of a meaningless 确认 / 取消 pair', () => {
    expect(template).toContain('>关闭</t-button>')
    expect(template).not.toContain('>确认</t-button>')
    expect(template).not.toContain('>取消</t-button>')
    expect(template).not.toContain('>新增</t-button>')
    expect(template).not.toContain('>保存</t-button>')
  })

  // 走查五轮（2026-09-30 用户走查）：底部不该有「取消 / 确认」，且动作条要钉在
  // 抽屉底部、不随标签内容高度上下跳。两件事同一个解法：接管 footer 插槽。
  it('takes over the drawer footer so the framework default pair never renders', () => {
    // 要么给 #footer 插槽、要么显式 :footer="false"；两条都不做就会长出默认那对。
    expect(
      template.includes('<template #footer>') || /:footer="(false|reviewMode)?"/.test(template),
      'TDesign 的 footer 默认是 true，不接管就渲染取消/确认',
    ).toBe(true)
  })

  it('pins the context actions into that footer instead of the scrolling body', () => {
    const footer = sliceBetween(template, '<template #footer>', '</template>')
    expect(footer).toContain('material-detail__actions')
    expect(footer).toContain('>关闭</t-button>')
    // 三颗上下文主操作都在页脚里，位置不随标签切换而移动。
    expect(footer).toContain('加入我的素材')
    expect(footer).toContain('去我的素材')
    expect(footer).toContain("$emit('download', material)")
    // 正文里不再留一份动作条——留一份就是两处按钮，一处在滚动区里。
    const body = template.slice(0, template.indexOf('<template #footer>'))
    expect(body).not.toContain('material-detail__actions')
  })

  // 走查五轮（设计图）：页脚左边「关闭」、右边主操作，两者撑满整条页脚。
  it('splits the footer into a 关闭 on the left and the context action on the right', () => {
    const footer = sliceBetween(template, '<template #footer>', '</template>')
    const closeAt = footer.indexOf('>关闭</t-button>')
    expect(closeAt).toBeGreaterThan(-1)
    expect(footer.indexOf('加入我的素材')).toBeGreaterThan(closeAt)
    expect(footer.indexOf('去我的素材')).toBeGreaterThan(closeAt)
    expect(source).toMatch(/\.material-detail__actions \{[^}]*justify-content: space-between/)
  })

  // 顶部把「这是什么」（来源副行 + 状态徽章）一次说清，正文才用于解释细节。
  it('heads the drawer with the source line and the status badges', () => {
    expect(template).toContain('material.author_name')
    expect(source).toContain('gameName(games, material.game_id)')
    expect(template).toContain('已加入我的素材')
    expect(source).toContain("mine: { type: Boolean, default: false }")
  })
})
