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
    const titles = [...template.matchAll(/class="detail-card__title"[^>]*>\s*([^<]+)/g)]
      .map((m) => m[1].trim())
    expect(titles).toEqual(['当前进度 / 使用情况', '文件信息', '来源信息', '来源内容池统计', '基本信息'])
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

  /**
   * 走查七轮（用户提示词第十节「详情信息顺序」）：第一段回答「我现在处理到哪一步，
   * 下一步是什么」，字段往后放。
   *
   * 「基本信息」剩下的三行（素材 ID / 游戏 / 入库时间）**没有删**，挪到最后一段：
   * 提示词那四段是「建议」，而没有一段提到这两样东西该去哪，直接删掉就是一次用户没
   * 裁定过的信息损失 —— 素材 ID 还是运营要复制去别处查的那串数字。顺序上官方那三段
   * 紧跟第一段，与提示词列出的次序一致。
   */
  it('opens the body with 当前进度 / 使用情况 and keeps the rest in the prompt order', () => {
    const body = sliceBetween(template, 'detail-workspace', '<template #footer>')
    expect(body).toContain('nextStepHint(material)')
    expect(body).toContain('usageFacts(material)')
    // 量标题标记而不是裸词：裸词会先量到正文注释里的那一次，读数就成了注释的函数。
    const cards = [...body.matchAll(/<h4 class="detail-card__title">\s*([^<\n]*)/g)]
      .map((m) => ({ title: m[1].trim(), at: m.index }))
    const at = (title) => cards.find((card) => card.title === title)?.at ?? -1
    expect(at('当前进度 / 使用情况')).toBeGreaterThan(-1)
    expect(at('基本信息')).toBeGreaterThan(at('来源内容池统计'))
    expect(at('当前进度 / 使用情况')).toBeLessThan(at('文件信息'))
    expect(at('文件信息')).toBeLessThan(at('来源信息'))
    expect(at('来源信息')).toBeLessThan(at('来源内容池统计'))
  })

  // 第一段的四件事：两个状态各一颗徽章、加入时间、下一步。
  it('states the file state, the usage state, the join time and the next step', () => {
    const card = sliceBetween(template, 'class="detail-card__title">当前进度 / 使用情况', '<h4 class="detail-card__title">')
    expect(card).toContain('videoStatusLabel(material.video_status)')
    expect(card).toContain('usageStatusLabel(material.usage_status)')
    expect(card).toContain('formatDateTime(material.added_at)')
    expect(card).toContain('nextStepHint(material)')
    // 加入时间是关系自己的时间，素材库上下文没有它——缺值画 —，不是画入库时间冒充。
    expect(card).toContain("material.added_at ? formatDateTime(material.added_at) : '—'")
  })

  // 使用状态读不到画 —：素材库上下文打开的详情就属于这种，而那不等于「使用中」。
  it('does not paint 使用中 on a relation it cannot read', () => {
    const card = sliceBetween(template, 'class="detail-card__title">当前进度 / 使用情况', '<h4 class="detail-card__title">')
    expect(card).toContain('v-if="material.usage_status"')
    expect(card).toMatch(/<template v-else>—<\/template>/)
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
    // 切的终点是抽屉自己那个收尾标记，不是第一个 `</template>`：走查七轮的页脚里
    // 有嵌套的 <template v-if>，按 `</template>` 切会把动作条从第一个分支处剪断。
    const footer = sliceBetween(template, '<template #footer>', '</template>\n  </t-drawer>')
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

  /**
   * 走查七轮（用户提示词第九节）：顶部用小型 Badge，不用横跨整页的状态色块。
   *
   * 使用状态徽章只在读得到时出：素材库上下文打开的这个抽屉没有关系那一维，
   * 画一颗「使用中」就是替服务端宣布了一条它没说过的关系。
   */
  it('adds the usage badge to the small badge row, without a full-width status bar', () => {
    const hero = sliceBetween(template, 'detail-hero', 'detail-card')
    expect(hero).toContain('usageStatusLabel(material.usage_status)')
    expect(hero).toContain('v-if="material.usage_status"')
    // 三颗徽章都在同一个 flex 行里——「不是色块」的判据是它们还在那个容器里。
    expect(source).toMatch(/\.detail-badges \{[^}]*display: flex/)
  })

  /**
   * 页脚按状态给真实业务动作（用户提示词第十一节）。「已放弃」只给恢复：一颗同时
   * 出现的「下载」会让运营以为那条关系还通着。
   *
   * 「加入合成」今天跳 `/compose`（ComingSoon），和行操作走同一个判断——详情与列表
   * 对同一个素材给出不同的下一步，是这两处各写一份的直接后果。
   */
  it('gives the footer the action the relation state calls for', () => {
    const footer = sliceBetween(template, '<template #footer>', '</template>\n  </t-drawer>')
    expect(footer).toContain("$emit('restore', material)")
    expect(footer).toContain('>恢复使用</t-button>')
    expect(footer).toContain("$emit('give-up', material)")
    expect(footer).toContain('>放弃使用</t-button>')
    expect(footer).toContain("$emit('compose', material)")
    expect(footer).toContain("$emit('redownload', material)")
    // 恢复与放弃互斥，下载与放弃互斥：同一个素材不能同时有两条相反的出路。
    expect(footer).toContain("material.usage_status === 'removed'")
    expect(footer).toContain("material.video_status === 'ready'")
  })
})
