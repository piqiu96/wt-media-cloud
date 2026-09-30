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

describe('material detail drawer', () => {
  // 素材 ID 与列表同形，不带 # 前缀。
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

  // 抖音接口不提供播放数（play_count 恒为 0，已对全部来源行核对），只展示拿得到数的四项；
  // view_count 仍在契约里。
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

  // 抽屉按上下文提供动作；已加入的行给「去我的素材」，不再给必然空转的「加入」。
  it('offers the actions of its context only', () => {
    expect(source).toContain("mode: { type: String, default: 'library' }")
    expect(source).toMatch(/v-if="mode === 'mine'"/)
    expect(source).toMatch(/mode === 'library' && !mine/)
    expect(source).toMatch(/mode === 'library' && mine/)
    // mine 上下文的主操作文案由状态驱动：failed 是「重试」，其余是「下载」。
    expect(source).toContain("$emit('download', material)\">{{ downloadActionLabel(material.video_status) }}")
  })

  // 分屏的门槛是信息真的填满多屏，不是「信息可以分成三类」。
  it('lays the sections out flat instead of behind tabs', () => {
    expect(template).not.toContain('<t-tabs')
    expect(template).not.toContain('<t-tab-panel')
    // 标签状态一起消失：留一个没人读的 ref 就是留一段会腐坏的死代码。
    expect(source).not.toContain('activeTab')
    // 取标题的标记而不是裸词：裸词会先量到注释里的那一次，直接判反。
    const titles = [...template.matchAll(/class="detail-card__title">\s*([一-龥]+)/g)].map((m) => m[1])
    expect(titles).toEqual(['使用情况', '基本信息', '文件信息', '来源信息', '来源内容池统计'])
  })

  // 量到 CSS 而不只量类名：类名是零成本的，样式才是用户看得见的那一半。
  it('frames every section as a bordered card, the way the content-pool detail does', () => {
    expect(template).toContain('detail-hero')
    expect(template).toContain('detail-card')
    // 量带引号的类名而不是裸类名：template 切到文件尾，裸类名会把 <style> 里的定义也算进来。
    expect(template.match(/class="detail-card__title"/g) ?? []).toHaveLength(5)
    const style = source.slice(source.indexOf('<style'))
    expect(style, '类名写了但样式没定义就只是一句说法').toContain('.detail-card {')
    expect(style).toMatch(/\.detail-card \{[^}]*border: 1px solid var\(--wt-border\)/)
    expect(style).toContain('.detail-hero {')
    expect(style).toContain('.detail-source-line {')
  })

  // 使用情况在最上面，基本信息在它下面。
  it('opens the body with the usage card, then 基本信息', () => {
    const body = sliceBetween(template, 'detail-workspace', '<template #footer>')
    expect(body).toContain('usageFacts(material)')
    expect(body.indexOf('class="detail-card__title">使用情况'))
      .toBeLessThan(body.indexOf('class="detail-card__title">基本信息'))
  })

  // 互动数据是来源行采集时的快照，跟着「来源」走。
  it('keeps the source content-pool statistics right after the 来源信息 card', () => {
    const body = sliceBetween(template, 'detail-workspace', '<template #footer>')
    expect(body).toContain('material.like_count')
    expect(body.indexOf('class="detail-card__title">来源内容池统计'))
      .toBeGreaterThan(body.indexOf('class="detail-card__title">来源信息</h4>'))
  })

  // 规范 §7.2：两个状态维度并排，不能为了页面简单把「已暂停 + 可下载」压成一个词。
  // 顶部就说清「这条素材还能不能选」和「它的文件好了没有」。
  it('heads the drawer with both status dimensions', () => {
    const hero = sliceBetween(template, 'detail-hero', 'detail-card')
    expect(hero).toContain('v-if="material.status"')
    expect(hero).toContain('materialStatusLabel(material.status)')
    expect(hero).toContain('videoStatusLabel(material.video_status)')
    expect(hero).toContain('已加入我的素材')
  })

  // 规范 §6.3：详情不出现无意义的「确认 / 取消」。
  //
  // 这一条证明不了那对按钮不在：用户看到的「取消 / 确认」是 TDesign Drawer 的
  // footer 默认值（props.footer 默认 true），源码字符串断言看不见组件库注入的页脚。
  it('closes with 关闭 instead of a meaningless 确认 / 取消 pair', () => {
    expect(template).toContain('>关闭</t-button>')
    expect(template).not.toContain('>确认</t-button>')
    expect(template).not.toContain('>取消</t-button>')
    expect(template).not.toContain('>新增</t-button>')
    expect(template).not.toContain('>保存</t-button>')
  })

  // 接管 footer 插槽：既挡掉默认那对按钮，又让动作条钉在底部不随正文高度跳。
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
    // 三颗上下文主操作都在页脚里，位置不随正文高度移动。
    expect(footer).toContain('加入我的素材')
    expect(footer).toContain('去我的素材')
    expect(footer).toContain("$emit('download', material)")
    // 正文里不再留一份动作条——留一份就是两处按钮，一处在滚动区里。
    const body = template.slice(0, template.indexOf('<template #footer>'))
    expect(body).not.toContain('material-detail__actions')
  })

  // 页脚左边「关闭」、右边主操作，两者撑满整条页脚。
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

  // 列表里的标题早就是外链，详情里的标题不能是一段死文本：同一个对象两处两种可点性。
  it('links the hero title to the source landing page', () => {
    const hero = sliceBetween(template, 'detail-hero', 'detail-card')
    expect(hero).toContain('<a v-if="material.source_url"')
    expect(hero).toContain(':href="material.source_url"')
    expect(hero).toContain("material.title || '未命名素材'")
    // 没有落地页的素材退回普通文本：链接形状留给真的能点的东西。
    expect(hero).toContain('<span v-else>')
  })

  // 作者本来就在副行与来源信息里，缺的是可点性。
  it('makes the author in the hero clickable too', () => {
    const hero = sliceBetween(template, 'detail-hero', 'detail-card')
    expect(hero).toContain('material.author_home_url')
    expect(hero).toContain('material.author_name')
  })

  // 游戏在列表里有列、在正文里有行，再在副行里说第三遍就是重复。
  it('keeps 游戏 out of the hero source line', () => {
    const hero = sliceBetween(template, 'detail-hero', 'detail-card')
    const line = sliceBetween(hero, 'detail-source-line', '</p>')
    expect(line).not.toContain('gameName')
    expect(line).toContain('material.platform')
  })
})
