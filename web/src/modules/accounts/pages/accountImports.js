// Cloud owns progress. This controller executes only a fresh per-stage grant.
export async function runImportItem({api,local,profiles,batch,row,reconcile=false,onChange=()=>{}}) {
 let current=batch
 const update=v=>{current=v;onChange(v);return v}
 const result=(step,fields={})=>api.result(batch.id,row,{expected_revision:current.revision,attempt:current.execution.attempt,step,...fields}).then(update)
 const cookieArgs=i=>({operationId:i.operation_id,bitProfileId:i.bit_profile_id,platform:batch.platform,inputCookie:i.input_cookie,fingerprint:i.fingerprint||'',manualLogin:i.step==='manual'})
 async function register(created) {
  const sync=await profiles.submit(created.snapshot)
  await profiles.confirm(sync.id||sync.scan_id,[created.bit_profile_id])
  const all=await profiles.listProfiles()
  const target=(Array.isArray(all)?all:all.items||[]).find(p=>p.bit_profile_id===created.bit_profile_id)
  if(!target)throw new Error('窗口已创建，请刷新后核实')
  return result('created',{browser_profile_id:String(target.id)})
 }
 const started=reconcile ? await api.action(batch.id,row,{expected_revision:batch.revision,action:'reconcile'}) : await api.start(batch.id,row,{expected_revision:batch.revision})
 update(started)
 if(reconcile&&!started.can_inspect||!reconcile&&!started.allow_execute)return current
 let i=current.execution
 if(i.step==='creating') {
  let created
  try {created=await local.profileCreate({name:`账号接入 ${batch.id}-${row}`,group_id:batch.content_json.group_id,creation_marker:`WT_IMPORT:${batch.id}:${row}:${i.attempt}`,inspect_only:reconcile})}
  catch {return result('creation_unknown')}
  if(created.status==='unavailable'&&!reconcile)return result('creation_failed')
  if(!created.bit_profile_id||['uncertain','unavailable'].includes(created.status))return result('creation_unknown')
  await register(created)
  if(!current.allow_execute)return current
  i=current.execution
 } else if(reconcile) {
  // Read-only on every recovery, including a lost apply response.
  let observed
  try {observed=await local.accountCookie({stage:'inspect',...cookieArgs(i)})}
  catch {return result('uncertain')}
  return result('completed',observed)
 }
 if(i.step!=='preparing')return current
 let before
 try {before=await local.accountCookie({stage:'prepare',...cookieArgs(i)})}
 catch {return result('preparation_failed')}
 await result('prepared',{before_cookie:before.before_cookie,fingerprint:before.fingerprint,login_status:before.login_status,platform_account_id:before.platform_account_id||''})
 if(!current.can_apply)return current
 i=current.execution
 let observed
 try {observed=await local.accountCookie({stage:'apply',...cookieArgs(i)})}
 catch {return result('uncertain')}
 return result('completed',observed)
}
