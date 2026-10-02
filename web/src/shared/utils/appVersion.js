import { isDesktop } from '../../utils.js'
import webPackage from '../../../package.json'

// 应用版本号：Desktop 运行时读 Tauri 真实版本，浏览器预览回落 Web 包版本。
// 登录页顶栏与后台左上角品牌共用，两处必须显示同一个数。
export async function resolveAppVersion() {
  if (isDesktop()) {
    try {
      const { getVersion } = await import('@tauri-apps/api/app')
      return await getVersion()
    } catch {
      // Browser preview uses the Web build version.
    }
  }
  return webPackage.version
}
