import { describe, expect, it } from 'vitest'
import { parseDownloadManifest } from './downloadManifest.js'

const version = 'v0.1.0'
const name = (target) => `WT-Media_${version}_${target}`
const item = (target) => ({
  file_name: name(target),
  url: `https://github.com/piqiu96/wt-media-workspace/releases/download/${version}/${name(target)}`,
})
const valid = () => ({
  schema_version: 1,
  version,
  downloads: {
    'windows-x64': item('windows-x64-setup.exe'),
    'macos-x64': item('macos-x64.zip'),
    'macos-arm64': item('macos-arm64.zip'),
  },
})

describe('public desktop download manifest', () => {
  it('accepts exactly the published release URLs for three platforms', () => {
    expect(Object.keys(parseDownloadManifest(valid()).downloads)).toEqual(['windows-x64', 'macos-x64', 'macos-arm64'])
  })

  it('rejects missing platforms and foreign or malformed URLs', () => {
    const missing = valid()
    delete missing.downloads['macos-arm64']
    expect(() => parseDownloadManifest(missing)).toThrow()
    const foreign = valid()
    foreign.downloads['windows-x64'].url = 'https://evil.example/installer.exe'
    expect(() => parseDownloadManifest(foreign)).toThrow()
    const wrongVersion = valid()
    wrongVersion.downloads['windows-x64'].url = item('windows-x64-setup.exe').url.replaceAll('v0.1.0', 'v0.1.1')
    expect(() => parseDownloadManifest(wrongVersion)).toThrow()
  })
})
