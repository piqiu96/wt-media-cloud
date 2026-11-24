import { shallowRef } from 'vue'
export const desktopEnvironment = shallowRef('')
export function setDesktopEnvironment(environment) {
  if (!['dev', 'test', 'prod'].includes(environment)) throw new Error('桌面环境配置无效，请重新安装对应版本')
  desktopEnvironment.value = environment
}
export function desktopBrand(environment) {
  const badge = environment === 'dev' ? 'DEV' : environment === 'test' ? 'TEST' : ''
  return { badge, name: badge ? `起飞 ${badge === 'DEV' ? 'Dev' : 'Test'}` : '起飞' }
}
