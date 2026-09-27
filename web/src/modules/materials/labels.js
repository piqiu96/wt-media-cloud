// 素材的状态与展示文案。
//
// 四种视频状态直接来自冻结的 `video_status` enum（contracts/business-schemas/v1/
// content-production.yaml），不多不少 —— 页面上出现第五种状态（比如「下载中」之外的
// 「解析中」）就意味着前端自己发明了一个服务端不会写的值，那种状态永远不会出现，
// 而它对应的进度条会永远停在初始位置。
export const VIDEO_STATUSES = ['not_downloaded', 'downloading', 'ready', 'failed']

const VIDEO_STATUS_LABELS = {
  not_downloaded: '未准备',
  downloading: '准备中',
  ready: '可下载',
  failed: '准备失败',
}

const VIDEO_STATUS_TONES = {
  not_downloaded: 'neutral',
  downloading: 'info',
  ready: 'success',
  failed: 'danger',
}

export function videoStatusLabel(value) {
  return VIDEO_STATUS_LABELS[value] || value || '-'
}

export function videoStatusTone(value) {
  return VIDEO_STATUS_TONES[value] || 'neutral'
}

/**
 * 点「下载到本机」之前该多说一句什么。
 *
 * 这里**故意没有**一个「能不能下载」的判据。服务端的 `CreateDownload` 接受
 * `not_downloaded` 与 `failed`：点了它会先建云端准备任务，再把本机下载挂上去等它交付。
 * 所以没有一种 `video_status` 是「点了也没用」的，而一个灰按钮正是在说这句话——运营
 * 看到的是这条路断了，实际只是要多走一步准备。
 *
 * 状态徽章已经写着「未准备」「准备失败」，所以这里只补「点下去会发生什么」这一句，
 * 而且只在它有信息量的时候说：`ready` 点了就下，`downloading` 正在准备，都不需要翻译。
 */
export function downloadHint(row) {
  if (row?.video_status === 'not_downloaded') return '点下载会先准备视频'
  if (row?.video_status === 'failed') return '点下载会重新准备'
  return ''
}

/**
 * 游戏名。查不到就退回 id 而不是 `-`：素材的 `game_id` 是一份真实数据，
 * 游戏表里没有它只是「这一页拿不到名字」，把它显示成「没有游戏」是另一回事。
 */
export function gameName(games, id) {
  if (!id) return '-'
  return games.find((item) => item.id === id)?.name || id
}

/** 摘要在表里放不下整串，但也不该断成两行：截到 16 位再加省略号。 */
export function shortDigest(value) {
  return value ? `${value.slice(0, 16)}…` : '-'
}
