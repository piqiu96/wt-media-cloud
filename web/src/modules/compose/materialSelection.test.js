import { describe, expect, it } from 'vitest'
import { materialRow, mergeSelection, acceptSelectedMaterials } from './materialSelection.js'
describe('owner material selection', () => {
  it('retains cross-page drafts, removes page deselections and deduplicates source aliases', () => {
    const old={1:{id:1,title:'old'},3:{id:3}}
    expect(mergeSelection(old,[{id:1,title:'updated'},{id:2}], [1,2])).toEqual({1:{id:1,title:'updated'},2:{id:2},3:{id:3}})
    expect(mergeSelection(old,[{id:1}],[])).toEqual({3:{id:3}})
    expect(materialRow({id:88,status:'active',material:{id:1,title:'same'}})).toEqual({id:1,title:'same',usage_id:88,owner_usage_status:'active'})
  })
  it('bounds concurrent joins and separates exact accepted and failed material IDs', async () => {
    let active=0, peak=0
    const result=await acceptSelectedMaterials(Array.from({length:7},(_,i)=>({id:i+1})),async(id)=>{
      active++; peak=Math.max(peak,active)
      await new Promise(resolve=>setTimeout(resolve,1))
      active--
      if(id===3) throw new Error('来源不可用')
    },2)
    expect(peak).toBeLessThanOrEqual(2)
    expect(result.accepted.map(row=>row.id).sort()).toEqual([1,2,4,5,6,7])
    expect(result.failed).toEqual([{id:3,selection_error:'来源不可用'}])
  })
})
