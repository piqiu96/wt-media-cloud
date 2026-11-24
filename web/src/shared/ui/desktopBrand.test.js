import { afterEach, expect, it } from 'vitest'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import BrandLogo from './BrandLogo.vue'
import { desktopEnvironment, setDesktopEnvironment } from './desktopBrand.js'
afterEach(() => { desktopEnvironment.value = '' })
for (const [environment, badge] of [['dev','DEV'],['test','TEST']]) {
  it(`${environment} desktop keeps its badge in full and collapsed navigation`, async () => {
    setDesktopEnvironment(environment)
    for (const compact of [false,true]) {
      const html=await renderToString(createSSRApp({render:()=>h(BrandLogo,{compact})}))
      expect(html).toContain('brand-logo__badge')
      expect(html).toContain('>'+badge+'</b>')
    }
  })
}
it('formal desktop and ordinary Web retain the original brand',async()=>{
  for(const environment of ['','prod']){
    desktopEnvironment.value=environment
    const html=await renderToString(createSSRApp({render:()=>h(BrandLogo)}))
    expect(html).not.toContain('brand-logo__badge')
    expect(html).toMatch(/<strong[^>]*>起飞<\/strong>/)
  }
})
it('unknown native identity cannot silently become formal desktop',()=>{
  expect(()=>setDesktopEnvironment('unknown')).toThrow('桌面环境配置无效')
})
