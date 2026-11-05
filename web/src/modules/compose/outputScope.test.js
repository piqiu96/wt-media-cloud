import { describe, it, expect } from 'vitest'
import { materialOutputQuery } from './outputScope.js'
describe('material output drilldown', () => {
 it('preserves applied business filters and replaces aggregation pagination', () => {
  const scope={search:'教程',team_id:2,operator_id:7,category_id:4,platform:'bilibili',video_status:'ready',created_from:'2026-11-01',created_before:'2026-11-06',page:3,page_size:20}
  expect(materialOutputQuery(scope,161,1,10)).toEqual({...scope,material_id:161,page:1,page_size:10})
  expect(materialOutputQuery({},161,2,10)).toEqual({material_id:161,page:2,page_size:10})
  expect(scope.page).toBe(3)
 })
})
