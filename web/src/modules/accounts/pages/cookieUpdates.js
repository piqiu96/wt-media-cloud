// The server grants each external write once; replay/unknown flows are read-only.
function localArgs(receipt, stage) {
  return {stage,operationId:receipt.operation_id,bitProfileId:receipt.bit_profile_id,platform:receipt.platform,expectedPlatformAccountId:receipt.expected_platform_account_id || '',inputCookie:receipt.input_cookie || '',fingerprint:receipt.fingerprint || ''}
}
function resultPayload(r) {
  return {verified:!!r.verified,status:r.status,applied:!!r.applied,cookies:r.cookies || [],platform_account_id:r.platform_account_id || '',name:r.name || '',avatar_url:r.avatar_url || '',login_status:r.login_status || '',input_matches:!!r.input_matches,message:r.message || ''}
}
export async function executeCookieUpdate(client,local,id,input,onReceipt=()=>{}) {
  let receipt=await client.startCookieUpdate(id,input);onReceipt(receipt)
  if (!receipt.allow_execute) return receipt
  try {
    const prepared=await local.accountCookie(localArgs(receipt,'prepare'))
    receipt=await client.snapshotCookieUpdate(id,receipt.operation_id,{before_cookie:prepared.before_cookie,fingerprint:prepared.fingerprint,login_status:prepared.login_status,platform_account_id:prepared.platform_account_id || ''});onReceipt(receipt)
  } catch(e) {
    const message=e?.code==='account_cookie_input_invalid' ? 'Cookie 格式不正确，请检查后重试' : '窗口状态检查未完成，本次未更新，请重试'
    return client.settleCookieUpdate(id,receipt.operation_id,{status:'failed',applied:false,message})
  }
  if (!receipt.can_apply) return receipt
  let result
  try { result=await local.accountCookie(localArgs(receipt,'apply')) }
  catch { result={status:'uncertain',applied:true,message:'更新结果待确认，请核实当前登录状态'} }
  return client.settleCookieUpdate(id,receipt.operation_id,resultPayload(result))
}
export async function reconcileCookieUpdate(client,local,id,operationId) {
  const receipt=await client.getCookieUpdate(id,operationId)
  if (['succeeded','failed'].includes(receipt.status)) return receipt
  // No snapshot has been authorized yet, therefore this operation cannot have written.
  if (receipt.step==='preparing') return client.settleCookieUpdate(id,operationId,{status:'failed',applied:false,message:'本次尚未更新 Cookie，可重新发起'})
  const result=await local.accountCookie(localArgs(receipt,'inspect'))
  return client.settleCookieUpdate(id,operationId,resultPayload(result))
}
