// Only explicit user actions retry. A lost acknowledgement is never a failed write.
function failurePhase(error, fallback) {
  if (error?.code === "PROFILE_PROXY_PENDING") return "apply-error"
  if (error?.errcode === 11003) return 'forbidden'
  if (error?.errcode === 23002) return 'identity'
  if (error?.errcode === 23008) return 'unbound'
  if (error?.errcode === 20004 || error?.errcode === 20009) return 'expired'
  return fallback
}
function explicitRefusal(error) {
  return [11001, 11003, 20003, 20004, 20009, 23002, 23008, 23009].includes(error?.errcode)
}
export function createProfileSyncWorkflow({ scan, submit, confirm, review, saved, preview, change }) {
  let busy = false
  let phase = 'idle'
  let current = null
  let receipt = null
  const update = (next, error = '') => { phase = next; change({ phase, error }) }
  async function displaySaved(value) {
    receipt = value
    try { await saved(receipt); update('result') }
    catch (error) { update('reload-error', error.message || '同步已保存，但列表加载失败。') }
  }
  async function guarded(action) {
    if (busy) return
    busy = true
    try { await action() } finally { busy = false }
  }
  return {
    start: () => guarded(async () => {
      current = null; receipt = null; update('reading')
      let snapshot
      try {
        snapshot = await scan()
        if (!snapshot) throw new Error('已离开本人窗口视图，本次同步已停止。')
      } catch (error) { update(failurePhase(error, 'read-error'), error.message || '读取本机窗口失败'); return }
      if (!snapshot.profiles?.length) { update('empty'); return }
      update('comparing')
      try { current = await submit(snapshot); preview(current, snapshot); update('preview') }
      catch (error) { update(failurePhase(error, 'submit-error'), error.message || '提交本机窗口失败') }
    }),
    confirm: selection => guarded(async () => {
      if (!current || !['preview', 'apply-error'].includes(phase)) return
      update('applying')
      let result
      try { result = await confirm(current.id, [...selection]) }
      catch (error) { update(failurePhase(error, explicitRefusal(error) ? 'apply-error' : 'uncertain'), error.message || '暂时无法确认保存结果'); return }
      await displaySaved(result)
    }),
    verify: () => guarded(async () => {
      if (!current || phase !== 'uncertain') return
      update('verifying')
      try {
        const result = await review()
        if (result.id !== current.id) { update('expired', '同步快照已被其他页面更新，请重新同步。'); return }
        if (result.status === 'confirmed') await displaySaved(result)
        else if (result.status === 'ready') update('apply-error', '当前快照尚未保存，可以重试确认。')
        else update('uncertain', '暂时无法确认保存结果，请稍后再次核实。')
      } catch (error) { update(failurePhase(error, 'uncertain'), error.message || '暂时无法读取保存结果') }
    }),
    reload: () => guarded(async () => {
      if (!receipt || phase !== 'reload-error') return
      update('reloading'); await displaySaved(receipt)
    }),
  }
}
