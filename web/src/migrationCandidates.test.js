import { describe, expect, it } from 'vitest'

import { candidateNames, loadCandidateNames } from './apps/desktop/features/local-settings/migrationCandidates.js'

const task = (over = {}) => ({
  id: 't-1',
  execution_scope: 'local_agent',
  asset_type: 'material',
  asset_id: 42,
  file_name: '演示素材-42.mp4',
  ...over,
})

describe('migration candidates', () => {
  /**
   * 名单是「这台机器下的」，不是「这条素材有名字」。
   *
   * 云端执行的任务没有落在本机的文件，把它列进搬运名单会让运营在一个不存在的文件上
   * 做决定 —— 而搬运与删除都不是可以猜的动作。
   */
  it('keeps only the rows this machine executed', () => {
    const names = candidateNames([
      task(),
      task({ id: 't-2', execution_scope: 'cloud', file_name: '云端处理的.mp4' }),
      task({ id: 't-3', execution_scope: 'local_agent', file_name: '另一个.mp4' }),
    ])

    expect(names).toEqual(['另一个.mp4', '演示素材-42.mp4'])
    expect(names).not.toContain('云端处理的.mp4')
  })

  /**
   * 没有文件名的行跳过。
   *
   * 列表接口对没有名字的任务**不下发这个键**（`file_name` 是 `omitempty`），所以这里
   * 要接住的是 `undefined`；一个空串送进 Rust 只会换回「不是一个文件名」。
   */
  it('skips rows whose executor never reported a name', () => {
    expect(candidateNames([
      task({ file_name: undefined }),
      task({ id: 't-2', file_name: '' }),
      task({ id: 't-3', file_name: '   ' }),
      task({ id: 't-4', file_name: '真的.mp4' }),
    ])).toEqual(['真的.mp4'])
  })

  /**
   * 名字**原样**用，不裁剪。
   *
   * 磁盘上的 `a.mp4` 与 ` a.mp4` 是两个文件，而 `file_name_of` 只拒绝路径分隔符，
   * 不拒绝空格 —— 裁掉前导空格会让后者的搬运去找错对象（或者更糟：找到同名的那一个）。
   */
  it('does not trim a name it keeps', () => {
    expect(candidateNames([task({ file_name: ' 前导空格.mp4' })])).toEqual([' 前导空格.mp4'])
  })

  // 同一个名字会出现在多条任务上（重试、重新下载、同一个素材点了两次）。名单是给
  // 人勾选的，重复行会让「搬几个文件」这个数字读成 2。
  it('lists a name once however many tasks carry it', () => {
    expect(candidateNames([
      task(),
      task({ id: 't-2' }),
      task({ id: 't-3', file_name: '演示素材-42.mp4' }),
    ])).toEqual(['演示素材-42.mp4'])
  })

  // 顺序必须是确定的：同一份任务在不同时刻读出来排成不同的样子，会让「上次看到的那
  // 一行」找不到。
  it('orders the names the same way every time', () => {
    const names = candidateNames([task({ file_name: 'b.mp4' }), task({ id: 't-2', file_name: 'a.mp4' })])
    expect(names).toEqual(['a.mp4', 'b.mp4'])
  })

  it('answers an empty list rather than failing when nothing was downloaded', () => {
    expect(candidateNames([])).toEqual([])
    expect(candidateNames(null)).toEqual([])
  })
})

describe('loading the candidate list', () => {
  it('reads the download centre’s own endpoint', async () => {
    const seen = []
    const names = await loadCandidateNames({
      client: {
        async listTasks() {
          seen.push('listTasks')
          return [task(), task({ id: 't-2', execution_scope: 'cloud' })]
        },
      },
    })

    expect(seen).toEqual(['listTasks'])
    expect(names).toEqual(['演示素材-42.mp4'])
  })

  /**
   * 取不到名单时**带着原因抛出去**，不返回空名单。
   *
   * 两者在界面上的差别是「读云端记录失败：…」与「暂无可搬文件」，而后者会让运营以为
   * 东西被清空了 —— 这一页接下来要拿这份名单去搬和删文件，一份假的空名单比一句错误
   * 危险得多。
   */
  it('reports why the list could not be read instead of answering empty', async () => {
    await expect(
      loadCandidateNames({
        client: {
          async listTasks() {
            throw new Error('HTTP 401')
          },
        },
      })
    ).rejects.toThrow(/HTTP 401/)

    await expect(
      loadCandidateNames({ client: { async listTasks() { throw 'not an Error' } } })
    ).rejects.toThrow(/读取本机下载记录失败/)
  })
})
