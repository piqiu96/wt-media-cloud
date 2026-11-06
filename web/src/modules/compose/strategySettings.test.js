import { describe, expect, it } from 'vitest'
import { reactive } from 'vue'
import { copySetting, initialParams, newSetting, settingFields, strategySettings, validateSettings } from './strategySettings.js'
const fields = settingFields({ required: ['start', 'guide_video_url'], properties: { start: { title: '开始', type: 'integer', minimum: 0, maximum: 20, default: 3 }, guide_video_url: { type: 'string' } } })
describe('strategy output settings', () => {
  it('opens reactive saved groups and copies without mutating the saved snapshot or retaining identity', () => {
    const strategy = reactive({ output_settings: [{ key: 'original', name: '通用', platforms: ['bilibili'], copies: 2, params: { start: 5 } }] })
    const draft = strategySettings(strategy)
    const copied = copySetting(reactive(draft[0]))
    copied.params.start = 9
    copied.platforms.push('baidu')
    draft[0].name = '改名'
    expect(copied.key).not.toBe('original')
    expect(strategy.output_settings[0]).toEqual({ key: 'original', name: '通用', platforms: ['bilibili'], copies: 2, params: { start: 5 } })
    expect(draft[0].params.start).toBe(5)
  })
  it('new groups use template defaults independently of edited groups', () => {
    const first = newSetting(fields)
    first.params.start = 15
    expect(newSetting(fields).params).toEqual(initialParams(fields))
    expect(first.platforms).toEqual([])
  })
  it('validates complete groups, platform whitelist, numeric bounds and public HTTPS address', () => {
    const group = { key: 'a', name: '组', platforms: ['bilibili', 'xiaohongshu'], copies: 2, params: { start: 0, guide_video_url: 'https://example.com/a.mp4' } }
    expect(validateSettings([group], fields)).toEqual({})
    expect(validateSettings([{ ...group, platforms: ['douyin'], copies: 0, params: { start: 21, guide_video_url: 'http://example.com/a.mp4' } }], fields).a).toEqual({ platforms: '请选择适用平台', copies: '每条素材生成 1～20 份', start: '请填写有效的开始', guide_video_url: '请填写文件管理中的视频链接或 HTTPS 视频地址' })
    expect(validateSettings([{ ...group, params: { start: null, guide_video_url: '' } }], fields).a).toHaveProperty('start')
  })
  it('retains old config for explicit migration without inventing target platforms', () => {
    expect(strategySettings({ config: { start: 8 } })[0]).toEqual({ key: 'default', name: '默认成片', platforms: [], copies: 1, params: { start: 8 } })
  })
})
