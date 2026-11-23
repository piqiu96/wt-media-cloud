<script setup>
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { currentPrototypeUser, currentPrototypeProfile } from '../publishing-design/prototypeSession.js'
import { operationURL } from '../publishing-design/routes.js'
import { accountScenes, accountSceneURL } from './scenes.js'
import ResourceEmptyState from '../../../src/shared/ui/resource/ResourceEmptyState.vue'
import { DescriptionsItem as Field, MessagePlugin } from 'tdesign-vue-next'
import { ChevronDownIcon, RefreshIcon, FolderOpenIcon, FileIcon, AddIcon, UserCircleIcon } from 'tdesign-icons-vue-next'
import ResourceCard from '../../../src/shared/ui/resource/ResourceCard.vue'
import ResourcePageHeader from '../../../src/shared/ui/resource/ResourcePageHeader.vue'
import ResourceSection from '../../../src/shared/ui/resource/ResourceSection.vue'
import ResourceDescriptions from '../../../src/shared/ui/resource/ResourceDescriptions.vue'
import ResourceForm from '../../../src/shared/ui/resource/ResourceForm.vue'
import ResourceRowActions from '../../../src/shared/ui/resource/ResourceRowActions.vue'
import ResourceFieldHelp from '../../../src/shared/ui/resource/ResourceFieldHelp.vue'
import BusinessStatus from '../../../src/shared/ui/BusinessStatus.vue'
import SystemAvatar from '../../../src/shared/ui/SystemAvatar.vue'
import FeedbackAlert from '../../../src/shared/ui/feedback/FeedbackAlert.vue'
import ResourceOverlay from '../../../src/shared/ui/resource/ResourceOverlay.vue'
const query = new URLSearchParams(location.search)
const props=defineProps({page:{type:String,default:'personal'}})
const emit=defineEmits(['navigate'])
const user=computed(currentPrototypeUser)
const profile=computed(currentPrototypeProfile)
const tab = ref(query.get('tab') === 'log' ? 'log' : 'file')
const editing = ref(query.get('edit') === 'profile')
const savedNickname = ref(profile.value.nickname), savedAvatar = ref(profile.value.avatar)
const nickname = ref(savedNickname.value), avatar = ref(savedAvatar.value), avatarOpen = ref(query.get('avatar') === 'open')
const desktop = query.get('client') !== 'web'
const scene = query.get('state') || ''
const admin = computed(()=>user.value.role === '管理员')
const bindingState = ref(scene === 'mismatch' ? 'mismatch' : scene.startsWith('unbound') ? 'unbound' : 'bound')
const boundAccount = ref('2c9b****619f')
const localAccount = ref(['unbound','unverified','offline'].includes(scene) ? '' : scene === 'mismatch' ? '7d42****8a10' : '2c9b****619f')
const profileError = ref(scene === 'profile-error' ? '保存失败，请重试。当前输入已保留。' : '')
const profileSaving = ref(scene === 'saving')
const bindingSummary = computed(() => !desktop ? '绑定与换绑需在桌面端操作' : scene === 'offline' ? '本机执行服务未连接，暂时无法读取比特账号' : !localAccount.value ? '尚未读取到本机比特浏览器账号' : bindingState.value === 'unbound' ? `当前比特账号 ${localAccount.value}，确认后绑定` : bindingState.value === 'mismatch' ? '当前比特浏览器登录的是另一个账号' : '当前比特浏览器已登录绑定账号')

const dirty = computed(() => nickname.value !== savedNickname.value || avatar.value !== savedAvatar.value)
const path = ref(query.get('state') === 'no-directory' ? '' : '/Users/aqiuye/Develop/workspace/wt-media/wt-media-download')
const configFile = '/Users/aqiuye/Library/Application Support/WTMedia/Desktop/settings.toml'
const pendingApply = ref(query.get('state') === 'unapplied')
const history = ref(['/Users/aqiuye/Downloads/WTMedia', '/Volumes/Media/WTMedia'])
const picker = ref(''), picked = ref(''), confirmation = ref(null), exported = ref(false)
const pickerOptions = ['/Users/aqiuye/Movies/WTMedia', '/Volumes/Media/Download', '/Users/aqiuye/Downloads/WTMedia']
const avatarOptions = [['sky','蓝天'],['ocean','海洋'],['sun','朝阳'],['mountain','远山'],['flower','花朵'],['star','星光']]
const storage = ref([{name:'缓存',size:'8.6 GB',source:'cache'}, {name:'本机控制台日志',size:'1.6 MB',source:'desktop'}, {name:'本机执行服务日志',size:'2.6 MB',source:'agent'}])
const storageColumns = [{colKey:'name',title:'内容'}, {colKey:'size',title:'占用空间',width:140}, {colKey:'action',title:'操作',width:180}]
const logGroups = [
 {source:'desktop',title:'本机控制台',files:[{name:'desktop.log',size:'1.4 KB',live:true,time:'19:27'}, {name:'desktop.log.2026-11-22-18',size:'837 B',live:false,time:'18:40'}, {name:'desktop.log.2026-11-22-17',size:'2.4 KB',live:false,time:'13:44'}, {name:'desktop.log.2026-11-22-12',size:'497 B',live:false,time:'12:44'}]},
 {source:'agent',title:'本机执行服务',files:[{name:'agent.log',size:'4.2 KB',live:true,time:'19:27'}, {name:'agent.log.2026-11-22',size:'18 KB',live:false,time:'18:20'}]}
]
const source = ref('desktop'), file = ref('desktop.log'), level = ref('all'), tailCount = ref(500)
const activeFile = computed(() => logGroups.find(g => g.source === source.value).files.find(f => f.name === file.value))
const logPath = computed(() => `/Users/aqiuye/Library/Logs/WTMedia/${source.value === 'desktop' ? 'Desktop' : 'Agent'}/${file.value}`)
const desktopLines = [
 ['info','2026-11-22 19:20:53.156 [INFO] desktop.startup: 配置来自应用设置；环境 prod'],
 ['info','2026-11-22 19:21:24.012 [INFO] agent.supervisor: 随应用的 Local Agent 校验通过'],
 ['info','2026-11-22 19:21:24.085 [INFO] agent.supervisor: Local Agent 已启动 (sidecar_started)'],
 ['info','2026-11-22 19:21:28.204 [INFO] agent.supervisor: Local Agent 已就绪，local API listening on 127.0.0.1:8765'],
 ['info','2026-11-22 19:26:55.067 [INFO] desktop.startup: 下载保存位置已读取'],
 ['warn','2026-11-22 19:27:04.312 [WARN] bitbrowser.status: 尚未读取到当前登录账号'],
 ['plain','  请确认比特浏览器已启动并登录，然后在工作环境中重新检查。'],
 ['info','2026-11-22 19:27:29.433 [INFO] local.log: 日志读取完成']]
const agentLines = [['info','2026-11-22 19:21:28.204 [INFO] agent.startup: Local Agent 已就绪'], ['info','2026-11-22 19:26:55.067 [INFO] local.file: 下载保存位置已应用'], ['warn','2026-11-22 19:27:04.312 [WARN] bitbrowser.status: 尚未读取到当前登录账号'], ['plain','  请确认比特浏览器已启动并登录。']]
const logLines = computed(() => (source.value === 'desktop' ? desktopLines : agentLines).slice(0, activeFile.value.live ? undefined : 3))
const visibleLines = computed(() => logLines.value.filter(([severity]) => severity === 'plain' || level.value === 'all' || ({trace:0,debug:1,info:2,warn:3,error:4}[severity] >= {trace:0,debug:1,info:2,warn:3,error:4}[level.value])))
const migration = ref(query.get('dialog') === 'migration'), migrationDone = ref(false)
const migrationRows = ref([{id:1,name:'火影忍者_运营素材_01.mp4',directory:'/Users/aqiuye/Downloads/WTMedia',size:'128 MB',state:'可搬运',choice:'keep'}, {id:2,name:'三角洲_成片_02.mp4',directory:'/Volumes/Media/WTMedia',size:'86 MB',state:'可搬运',choice:'keep'}, {id:3,name:'火影忍者_成片_03.mp4',directory:'/Volumes/Media/WTMedia',size:'64 MB',state:'目标目录同名',choice:'keep'}])
const migrationColumns = [{colKey:'name',title:'文件'}, {colKey:'size',title:'大小',width:88}, {colKey:'state',title:'状态',width:122}, {colKey:'choice',title:'处理方式',width:224}]
const chosenCount = computed(() => migrationRows.value.filter(r => r.choice !== 'keep').length)
function demo() { MessagePlugin.info('设计演示：未操作真实文件或账号') }
function cancelProfile() { nickname.value=savedNickname.value; avatar.value=savedAvatar.value; editing.value=false; avatarOpen.value=false }
function saveProfile() { if(profileSaving.value)return;profileError.value='';savedNickname.value=nickname.value.trim(); savedAvatar.value=avatar.value;profile.value.nickname=savedNickname.value;profile.value.avatar=savedAvatar.value; editing.value=false; avatarOpen.value=false; MessagePlugin.success('已保存演示资料') }
let leaveDecision=null
function confirmLeave(){
 if(profileSaving.value){MessagePlugin.info('资料保存中，请稍候');return Promise.resolve(false)}
 if(!editing.value||!dirty.value)return Promise.resolve(true)
 return new Promise(resolve=>{leaveDecision=resolve;confirmation.value={kind:'leave',title:'放弃未保存的修改？',message:'离开后，尚未保存的昵称与头像修改将丢失。'}})
}
function cancelOperation(){confirmation.value=null;if(leaveDecision){leaveDecision(false);leaveDecision=null}}
function showLog(s){source.value=s;file.value=s==='desktop'?'desktop.log':'agent.log';tab.value='log'}
function protectUnload(event){if(editing.value&&dirty.value||profileSaving.value){event.preventDefault();event.returnValue=''}}
onMounted(()=>window.addEventListener('beforeunload',protectUnload))
onBeforeUnmount(()=>{window.removeEventListener('beforeunload',protectUnload);leaveDecision?.(false)})
defineExpose({confirmLeave})
const sceneSelection=accountScenes.find(s=>s.query===[...query].filter(([key])=>!['page','review'].includes(key)).map(([key,value])=>`${key}=${value}`).join('&'))?.id||'readonly'
function selectScene(id){profileSaving.value=false;cancelProfile();emit('navigate',accountSceneURL(id))}
function selectLog(s,n) { source.value=s;file.value=n }
function openPicker(kind) { picker.value=kind;picked.value=pickerOptions.find(p=>kind==='download'?p!==path.value:!history.value.includes(p)&&p!==path.value)||pickerOptions[0] }
function chooseDirectory() {
 if(picker.value==='download') { history.value=[...new Set([...history.value,path.value].filter(Boolean))].filter(p=>p!==picked.value);path.value=picked.value;pendingApply.value=false }
 else if(picked.value!==path.value&&!history.value.includes(picked.value)) history.value.push(picked.value)
 picker.value='';MessagePlugin.success('已更新演示目录')
}
function removeHistory(dir) { confirmation.value={kind:'remove',title:'移除此查找目录？',message:`不再查找 ${dir} 中的文件，相关旧文件可能显示未找到。实际文件不会删除，重新添加可恢复。`,dir} }
function clean(row) { if(row.source==='cache'){row.size='0 B';MessagePlugin.success('演示结果：缓存已清理')} else confirmation.value={kind:'clean',title:'清理已轮转日志？',message:`将清理${row.name}中已轮转的日志，历史排障记录将不可恢复；正在写入的日志保持不变。`,row} }
function runMigration() { if(migrationRows.value.some(r=>r.choice==='delete')) confirmation.value={kind:'delete-files',title:'删除所选文件？',message:'所选删除项会从原目录移除且不可恢复。搬运项将移到当前下载目录，保留项不处理。'}; else migrationDone.value=true }
function confirmOperation() { const c=confirmation.value;confirmation.value=null;if(c.kind==='remove')history.value=history.value.filter(p=>p!==c.dir);if(c.kind==='clean'){c.row.size='1.4 KB';MessagePlugin.success('演示结果：已清理轮转日志，当前日志保留')}if(c.kind==='leave'){cancelProfile();leaveDecision?.(true);leaveDecision=null}if(['rebind','bind'].includes(c.kind)){boundAccount.value=localAccount.value;bindingState.value='bound';MessagePlugin.success('已更新演示绑定')}if(c.kind==='delete-files')migrationDone.value=true }
if(query.get('dialog')==='bind') confirmation.value={kind:'bind',title:'绑定当前比特账号？',message:`将本机读取到的 ${localAccount.value} 绑定到当前账号，之后使用该比特主账号下的窗口执行任务。`}
if(query.get('dialog')==='rebind') confirmation.value={kind:'rebind',title:'换绑为当前比特账号？',message:'将绑定从 2c9b****619f 改为 7d42****8a10。原有窗口记录保留，不删除或迁移；切回原账号可恢复。正在执行的任务不受影响。'}
if(query.get('dialog')==='leave'){nickname.value='运营昵称';confirmation.value={kind:'leave',title:'放弃未保存的修改？',message:'离开后，尚未保存的昵称与头像修改将丢失。',next:'settings'}}
if(scene==='profile-error')nickname.value='运营昵称'
</script>

<template>
<section class="account-settings-page">
    <div class="breadcrumbs">{{page==='personal'?'我的账号':'桌面设置'}}</div>
    <template v-if="page==='personal'">
      <ResourcePageHeader title="我的账号"><template v-if="query.get('review')==='account'" #actions><div class="account-review-control"><span>原型状态</span><t-select aria-label="原型状态" :value="sceneSelection" :options="accountScenes.map(s=>({label:s.group+' · '+s.label,value:s.id}))" @change="selectScene"/></div></template></ResourcePageHeader>
      <div class="personal-content">
        <ResourceCard class="detail-card">
          <ResourceSection title="账号资料">
            <div v-if="!editing" class="identity-row"><SystemAvatar :avatar-id="savedAvatar" :size="48"/><div class="identity"><strong>{{savedNickname}}</strong><span>用户 UID {{user.id}}</span></div><ResourceRowActions><t-button variant="text" @click="editing=true">编辑</t-button></ResourceRowActions></div>
            <ResourceForm v-else class="profile-form">
              <div class="profile-fields"><t-form-item label="昵称"><t-input v-model="nickname" :disabled="profileSaving" maxlength="64"/></t-form-item><t-form-item label="头像"><div class="avatar-control"><SystemAvatar :avatar-id="avatar" :size="36"/><ResourceRowActions><t-button variant="text" :disabled="profileSaving" @click="avatarOpen=!avatarOpen">更换头像<ChevronDownIcon/></t-button></ResourceRowActions></div></t-form-item></div>
              <div v-if="avatarOpen" class="avatar-list"><t-radio-group v-model="avatar" :disabled="profileSaving"><t-radio-button v-for="[id,label] in avatarOptions" :key="id" :value="id"><SystemAvatar :avatar-id="id" :size="28"/>{{label}}</t-radio-button></t-radio-group></div>
              <FeedbackAlert v-if="profileError" class="profile-error" theme="error" :message="profileError"/><div class="resource-form-actions"><t-button theme="default" :disabled="profileSaving" @click="cancelProfile">取消</t-button><t-button theme="primary" :loading="profileSaving" :disabled="!profileSaving&&(!nickname.trim()||!dirty)" @click="saveProfile">保存</t-button></div>
            </ResourceForm>
            <ResourceDescriptions><Field v-if="editing" label="用户 UID">{{user.id}}</Field><Field label="用户名">{{user.name}}</Field><Field label="角色">{{user.role}}</Field><Field label="所属团队">{{user.team||'—'}}</Field><Field label="类目范围">{{admin?'全部类目':user.categories.join('、')}}</Field></ResourceDescriptions>
          </ResourceSection>
        </ResourceCard>
        <ResourceCard v-if="!admin" class="browser-card detail-card">
          <ResourceSection title="比特浏览器" class="browser-section">
          <template #title-extra><span class="browser-binding-status"><BusinessStatus :status="bindingState==='unbound'?'unknown':'success'" :label="bindingState==='unbound'?'未绑定':'已绑定'"/></span></template>
          <div class="device-hero"><span class="device-icon"><UserCircleIcon/></span><div><strong>{{bindingState==='unbound'?'尚未绑定比特浏览器账号':'已绑定比特浏览器账号'}}</strong><span>{{bindingSummary}}</span></div></div>
          <dl class="device-details">
            <div><dt>绑定比特账号</dt><dd>{{bindingState==='unbound'?'—':boundAccount}}</dd></div>
            <div><dt>绑定时间</dt><dd>{{bindingState==='unbound'?'—':'2026/07/22 21:43:50'}}</dd></div>
            <div><dt>最近验证</dt><dd>{{bindingState==='unbound'?'—':'2026/11/22 20:30:00'}}</dd></div>
            <div v-if="desktop&&bindingState==='mismatch'"><dt>当前比特账号</dt><dd>7d42****8a10（与绑定账号不一致）</dd></div>
          </dl>
          <FeedbackAlert v-if="desktop&&bindingState==='mismatch'" class="device-alert" theme="warning" message="当前比特浏览器登录账号与系统绑定账号不一致，请切回原账号后重新检查。"><template #operation><t-button variant="text" @click="confirmation={kind:'rebind',title:'换绑为当前比特账号？',message:'将绑定从 2c9b****619f 改为 7d42****8a10。原有窗口记录保留，不删除或迁移；切回原账号可恢复。'}">换绑为当前账号</t-button></template></FeedbackAlert>
          <p v-if="!desktop" class="device-note">Cloud 网页可查看绑定状态；绑定与换绑需在桌面端操作。</p>
          <div v-else-if="bindingState==='unbound'" class="device-actions"><t-button v-if="localAccount" theme="primary" @click="confirmation={kind:'bind',title:'绑定当前比特账号？',message:`将本机读取到的 ${localAccount} 绑定到当前账号，之后使用该比特主账号下的窗口执行任务。`}">绑定比特浏览器账号</t-button><template v-else><p class="device-note">请先启动并登录比特浏览器，在工作环境中检查后再绑定。</p><t-button variant="text" @click="demo">去工作环境检查</t-button></template></div>
          <div v-else-if="!localAccount" class="device-actions"><t-button variant="text" @click="demo">去工作环境检查</t-button></div>
          </ResourceSection>
          <ResourceSection v-if="desktop" title="本机" variant="form" class="machine-section">
            <dl class="device-details">
              <div><dt>设备 ID</dt><dd>WT-2847231ba08be1a15df9d4229bb91dc3</dd></div>
              <div><dt>系统</dt><dd>macOS · arm64</dd></div>
              <div><dt>Agent 状态</dt><dd>{{scene==='offline'?'无法连接':'空闲'}}</dd></div>
              <div><dt>Agent 版本</dt><dd>{{scene==='offline'?'未读取到':'0.3.0'}}</dd></div>
            </dl>
          </ResourceSection>
        </ResourceCard>
      </div>
    </template>
    <template v-else-if="admin||!desktop"><ResourcePageHeader title="桌面设置"/><ResourceEmptyState title="桌面设置仅对桌面端运营开放" description="当前身份或客户端没有桌面设置入口。"><t-button theme="primary" @click="emit('navigate',operationURL('personal'))">返回我的账号</t-button></ResourceEmptyState></template>
    <template v-else>
      <ResourcePageHeader title="桌面设置"/>
      <div class="settings-tabs"><t-tabs v-model="tab"><t-tab-panel value="file" label="下载与存储"/><t-tab-panel value="log" label="本地日志"/></t-tabs></div>
      <ResourceCard v-if="tab==='file'" class="download-body detail-card">
        <ResourceSection title="系统配置文件" variant="form">
          <div class="directory-value"><FileIcon/><span class="path-value" data-testid="config-file-path">{{configFile}}</span></div>
        </ResourceSection>
        <ResourceSection title="下载目录" variant="form">
          <div class="directory-row"><div class="directory-value"><FolderOpenIcon/><span class="path-value">{{path||'尚未设置下载目录'}}</span></div><div class="resource-form-actions"><t-button theme="default" @click="openPicker('download')">{{path?'更改目录':'设置目录'}}</t-button><t-button v-if="path" theme="default" @click="demo"><template #icon><FolderOpenIcon/></template>打开目录</t-button></div></div>
          <p class="helper">新下载的素材与成片保存到此处，更改目录不会移动已有文件。</p>
          <FeedbackAlert v-if="pendingApply" theme="warning" message="目录已保存，尚未应用到执行服务。"><template #operation><t-button variant="text" @click="pendingApply=false">重试应用</t-button></template></FeedbackAlert>
        </ResourceSection>
        <ResourceSection title="历史文件查找目录" variant="form">
          <template #title-extra><ResourceFieldHelp label="历史文件查找目录" content="查找旧目录中的文件，不改变下载目录。移除后相关旧文件可能显示未找到，文件本身不会删除。"/><ResourceRowActions><t-button variant="text" @click="openPicker('history')"><AddIcon/>添加目录</t-button></ResourceRowActions></template>
          <div v-for="dir in history" :key="dir" class="history-row"><div class="directory-value"><FolderOpenIcon/><span class="path-value">{{dir}}</span></div><ResourceRowActions><t-button variant="text" @click="demo">打开</t-button><t-button variant="text" @click="removeHistory(dir)">移除</t-button></ResourceRowActions></div>
          <t-empty v-if="!history.length" size="small" description="暂无历史查找目录"/>
        </ResourceSection>
        <div class="migration-entry"><div><strong>整理已有文件</strong></div><div class="resource-form-actions"><t-button theme="default" :disabled="!path" @click="migration=true;migrationDone=false">整理文件</t-button></div></div>
        <ResourceSection title="存储与清理" variant="form">
          <template #title-extra><span class="section-meta">下载磁盘可用 47 GB</span><ResourceRowActions><t-button variant="text" @click="demo"><RefreshIcon/>刷新</t-button></ResourceRowActions></template>
          <t-table row-key="source" class="resource-data-table resource-embedded-table" :data="storage" :columns="storageColumns" size="small"><template #action="{row}"><ResourceRowActions><t-button v-if="row.source!=='cache'" variant="text" @click="showLog(row.source)">查看日志</t-button><t-button variant="text" @click="clean(row)">清理</t-button></ResourceRowActions></template></t-table>
        </ResourceSection>
      </ResourceCard>
      <ResourceCard v-else class="logs-card">
        <div class="log-toolbar"><div><label>最低级别</label><t-select v-model="level" :options="[{label:'全部',value:'all'},{label:'调试及以上',value:'debug'},{label:'信息及以上',value:'info'},{label:'警告及以上',value:'warn'},{label:'错误',value:'error'}]"/></div><div><label>读取尾部</label><t-select v-model="tailCount" :options="[500,1000,5000].map(v=>({label:v+' 行',value:v}))"/></div><ResourceRowActions><t-button variant="text" @click="demo"><RefreshIcon/>刷新</t-button><t-button variant="text" @click="demo"><FolderOpenIcon/>打开日志文件夹</t-button></ResourceRowActions><div class="resource-form-actions"><t-button theme="primary" @click="exported=true">导出诊断包</t-button></div></div>
        <div class="log-split">
          <aside class="file-list" aria-label="日志文件">
            <div v-for="group in logGroups" :key="group.source" class="file-group"><h3>{{group.title}}</h3><button v-for="f in group.files" :key="f.name" class="log-file-item" :class="{'is-selected':source===group.source&&file===f.name}" :aria-pressed="source===group.source&&file===f.name" @click="selectLog(group.source,f.name)"><span class="file-title"><FileIcon/>{{f.name}}</span><span class="file-meta"><BusinessStatus :status="f.live?'running':'archived'" :label="f.live?'正在写入':'已归档'"/><span>{{f.size}} · {{f.time}}</span></span></button></div>
          </aside>
          <section class="log-reader" aria-label="日志正文"><div class="reader-heading"><strong>{{file}}</strong><ResourceFieldHelp label="日志路径" :content="logPath"/></div><div class="log-meta"><span>读取 {{logLines.length}} 行，显示 {{visibleLines.length}} 行</span><span>尾部 {{tailCount}} 行以内 · {{activeFile.size}}</span></div>
            <div class="log-view"><div v-for="([severity,line],i) in visibleLines" :key="line" class="log-line" :class="severity"><span class="line-number">{{i+1}}</span><code>{{line}}</code></div><t-empty v-if="!visibleLines.length" size="small" description="没有符合当前级别的日志"><template #action><t-button theme="primary" variant="text" @click="level='all'">重置级别</t-button></template></t-empty></div>
            <div class="log-footer"><span>保留原文、续行及未分级内容</span><ResourceFieldHelp label="诊断包" content="包含版本、组件状态、脱敏日志及失败任务摘要；仅作排障，不重新执行任务。"/></div>
          </section>
        </div>
      </ResourceCard>
    </template>
  <ResourceOverlay :visible="!!picker" :title="picker==='download'?'选择下载目录':'添加查找目录'" @update:visible="v=>{if(!v)picker=''}" form-actions>
    <p class="helper overlay-help">目录选择演示；正式页面使用系统目录选择器。选择后仅更新原型数据。</p><ResourceForm><t-form-item label="目录"><t-select v-model="picked" :options="pickerOptions.map(v=>({label:v,value:v}))"/></t-form-item></ResourceForm>
    <template #footer><t-button theme="default" @click="picker=''">取消</t-button><t-button theme="primary" @click="chooseDirectory">{{picker==='download'?'选择并保存':'添加'}}</t-button></template>
  </ResourceOverlay>
  <ResourceOverlay :visible="migration" title="整理已有文件" :width="960" @update:visible="migration=$event" form-actions>
    <template v-if="!migrationDone"><p class="migration-target">搬运目标：{{path}}</p><t-table row-key="id" class="resource-data-table resource-embedded-table" :data="migrationRows" :columns="migrationColumns"><template #name="{row}"><div>{{row.name}}</div><p class="helper">{{row.directory}}</p></template><template #state="{row}"><BusinessStatus :status="row.id===3?'warning':'normal'" :label="row.state"/></template><template #choice="{row}"><t-radio-group v-model="row.choice" variant="default-filled"><t-radio-button value="keep">保留</t-radio-button><t-radio-button value="move" :disabled="row.id===3">搬运</t-radio-button><t-radio-button value="delete">删除</t-radio-button></t-radio-group></template></t-table><p class="helper cleanup-note">默认保留；同名冲突不能搬运，不自动覆盖或重命名。删除执行前将再次确认。</p></template>
    <div v-else class="migration-result"><BusinessStatus status="success" label="演示处理完成"/><p>已处理 {{chosenCount}} 个文件，其他文件保持不变。</p></div>
    <template #footer><t-button theme="default" @click="migration=false">关闭</t-button><t-button v-if="!migrationDone" theme="primary" :disabled="!chosenCount" @click="runMigration">执行处理</t-button></template>
  </ResourceOverlay>
  <ResourceOverlay v-if="confirmation" :visible="!!confirmation" :title="confirmation?.title||'确认操作'" @update:visible="v=>{if(!v)cancelOperation()}" form-actions><p class="confirm-message">{{confirmation?.message}}</p><template #footer><t-button theme="default" @click="cancelOperation">取消</t-button><t-button :theme="['clean','delete-files'].includes(confirmation?.kind)?'danger':'primary'" @click="confirmOperation">{{confirmation?.kind==='delete-files'?'删除':confirmation?.kind==='leave'?'放弃修改':'确认'}}</t-button></template></ResourceOverlay>
  <ResourceOverlay :visible="exported" title="诊断包已导出（演示）" @update:visible="exported=$event" form-actions><p>示意输出：/Users/aqiuye/Downloads/WTMedia-diagnostic.zip</p><p class="helper">此预览不生成真实归档，正式结果展示实际输出位置及缺失项。</p><template #footer><t-button theme="default" @click="exported=false">关闭</t-button></template></ResourceOverlay>
</section>
</template>
<style scoped>
.account-settings-page{min-width:0}.account-review-control{display:flex;align-items:center;gap:12px;font-size:13px;color:var(--wt-text-tertiary)}.account-review-control .t-select__wrap{width:260px}
.personal-content{max-width:1120px;display:grid;gap:16px}.identity-row{display:flex;align-items:center;gap:16px;margin-bottom:16px}.identity{display:flex;flex-direction:column;gap:4px}.identity strong{font-size:16px;font-weight:500}.identity span,.helper,.section-meta,.log-meta,.file-meta,.log-footer{font-size:12px;color:var(--wt-text-tertiary);line-height:20px}.identity-row>.resource-row-actions{margin-left:auto}.helper{margin:4px 0 0}.profile-form{max-width:680px;margin-bottom:24px}.profile-fields{display:grid;grid-template-columns:1fr 1fr;gap:24px}.avatar-control{display:flex;align-items:center;gap:12px}.avatar-list{margin-bottom:16px}.avatar-list :deep(.t-radio-button){height:auto;padding:8px}.avatar-list :deep(.t-radio-button__label){display:flex;align-items:center;gap:8px}.resource-form-actions{display:flex;gap:8px;justify-content:flex-end;flex-wrap:wrap}
.browser-binding-status{display:inline-flex;align-items:center;margin-left:auto;flex-shrink:0;white-space:nowrap}
.device-hero{display:flex;align-items:center;gap:16px;margin:24px 0;padding:20px;border-radius:var(--wt-radius-md);background:var(--td-brand-color-1)}.device-icon{display:grid;place-items:center;flex:0 0 48px;width:48px;height:48px;border-radius:var(--wt-radius-md);background:var(--td-brand-color-2, var(--wt-color-primary-border));color:var(--wt-primary);font-size:26px}.device-hero>div{display:flex;flex-direction:column;gap:6px;min-width:0}.device-hero strong{font-size:16px;line-height:24px;font-weight:600}.device-hero div>span{font-size:13px;line-height:22px;color:var(--wt-text-tertiary)}
.device-details{margin:0}.device-details>div{display:grid;grid-template-columns:112px minmax(0,1fr);column-gap:16px;padding:8px 0;line-height:24px}.device-details dt{color:var(--wt-text-tertiary);font-size:13px}.device-details dd{margin:0;text-align:left;overflow-wrap:anywhere;min-width:0;font-size:14px}.machine-section{margin-top:24px;padding-top:24px;border-top:1px solid var(--wt-border)}.device-alert{margin-top:20px}.profile-error{margin-bottom:16px}.device-actions{display:flex;flex-wrap:wrap;align-items:center;gap:12px;margin-top:16px}.device-actions .device-note{margin:0}.device-note{margin:16px 0 0;color:var(--wt-text-tertiary);font-size:13px;line-height:22px}

.settings-tabs{margin-bottom:20px}.settings-tabs :deep(.t-tabs){background:transparent}.settings-tabs :deep(.t-tabs__content){display:none}.download-body{max-width:1280px}.download-body :deep(.resource-section__header h3){width:100%}.download-body :deep(.resource-section__header .resource-row-actions){margin-left:auto}.download-body :deep(.resource-section+.resource-section){margin-top:24px}.directory-row,.history-row{display:flex;align-items:center;gap:24px;justify-content:space-between}.directory-value{display:flex;align-items:flex-start;gap:10px;min-width:0;line-height:24px}.directory-value>.t-icon{flex-shrink:0;margin-top:4px;color:var(--wt-primary)}.path-value{overflow-wrap:anywhere}.directory-row>.resource-form-actions,.history-row>.resource-row-actions{flex-shrink:0}.history-row{min-height:44px}.history-row+.history-row{border-top:1px solid var(--wt-border)}.migration-entry{display:flex;align-items:center;justify-content:space-between;gap:24px;padding:20px 0;margin:16px 0 24px;border-top:1px solid var(--wt-border);border-bottom:1px solid var(--wt-border)}.migration-entry strong{font-size:16px;font-weight:500}.migration-entry .helper{display:block}.section-meta{margin-left:auto;font-weight:400}.cleanup-note{margin-top:12px}
.logs-card{padding:0;overflow:hidden}.log-toolbar{display:flex;align-items:flex-end;gap:16px;padding:20px 24px;border-bottom:1px solid var(--wt-border);flex-wrap:wrap}.log-toolbar>div:not(.resource-row-actions):not(.resource-form-actions){width:160px}.log-toolbar label{display:block;font-size:13px;font-weight:500;margin-bottom:8px}.log-toolbar>.resource-form-actions{margin-left:auto}.log-toolbar>.resource-row-actions{margin-bottom:2px}.log-split{display:grid;grid-template-columns:280px minmax(0,1fr);min-height:560px}.file-list{padding:16px 12px;border-right:1px solid var(--wt-border);overflow:auto;max-height:700px}.file-group+.file-group{margin-top:20px}.file-group h3{font-size:13px;font-weight:500;color:var(--wt-text-tertiary);margin:0 12px 8px}.log-file-item{display:block;width:100%;text-align:left;border:1px solid transparent;border-radius:4px;background:transparent;padding:10px 12px;margin-bottom:4px;color:var(--wt-text-primary);font:inherit;cursor:pointer}.log-file-item:hover{background:var(--wt-color-bg-hover)}.log-file-item.is-selected{background:var(--wt-info-bg);border-color:var(--wt-color-primary-border)}.log-file-item:focus-visible{outline:2px solid var(--wt-primary);outline-offset:1px}.file-title{display:flex;gap:8px;align-items:flex-start;font-size:13px;line-height:20px;overflow-wrap:anywhere}.file-title>.t-icon{flex-shrink:0;margin-top:2px}.file-meta{display:flex;gap:8px;align-items:center;margin:8px 0 0 24px;flex-wrap:wrap}.log-reader{min-width:0;padding:20px 24px}.reader-heading{display:flex;align-items:center;gap:8px;min-width:0}.reader-heading strong{font-size:16px;font-weight:500;overflow-wrap:anywhere}.log-meta{display:flex;gap:16px;flex-wrap:wrap;margin:8px 0 16px}.log-view{min-height:420px;max-height:560px;overflow:auto;background:var(--wt-color-bg-subtle);border:1px solid var(--wt-border);border-radius:4px;padding:12px 0}.log-line{display:flex;font-size:13px;line-height:24px;padding-right:12px}.line-number{width:40px;flex-shrink:0;text-align:right;padding-right:12px;color:var(--wt-color-text-assist);user-select:none}.log-line code{white-space:pre-wrap;overflow-wrap:anywhere;font-family:ui-monospace,SFMono-Regular,Menlo,monospace}.log-line.warn code{color:var(--wt-color-warning-text)}.log-line.plain code{color:var(--wt-text-tertiary)}.log-footer{display:flex;align-items:center;justify-content:space-between;margin-top:12px}.overlay-help{margin:0 0 16px}.confirm-message{line-height:24px;overflow-wrap:anywhere;margin:0}.migration-target{margin:0 0 16px;line-height:24px;overflow-wrap:anywhere}.migration-result{padding:24px;text-align:center}
@media(max-width:1200px){.log-split{grid-template-columns:240px minmax(0,1fr)}.log-reader{padding:20px 16px}.directory-row{align-items:flex-start;flex-wrap:wrap}.directory-row>.resource-form-actions{justify-content:flex-start}.log-toolbar>.resource-form-actions{margin-left:0}}
@media(max-width:900px){.profile-fields{grid-template-columns:1fr}.history-row{align-items:flex-start;gap:12px;padding:8px 0}.log-split{grid-template-columns:220px minmax(0,1fr)}}
</style>
