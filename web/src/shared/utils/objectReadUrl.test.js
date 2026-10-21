import { describe, expect, it } from 'vitest'
import { publicObjectReadUrl } from './objectReadUrl.js'

describe('public object read URL', () => {
  it('maps a finished video from the S3 write path to the public read path', () => {
    const signed = 'https://s3.oss.longyanyue.cn/data/wt_media/dev/compose-outputs/315/1/output.mp4?X-Amz-Signature=secret'
    expect(publicObjectReadUrl(signed)).toBe('https://data.bucket.oss.longyanyue.cn/wt_media/dev/compose-outputs/315/1/output.mp4')
  })

  it('uses the same rule for material videos and existing objects in the data bucket', () => {
    expect(publicObjectReadUrl('https://s3.oss.longyanyue.cn/data/wt_media/dev/materials/30/a.mp4?X-Amz-Signature=secret'))
      .toBe('https://data.bucket.oss.longyanyue.cn/wt_media/dev/materials/30/a.mp4')
    expect(publicObjectReadUrl('https://s3.oss.longyanyue.cn/data/2026h2/game_img/ksgjc.png'))
      .toBe('https://data.bucket.oss.longyanyue.cn/2026h2/game_img/ksgjc.png')
  })

  it('leaves other buckets and storage hosts unchanged', () => {
    const otherBucket = 'https://s3.oss.longyanyue.cn/archive/wt_media/dev/materials/30/a.mp4?signature=secret'
    const otherHost = 'https://example.com/data/wt_media/dev/materials/30/a.mp4?signature=secret'
    const alreadyPublic = 'https://data.bucket.oss.longyanyue.cn/wt_media/dev/materials/30/a.mp4'
    expect(publicObjectReadUrl(otherBucket)).toBe(otherBucket)
    expect(publicObjectReadUrl(otherHost)).toBe(otherHost)
    expect(publicObjectReadUrl(alreadyPublic)).toBe(alreadyPublic)
  })

  it('keeps an absent address empty', () => {
    expect(publicObjectReadUrl('')).toBe('')
    expect(publicObjectReadUrl(null)).toBe('')
  })
})
