import { describe, expect, it, vi } from 'vitest'
import { useStrategyDirectory } from './strategyDirectory.js'
const deferred = () => { let resolve; const promise = new Promise(r => { resolve = r }); return {promise, resolve} }
describe('remote strategy directory', () => {
 it('requests bounded pages with exact context and searches beyond the initial directory', async () => {
  const client = {listStrategies: vi.fn().mockResolvedValueOnce({items:[{id:1,name:'First'}],total:201}).mockResolvedValueOnce({items:[{id:201,name:'Old'}],total:201}).mockResolvedValueOnce({items:[{id:201,name:'Old'}],total:1})}
  const state = useStrategyDirectory(client), context={team_id:2,category_id:3,operator_id:4,status:'enabled'}
  await state.load(context); expect(state.hasMore.value).toBe(true)
  await state.more(); expect(client.listStrategies.mock.calls[1][0]).toEqual({...context,search:'',page:2,page_size:20})
  await state.load(context,' Old '); expect(client.listStrategies.mock.calls[2][0]).toEqual({...context,search:'Old',page:1,page_size:20})
  expect(state.rows.value).toEqual([{id:201,name:'Old'}]); expect(state.hasMore.value).toBe(false)
 })
 it('keeps numeric ID matches whose display name does not contain the search term', async () => {
  const client={listStrategies:vi.fn().mockResolvedValue({items:[{id:225,name:'Original named strategy'}],total:1})}
  const state=useStrategyDirectory(client);await state.load({},'225')
  expect(client.listStrategies).toHaveBeenCalledWith({search:'225',page:1,page_size:20})
  expect(state.rows.value).toEqual([{id:225,name:'Original named strategy'}])
 })
 it('resolves the selected full configuration by ID even outside loaded pages', async () => {
  const detail={id:225,name:'Original',output_settings:[{key:'a',copies:2,params:{end:4}}]}
  const client={getStrategy:vi.fn().mockResolvedValue(detail)}
  const state=useStrategyDirectory(client); await state.resolve(225)
  expect(client.getStrategy).toHaveBeenCalledWith(225); expect(state.selected.value).toEqual(detail); expect(state.rows.value).toEqual([])
 })
 it('ignores obsolete responses when search or assignment context changes', async () => {
  const old=deferred(), fresh=deferred(), detail=deferred()
  const state=useStrategyDirectory({listStrategies:vi.fn().mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise),getStrategy:()=>detail.promise})
  const first=state.load({team_id:1}), second=state.load({team_id:2}), selection=state.resolve(4)
  fresh.resolve({items:[{id:2}],total:1}); await second
  old.resolve({items:[{id:1}],total:200}); await first; expect(state.rows.value).toEqual([{id:2}])
  state.invalidate(); detail.resolve({id:4}); await selection; expect(state.selected.value).toBe(null); expect(state.rows.value).toEqual([])
 })
 it('preserves loaded choices on a failed subsequent page and supports retry', async () => {
  const state=useStrategyDirectory({listStrategies:vi.fn().mockResolvedValueOnce({items:[{id:1}],total:21}).mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({items:[{id:21}],total:21})})
  await state.load(); await state.more(); expect(state.rows.value).toEqual([{id:1}]);expect(state.error.value).toBe('offline')
  await state.more(); expect(state.rows.value.map(row=>row.id)).toEqual([1,21])
 })
})
