// 拿到签名地址之后，这一步交给谁。
//
// 两个宿主的规矩不一样，而且不能互相顶替：
//
// - **浏览器**：跨过 `await` 之后就不再算「用户手势」，那时才调的 `window.open` 会被
//   拦（WebKit 一律拦）。所以空标签页必须在**取地址之前**同步开出来，拿到地址后再
//   导航它。`opener` 置空等价于 `rel="noopener"`。
// - **打包的桌面端**：宿主是 WKWebView，页面根本开不出窗口——新窗口请求由壳接管
//   （`wt-media-desktop` 的 `external_links`），壳拒绝在应用内开窗、把 http/https 交给
//   系统浏览器。这里因此**不预留标签页**：预留只会拿到 `null`，而把那个 `null` 读成
//   「被拦」正是此前「浏览器拦截了新标签页」那句误报的来处。
//
// 地址是签名的、会过期，所以每次点击现取，不预先拿也不留：`getVideoUrl` 只在这一刻
// 被调用一次。

/** 取一次签名地址。空地址当失败抛出，别让空串走到窗口 API 里去。 */
async function cloudVideoAddress(client, materialId) {
  const data = await client.getVideoUrl(materialId)
  const url = data?.url
  if (!url) throw new Error('云端视频地址为空')
  return url
}

/**
 * @param {object} input
 * @param {{ getVideoUrl: (materialId: number) => Promise<object> }} input.client
 * @param {boolean} input.desktop 宿主是不是打包的桌面端（`isDesktopRuntime()`）
 * @param {(message: string) => void} input.reportError 报错出口
 */
export async function handOverCloudVideo({ client, materialId, desktop, reportError }) {
  // `desktop` 为真时连预留都不发：桌面端这一侧没有「用户手势」可用——地址要等一次
  // 请求——所以那边是壳把无手势的开口打开（见 `external_links` 的头注释）。
  const tab = desktop ? null : window.open('', '_blank')
  if (!desktop && !tab) {
    // 连同步的 window.open 都被拦，就没有可导航的标签页了；此时再去取地址是白发一次
    // 签名请求。
    reportError('浏览器拦截了新标签页，请允许弹出窗口后重试')
    return
  }
  if (tab) tab.opener = null
  try {
    const url = await cloudVideoAddress(client, materialId)
    // 桌面端这一次 `window.open` 的返回值恒为 `null`：壳一律拒绝在应用内开窗，地址
    // 已经交给系统浏览器了。判它就会把成功读成失败。
    if (tab) tab.location.replace(url)
    else window.open(url, '_blank', 'noopener')
  } catch (error) {
    if (tab) tab.close()
    reportError(error?.message || '打开云端视频失败')
  }
}
