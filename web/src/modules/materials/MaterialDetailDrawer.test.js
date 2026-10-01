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

  // 云端视频地址是签名的、会过期：只在点击那一刻去取，不在打开抽屉时预取。
  it('asks for the cloud video address on the click, not when the drawer opens', () => {
    expect(source).toContain('handOverCloudVideo(')
    expect(source).toContain('@click.prevent="openCloudVideo"')
    // 未就绪的素材没有地址，一次必然 409 的请求不发出去 —— 链接本身就不渲染。
    expect(source).toContain('v-if="material.video_status === \'ready\'"')
  })

  // 两个宿主的交付方式不同（浏览器自己开标签页、桌面端由壳交给系统浏览器），判在
  // `cloudVideo.js` 里，由它自己的用例逐条钉住。抽屉要回答的是**现在跑在哪个宿主
  // 里**——此前这一问漏了，桌面端被当成浏览器，于是弹出「浏览器拦截了新标签页」。
  it('asks the running host before deciding how the video is opened', () => {
    const body = source.slice(source.indexOf('function openCloudVideo'))
    expect(body).toContain('isDesktopRuntime()')
    expect(body).toContain('props.material.id')
    expect(body).toContain('handOverCloudVideo({')
    // 窗口怎么开也不由这里决定：抽屉里不该再有窗口 API。
    expect(body).not.toContain('window.open(')
  })

  // 预取留下的那两个形状不该有残留：一个 ref、一段「地址获取中」的中间态。
  it('keeps no held address and no fetching placeholder', () => {
    expect(source).not.toContain('videoUrl')
    expect(source).not.toContain('地址获取中')
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
    // mine 上下文的主操作文案由下载状态驱动（任务 23）：failed 是「重新下载」，其余是「下载」。
    expect(source).toContain("$emit('download', material)\">{{ downloadActionLabel(downloadStatus) }}")
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
  // 文件状态徽章是「下载状态优先、video_status 兜底」的组合（与我的素材页同语义），
  // 单测钉在组合函数上（下方「derives the file state」组）。
  it('states the file state, the usage state, the join time and the next step', () => {
    const card = sliceBetween(template, 'class="detail-card__title">当前进度 / 使用情况', '<h4 class="detail-card__title">')
    expect(card).toContain('fileStatus.tone')
    expect(card).toContain('fileStatus.label')
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
    expect(hero).toContain('fileStatus.tone')
    expect(hero).toContain('fileStatus.label')
    expect(hero).toContain('已加入我的素材')
  })

  // 规范 §6.3：关闭走抽屉右上角的 ×（2026-09-30 用户裁定），页脚不再放「关闭」。
  //
  // 这几条证明不了「取消 / 确认」不在：用户看到的那一对是 TDesign Drawer 的 footer
  // 默认值（props.footer 默认 true），源码字符串断言看不见组件库注入的页脚——挡住它的
  // 是下面「接管页脚」那条。
  it('leaves closing to the header × and writes no 关闭 into the footer', () => {
    // × 不是默认带上的：tdesign-vue-next 1.20.3 的 drawer `closeBtn` 没有 default，
    // 不写这颗属性抽屉就没有关闭入口（见 drawerFooterConvention.test.js）。
    expect(template).toContain(':close-btn="true"')
    expect(template).not.toContain('>关闭</t-button>')
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
    expect(footer).not.toContain('>关闭</t-button>')
    // 三颗上下文主操作都在页脚里，位置不随正文高度移动。
    expect(footer).toContain('加入我的素材')
    expect(footer).toContain('去我的素材')
    expect(footer).toContain("$emit('download', material)")
    // 正文里不再留一份动作条——留一份就是两处按钮，一处在滚动区里。
    const body = template.slice(0, template.indexOf('<template #footer>'))
    expect(body).not.toContain('material-detail__actions')
  })

  // 页脚整条只剩业务动作，靠右收；关闭是右上角 × 的事，不在页脚占一格。
  it('keeps the footer on the context actions alone, aligned right', () => {
    const footer = sliceBetween(template, '<template #footer>', '</template>\n  </t-drawer>')
    expect(footer).not.toContain('>关闭</t-button>')
    expect(source).toMatch(/\.material-detail__actions \{[^}]*justify-content: flex-end/)
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
   * 页脚按「使用状态 × 下载状态」矩阵给真实业务动作（CHG-069 任务 23，与行内同一张表）。
   * 「已放弃」只给恢复：一颗同时出现的「下载」会让运营以为那条关系还通着。
   *
   * 使用中：已下载 → 加入合成；下载中 → 取消下载（不再给「再下载」，文件已经在准备了）；
   * 未下载/失败 → 下载/重新下载。
   */
  it('gives the footer the action the usage × download matrix calls for', () => {
    const footer = sliceBetween(template, '<template #footer>', '</template>\n  </t-drawer>')
    expect(footer).toContain("$emit('restore', material)")
    expect(footer).toContain('>恢复使用</t-button>')
    expect(footer).toContain("$emit('give-up', material)")
    expect(footer).toContain('>放弃使用</t-button>')
    expect(footer).toContain("$emit('compose', material)")
    expect(footer).toContain("downloadStatus === 'downloaded'")
    expect(footer).toContain("downloadStatus === 'downloading'")
    expect(footer).toContain('@click="cancelDownload">取消下载</t-button>')
    expect(footer).toContain("$emit('download', material)\">{{ downloadActionLabel(downloadStatus) }}")
    // 恢复与放弃互斥，下载与放弃互斥：同一个素材不能同时有两条相反的出路。
    expect(footer).toContain("material.usage_status === 'removed'")
    // 页脚判据切到 download_status，不再碰 video_status。
    expect(footer).not.toContain('video_status')
  })
})

/**
 * 已下载的素材要能说清「原文件在我这台机器的哪个文件夹里」，并能打开那个文件夹
 * （2026-09-30 用户裁定「需要有展示下载目录的地方同时能打开目录快速找到原视频」，
 * 并指定「放在文件信息一栏里」）。
 *
 * 这一块钉住的是**值从哪来**：文件名是执行器报的那一个（从这张用户的下载任务表里
 * 挑），目录是 Desktop 量出来的（`local_saved_file_states`）—— 两者都不是前端拼的，
 * 也不是拿文件状态（`video_status` 说的是云端那份源文件）冒充的。
 */
describe('the local copy of a downloaded material', () => {
  it('takes the file name from the download tasks and the directory from this machine', () => {
    expect(source).toContain("import { createFileTransferClient } from '../../shared/api/fileTransfer.js'")
    expect(source).toContain("import { downloadedFileName, isTerminal } from '../transfer/downloadFacts.js'")
    expect(source).toContain("import { isDesktopRuntime, revealSavedFile, savedFileStates } from '../transfer/desktopBridge.js'")
    expect(source).toContain('downloadedFileName(tasks')
    expect(source).toContain('savedFileStates([name])')
    // 名字从扫描结果那一条上读回来（`savedFileStates` 的表里带着 `name`），不另存一份。
    expect(source).toContain('revealSavedFile(localFile.value.name')
  })

  // 切到这张卡自己的 `</section>` 为止，不是切到下一张卡的标题：后者会把两张卡之间的空档
  // 也算进来，把这块搬出卡片照样通过（实测：搬到空档里 29 条仍全绿）。
  it('puts the download directory in the 文件信息 card, with a way to open it', () => {
    const card = sliceBetween(template, 'class="detail-card__title">\n            文件信息', '</section>')
    expect(card).toContain('下载目录')
    expect(card).toContain('localFile.directory')
    expect(card).toContain('打开目录')
  })

  // 下载归「我的素材」（§2）：素材库上下文打开的详情不该出现本机文件的那一行，
  // 那一页连「下载」入口都没有。
  it('asks this machine only in the 我的素材 context', () => {
    // 锚在**这个** watcher 自己的依赖数组上：详情里另有一个同开头的 watcher（云端视频
    // 地址），按前缀切会切到那一个，断言就在别的代码上通过了。
    const watcher = sliceBetween(source, 'watch(() => [props.visible, props.material?.id, props.mode]', '\n})')
    expect(watcher).toContain("mode !== 'mine'")
  })

  /**
   * 读不到本机时整行不出现 —— 浏览器读不到本机文件，Desktop 也要扫过才知道。
   *
   * 与下载中心同一条约定（`desktopBridge.savedFileStates` 在浏览器里返回空表）：
   * 把「我读不到」写成「文件不在」，运营会据此重新下一份几百兆。
   */
  it('says nothing at all when this machine has nothing measured', () => {
    // 一整行连标签一起钉：条件与元素分开断言时，条件写在别处也能通过。
    expect(template).toContain('<div v-if="localFile?.directory" class="detail-local-file">')
    const block = sliceBetween(template, 'class="detail-local-file"', '</section>')
    expect(block).not.toContain('未下载')
    expect(block).not.toContain('本地文件')
  })

  // 这一行说的是磁盘上那一份，与云端那份源文件的就绪状态无关：拿 `video_status`
  // 当条件是错的 —— 它会在一块「文件就在这儿」的素材上说「未就绪」，也会在一块
  // 云端就绪但本机从没下过的素材上画一个不存在的目录。
  it('does not read the local copy off the file state', () => {
    const block = sliceBetween(template, 'class="detail-local-file"', '</section>')
    expect(block).not.toContain('video_status')
    expect(source).toContain('downloadedFileName(tasks, id)')
    // 判据在**取事实的那一段**里：这条 watch 体里一次都不提文件状态。整份源码不能这样断言
    // ——文件状态在别处有正当用处（云端视频地址那条 watch 就按它分支）。
    const watcher = sliceBetween(source, 'watch(() => [props.visible, props.material?.id, props.mode]', '\n})')
    expect(watcher).not.toContain('video_status')
  })
})

/**
 * 详情抽屉的「文件状态」徽章与我的素材页同语义：下载生命周期优先，素材云侧
 * `video_status` 兜底。下载生命周期在 mine 上下文里从它**已拉的**这张用户任务表就地
 * 派生，不为此加接口。
 */
describe('the drawer file state derives from the user download lifecycle', () => {
  it('derives the download lifecycle from the newest user_download task', () => {
    // 徽章词表从 labels.js 取（import 是换行多名的，逐名断言）。
    const labelsImport = sliceBetween(source, "import {\n", "} from './labels.js'")
    expect(labelsImport).toContain('downloadStatusLabel')
    expect(labelsImport).toContain('downloadStatusTone')
    // 只看本机 + 这个动作（与 downloadedFileName 同一套过滤），三档映射。
    expect(source).toContain("task?.execution_scope !== 'local_agent'")
    expect(source).toContain("task?.purpose !== 'user_download'")
    expect(source).toContain("return 'downloading'")
    expect(source).toContain("return 'downloaded'")
    expect(source).toContain("return 'failed'")
  })

  it('prioritises the download status and falls back to video_status, failed included', () => {
    const block = sliceBetween(source, 'const fileStatus = computed(() => {', '})')
    expect(block).toContain('downloadStatusLabel(downloadStatus.value)')
    expect(block).toContain("props.material?.video_status === 'failed'")
    expect(block).toContain("label: '下载失败'")
    expect(block).toContain('videoStatusLabel(props.material?.video_status)')
  })

  it('fills the derived status in the mine-context watcher, without reading video_status there', () => {
    const watcher = sliceBetween(source, 'watch(() => [props.visible, props.material?.id, props.mode]', '\n})')
    expect(watcher).toContain('deriveDownloadStatus(tasks, id)')
    expect(watcher).toContain('deriveActiveTask(tasks, id)')
    expect(watcher).not.toContain('video_status')
  })
})

/**
 * 下载进行中（CHG-069 任务 23）：正文「文件信息」卡里渲染下载进度/速度/预计时间，
 * 页脚主操作换成「取消下载」。读数与下载中心同一套 —— 从 transferRow 来，不是前端编的。
 */
describe('the drawer while a download is in progress', () => {
  it('renders the live download facts in the 文件信息 card', () => {
    const card = sliceBetween(template, 'class="detail-card__title">\n            文件信息', '</section>')
    expect(card).toContain('下载进度')
    expect(card).toContain('activeRow.progress.percent')
    expect(card).toContain('activeRow.pendingText')
    expect(card).toContain('activeRow.sizeText')
    expect(card).toContain('activeRow.rateText')
    expect(card).toContain('activeRow.etaText')
    // 进度条只在分母已知时画（与下载中心同一约定）。
    expect(card).toContain('activeRow.showBar')
  })

  it('finds the active task through the same user_download filter as the lifecycle', () => {
    expect(source).toContain("task?.execution_scope !== 'local_agent'")
    expect(source).toContain("task?.purpose !== 'user_download'")
    expect(source).toContain("task?.status === 'pending' || task?.status === 'running'")
  })

  it('cancels through the transfer client and remembers the request locally', () => {
    expect(source).toContain('transfer.cancelTask(activeTransferTask.value.id)')
    expect(source).toContain('isTerminal(refreshed)')
    expect(source).toContain('cancelRequested.value = true')
    // 请求发出后按钮收回、显示「正在取消」。
    expect(source).toContain('正在取消')
  })

  // 只有正在下载（有进行中任务）才出这一块；没有就整块不出现。
  it('shows the download block only while a transfer is actually running', () => {
    expect(template).toContain('v-if="activeRow" class="detail-download"')
  })
})
