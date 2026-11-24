import { isDesktop } from '../../utils.js'

// Only Desktop has a user-facing version. Read the installed native package;
// a failed native call must not substitute the Web source package version.
export async function resolveAppVersion() {
  if (!isDesktop()) return ''
  try {
    const { getVersion } = await import('@tauri-apps/api/app')
    return await getVersion()
  } catch {
    return ''
  }
}
