import { operationURL } from '../publishing-design/routes.js'
export const accountScenes = [
 {id:'readonly',group:'资料',label:'默认只读',query:''},
 {id:'edit',group:'资料',label:'编辑资料',query:'edit=profile'},
 {id:'avatar',group:'资料',label:'选择头像',query:'edit=profile&avatar=open'},
 {id:'saving',group:'资料',label:'保存中',query:'edit=profile&state=saving'},
 {id:'save-error',group:'资料',label:'保存失败',query:'edit=profile&state=profile-error'},
 {id:'unbound-ready',group:'绑定',label:'未绑定 · 已读取账号',query:'state=unbound-ready'},
 {id:'unbound',group:'绑定',label:'未绑定 · 未读取账号',query:'state=unbound'},
 {id:'unverified',group:'绑定',label:'已绑定 · 未读取本机账号',query:'state=unverified'},
 {id:'offline',group:'绑定',label:'本机服务未连接',query:'state=offline'},
 {id:'mismatch',group:'绑定',label:'当前账号不一致',query:'state=mismatch'},
 {id:'bind-confirm',group:'确认',label:'首次绑定确认',query:'state=unbound-ready&dialog=bind'},
 {id:'rebind-confirm',group:'确认',label:'换绑确认',query:'state=mismatch&dialog=rebind'},
 {id:'leave-confirm',group:'确认',label:'放弃修改确认',query:'edit=profile&dialog=leave'},
 {id:'web',group:'访问范围',label:'网页端',query:'client=web'},
 {id:'admin',group:'访问范围',label:'管理员',query:'role=admin'},
]
export function accountSceneURL(id){const scene=accountScenes.find(s=>s.id===id)||accountScenes[0];return operationURL('personal')+'?review=account'+(scene.query?'&'+scene.query:'')}
