import { describe, expect, it } from 'vitest'
import { runningReadFailureCopy, runningReadSuccessCopy } from './profileRunningFeedback.js'

describe('running-state refresh feedback', () => {
  it('keeps successful manual feedback tied to the actual scan', () => {
    expect(runningReadSuccessCopy({ profiles: [{}, {}, {}] })).toBe('已读取 3 个比特窗口的运行状态。')
  })

  it.each([
    ['BIT_NOT_BOUND', '尚未绑定比特账号，请先到「个人信息」完成绑定。'],
    ['BIT_MAIN_USER_MISMATCH', '比特登录账号与系统绑定账号不一致，请切换到正确账号后再试。'],
    ['BIT_BROWSER_OFFLINE', '无法连接比特，请启动比特并确认已登录。'],
    ['agent_unreachable', '本机服务暂不可用，请重启桌面端后再试。'],
  ])('provides a concrete action for %s', (code, message) => {
    expect(runningReadFailureCopy({ code })).toBe(message)
  })

  it('does not expose unknown technical errors to operators', () => {
    expect(runningReadFailureCopy({ message: 'socket ECONNRESET at localhost:54345' })).toBe('读取运行状态失败，请确认比特已启动并登录，稍后再试。')
  })
})
