import {readFileSync} from 'node:fs'
import {describe,it,expect} from 'vitest'
const source=readFileSync(new URL('./apps/desktop/features/local-settings/LocalSettingsPage.vue',import.meta.url),'utf8')
describe('file organize wiring',()=>{
 it('uses cloud download records and native plan instead of accepting arbitrary paths',()=>{
  expect(source).toContain('await loadCandidateNames()')
  expect(source).toContain('service.migrationPlan(candidateNames)')
  expect(source).not.toContain('local_path')
  expect(source).toContain('migrationRows(migrationPlan.value, migrationChoices.value)')
 })
 it('defaults to keep and sends only selected names to separate actions',()=>{
  expect(source).toContain('migrationChoices.value = {}')
  for(const action of ['move','delete']){
   expect(source).toContain(`chosenNames(migrationRowsFor.value, FILE_CHOICES.${action})`)
   expect(source).toContain(`@click="runMigration(FILE_CHOICES.${action})"`)
  }
  expect(source).toContain('service.deleteSavedFiles(names)')
  expect(source).toContain('service.moveSavedFiles(names)')
  expect(source).toContain('feedback.confirmWarning')
 })
 it('keeps actual partial results and pending selections visible for retry',()=>{
  expect(source).toContain('migrationResult.value = describeMigration(report, action)')
  expect(source).toContain('!completed.has(name)')
  expect(source).toContain('v-if="migrationError"')
  expect(source).toContain('v-if="migrationResult"')
  expect(source).toContain('!migrationLoading && !migrationError')
 })
})
