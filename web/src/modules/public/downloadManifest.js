const REPOSITORY = 'piqiu96/wt-media-workspace'
const PLATFORMS = {
  'windows-x64': 'windows-x64-setup.exe',
  'macos-x64': 'macos-x64.zip',
  'macos-arm64': 'macos-arm64.zip',
}

export function parseDownloadManifest(value) {
  if (!value || value.schema_version !== 1 || !/^v\d+\.\d+\.\d+$/.test(value.version)) {
    throw new Error('桌面版下载清单无效')
  }
  const downloads = value.downloads
  if (!downloads || Object.keys(downloads).length !== 3 || Object.keys(PLATFORMS).some((platform) => !downloads[platform])) {
    throw new Error('桌面版下载清单缺少平台')
  }
  for (const [platform, suffix] of Object.entries(PLATFORMS)) {
    const item = downloads[platform]
    const expectedName = `WT-Media_${value.version}_${suffix}`
    const expectedUrl = `https://github.com/${REPOSITORY}/releases/download/${value.version}/${expectedName}`
    if (item?.file_name !== expectedName || item?.url !== expectedUrl) {
      throw new Error(`桌面版 ${platform} 下载链接无效`)
    }
  }
  return value
}

export async function loadDownloadManifest(fetcher = fetch) {
  const response = await fetcher('/desktop-downloads.json', { cache: 'no-store' })
  if (!response.ok) throw new Error('暂时无法读取桌面版下载信息')
  return parseDownloadManifest(await response.json())
}
