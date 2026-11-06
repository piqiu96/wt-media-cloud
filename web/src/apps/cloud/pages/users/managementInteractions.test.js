import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { usePendingActions, useEditorClose, useReferenceDetails } from './managementInteractions.js'
const deferred = () => { let resolve, reject; const promise = new Promise((a,b) => { resolve=a;reject=b }); return {promise,resolve,reject} }

describe('system management interactions', () => {
 it('locks a row before confirmation and releases it after failure', async () => {
  const state=usePendingActions(), wait=deferred(), action=vi.fn(()=>wait.promise)
  const first=state.run(1,action)
  await state.run(1,action)
  expect(action).toHaveBeenCalledTimes(1);expect(state.pending.value.has(1)).toBe(true)
  wait.reject(new Error('failed'));await expect(first).rejects.toThrow('failed')
  expect(state.pending.value.size).toBe(0)
 })
 it('keeps drafts when discard is cancelled and prevents closing during save',async()=>{
  const visible=ref(true),busy=ref(false),draft=ref('before'),confirm=vi.fn().mockResolvedValue(false)
  const guard=useEditorClose({visible,busy,draft:()=>draft.value,confirm})
  guard.capture();draft.value='after';await guard.close();expect(visible.value).toBe(true)
  busy.value=true;await guard.close();expect(confirm).toHaveBeenCalledTimes(1)
  busy.value=false;confirm.mockResolvedValue(true);await guard.close();expect(visible.value).toBe(false)
 })
 it('keeps failure distinct from a successful empty reference list and supports retry', async()=>{
  const load=vi.fn().mockRejectedValueOnce(new Error('读取失败')).mockResolvedValue({users:[],media_accounts:[]})
  const state=useReferenceDetails(load)
  await state.open({id:4,name:'游戏'});expect(state.error.value).toBe('读取失败');expect(state.data.value).toBeNull()
  await state.reload();expect(state.error.value).toBe('');expect(state.data.value.users).toEqual([])
 })
 it('ignores old requests after another category opens', async()=>{
  const old=deferred(),load=vi.fn().mockReturnValueOnce(old.promise).mockResolvedValue({users:[{user_id:2}]})
  const state=useReferenceDetails(load)
  const request=state.open({id:1});await state.open({id:2})
  old.resolve({users:[{user_id:1}]});await request
  expect(state.data.value.users[0].user_id).toBe(2);expect(state.loading.value).toBe(false)
 })
})
