import { afterEach, describe, expect, it, vi } from 'vitest'
import { TRANSFER_COMMANDS, isDesktopRuntime, openSavedFile, revealSavedFile, savedFileStates } from './desktopBridge.js'

// 桌面运行时在测试里是**造出来的**：`environment: 'node'` 下根本没有 `window`，
// 于是 `isDesktopRuntime()` 天然为假 —— 这本身也要钉（见下），否则「浏览器里不该发请求」
// 这条会因为测试环境的巧合而永远成立。
function pretendDesktop() {
  globalThis.window = { __TAURI_INTERNALS__: {} }
}

afterEach(() => {
  delete globalThis.window
})

describe('desktop bridge', () => {
  /**
   * 命令名是**跨仓字面量**，两边各写一份，任何一侧改名都不会有编译器报错。
   *
   * 这条钉的就是那个字面量：Desktop 仓 `commands/downloads.rs` 与
   * `commands/saved_files.rs` 里的命令名必须逐字一致，改一侧就必须改这里 ——
   * 否则界面上是「点了没反应」或一句 `Command not found`，而两仓各自的测试全绿。
   */
  it('pins the command names the Desktop repository registers', () => {
    expect(TRANSFER_COMMANDS).toEqual({
      openSavedFile: 'local_open_saved_file',
      revealSavedFile: 'local_reveal_saved_file',
      savedFileStates: 'local_saved_file_states',
    })
  })

  it('knows it is not on the desktop by default', () => {
    expect(isDesktopRuntime()).toBe(false)
    pretendDesktop()
    expect(isDesktopRuntime()).toBe(true)
  })
})

describe('opening a saved file', () => {
  it('sends the name the executor reported', async () => {
    pretendDesktop()
    const invokeImpl = vi.fn().mockResolvedValue(undefined)
    await openSavedFile('演示素材-42.mp4', { invokeImpl })
    // 只发名字。落点目录由 Rust 侧在已知的保存位置里找 —— 前端不知道它，也不该猜。
    expect(invokeImpl).toHaveBeenCalledWith('local_open_saved_file', { name: '演示素材-42.mp4' })
  })

  it('refuses in a browser and refuses without a name', async () => {
    const invokeImpl = vi.fn()
    await expect(openSavedFile('a.mp4', { invokeImpl })).rejects.toThrow('桌面客户端')
    pretendDesktop()
    await expect(openSavedFile('', { invokeImpl })).rejects.toThrow('没有报告文件名')
    expect(invokeImpl).not.toHaveBeenCalled()
  })
})

/**
 * 打开一个已下载文件**所在的目录**。
 *
 * 与「打开文件」同一形状（都是动作，所以浏览器里必须明说做不到，也必须收名字不收路径），
 * 差别只在那半句：交给文件管理器的是那一份的**所在目录**，不是文件本身 —— 交给播放器
 * 打开视频是另一颗按钮的事。
 */
describe('revealing the directory a saved file is in', () => {
  it('sends the name the executor reported', async () => {
    pretendDesktop()
    const invokeImpl = vi.fn().mockResolvedValue('/Volumes/Movies/WTMedia')
    await revealSavedFile('演示素材-42.mp4', { invokeImpl })
    expect(invokeImpl).toHaveBeenCalledWith('local_reveal_saved_file', { name: '演示素材-42.mp4' })
  })

  it('refuses in a browser and refuses without a name', async () => {
    const invokeImpl = vi.fn()
    await expect(revealSavedFile('a.mp4', { invokeImpl })).rejects.toThrow('桌面客户端')
    pretendDesktop()
    await expect(revealSavedFile('', { invokeImpl })).rejects.toThrow('没有报告文件名')
    expect(invokeImpl).not.toHaveBeenCalled()
  })
})

/**
 * 一批文件现在各自在哪儿。
 *
 * 与「打开文件」相反，读不到就不是错误：浏览器读不到本机，得到的是**未知**，
 * 而未知不是「不在」。这一块钉的是那条界线在桥这一层没有被抹掉。
 */
describe('reading where saved files are', () => {
  it('asks once for the whole batch, without repeats', async () => {
    pretendDesktop()
    const invokeImpl = vi.fn().mockResolvedValue([])
    await savedFileStates(['b.mp4', 'a.mp4', 'b.mp4'], { invokeImpl })
    expect(invokeImpl).toHaveBeenCalledTimes(1)
    expect(invokeImpl).toHaveBeenCalledWith('local_saved_file_states', { names: ['b.mp4', 'a.mp4'] })
  })

  it('says nothing at all with nothing to ask about', async () => {
    pretendDesktop()
    const invokeImpl = vi.fn()
    expect(await savedFileStates([], { invokeImpl })).toEqual({})
    expect(await savedFileStates(null, { invokeImpl })).toEqual({})
    expect(await savedFileStates(['', '   ', null, 7], { invokeImpl })).toEqual({})
    expect(invokeImpl).not.toHaveBeenCalled()
  })

  // 浏览器不是错误分支：读本机文件本来就做不到，空表即「没人查过」。
  it('answers nothing rather than throwing when there is no desktop', async () => {
    const invokeImpl = vi.fn()
    expect(await savedFileStates(['a.mp4'], { invokeImpl })).toEqual({})
    expect(invokeImpl).not.toHaveBeenCalled()
  })

  it('reads a directory in the current location apart from an older one', async () => {
    pretendDesktop()
    const invokeImpl = vi.fn().mockResolvedValue([
      { name: 'a.mp4', directory: '/新位置', current: true, bytes: 230 },
      { name: 'b.mp4', directory: '/旧位置', current: false, bytes: 12 },
      { name: 'c.mp4', directory: null, current: false, bytes: null },
    ])

    expect(await savedFileStates(['a.mp4', 'b.mp4', 'c.mp4'], { invokeImpl })).toEqual({
      'a.mp4': { name: 'a.mp4', presence: 'present_current', directory: '/新位置', bytes: 230 },
      // 「在旧目录里」是一个**答案**，不是缺省：运营换了保存位置以后，文件就在这儿。
      'b.mp4': { name: 'b.mp4', presence: 'present_elsewhere', directory: '/旧位置', bytes: 12 },
      // 查过了、所有已知位置都没有 —— 这才是「可以重新下载」那一格。
      'c.mp4': { name: 'c.mp4', presence: 'absent', directory: '', bytes: null },
    })
  })

  /**
   * Rust 没答上来的名字**不进表**，于是下游读成 `unknown`。
   *
   * 这是刻意的：少答一个名字有可能是桥丢了、有可能是命令出错，任何一种都不构成
   * 「这个文件不在了」的证据。此刻把它填成 `absent` 会在界面上给出一个「重新下载」，
   * 而文件可能好端端地躺在那里 —— 拿一次失败当事实，比少一个按钮贵得多。
   */
  it('leaves a name the answer skipped as never-looked', async () => {
    pretendDesktop()
    const invokeImpl = vi.fn().mockResolvedValue([
      { name: 'a.mp4', directory: null, current: false, bytes: null },
    ])
    const table = await savedFileStates(['a.mp4', 'b.mp4'], { invokeImpl })
    expect(table['a.mp4'].presence).toBe('absent')
    expect('b.mp4' in table).toBe(false)
  })

  it('does not turn a missing size into a number', async () => {
    pretendDesktop()
    const invokeImpl = vi.fn().mockResolvedValue([
      { name: 'a.mp4', directory: '/新位置', current: true },
      { name: 'b.mp4', directory: '/新位置', current: true, bytes: '230' },
      { name: 'c.mp4', directory: '/新位置', current: true, bytes: 0 },
    ])
    const table = await savedFileStates(['a.mp4', 'b.mp4', 'c.mp4'], { invokeImpl })
    expect(table['a.mp4'].bytes).toBeNull()
    expect(table['b.mp4'].bytes).toBeNull()
    // 0 是量到的 0，不是「不知道」。
    expect(table['c.mp4'].bytes).toBe(0)
  })

  // 一个名字在历史目录里、`current` 缺席（老响应或不完整的行）时按「不在当前位置」读 ——
  // 目录是主判据，`current` 只用来分「当前」与「更早」。
  it('treats a missing current flag as elsewhere rather than current', async () => {
    pretendDesktop()
    const invokeImpl = vi.fn().mockResolvedValue([{ name: 'a.mp4', directory: '/旧位置', bytes: 1 }])
    expect((await savedFileStates(['a.mp4'], { invokeImpl }))['a.mp4'].presence).toBe('present_elsewhere')
  })
})
