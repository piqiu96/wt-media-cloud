import {readFileSync} from 'node:fs'
import {describe,it,expect} from 'vitest'
const source=readFileSync(new URL('./apps/desktop/features/local-settings/LocalSettingsPage.vue',import.meta.url),'utf8')
const body=name=>source.split(`async function ${name}()`)[1].split('\n}')[0]
describe('native folder choices stay independent',()=>{
 it('uses a native save picker without forcing file migration',()=>{
  const save=body('saveSaveDir')
  expect(save).toContain('service.pickSaveDirectory()')
  expect(save).toContain('if (!written) return')
  expect(save).toContain('await pushSaveDir()')
  expect(save).not.toContain('openMigration()')
 })
 it('adding lookup registration never changes where downloads go',()=>{
  const add=body('addSearchDirectory')
  expect(add).toContain('service.pickSearchDirectory()')
  expect(add).toContain('if (!answer) return')
  expect(add).toContain('describeSearchDirectory(answer)')
  expect(add).not.toContain('setSaveDir')
  expect(add).not.toContain('pushSaveDir')
 })
})
