<script setup>
import { computed, ref, watch } from 'vue';
import ResourceOverlay from './PrototypeOverlay.vue';
import ResourceFieldHelp from '../../../src/shared/ui/resource/ResourceFieldHelp.vue';
import BatchConfigurationField from './BatchConfigurationField.vue';
import { ChevronDownIcon, ChevronRightIcon } from 'tdesign-icons-vue-next';
import { feedback } from '../../../src/shared/ui/feedback/index.js';
import { declarationOptions } from './model.js';
import { configurationCopyFields, configurationTextFields, prepareBatchConfiguration, applyBatchConfiguration, editConfigurationField, regenerateConfigurationField, configurationIssues, configurationFieldError, updateConfigurationCover } from './batchConfiguration.js';
import { coverTextFieldsForRow, coverSpecs, isTemplateCover } from './covers.js';
const props=defineProps({visible:Boolean,rows:Array,ids:Array,db:Object});
const emit=defineEmits(['update:visible','working','preview']);
const sections=ref([]),commonFields=ref([]),declaration=ref(''),visibility=ref('公开');
const fields=ref([]),style=ref('实战攻略'),overwrite=ref(false);
const coverMode=ref('source'),imageId=ref(null),templateId=ref(null);
const templateIds=ref({});
const templateSpecs=computed(()=>coverSpecs(platform.value));
const templatesFor=spec=>templates.value.filter(template=>Math.abs(template.document.width/template.document.height-spec.width/spec.height)<0.01);
const results=ref(null),busy=ref(false),error=ref(''),capturedIds=ref([]),preparedConfig=ref(null);
const labels={title:'发布标题',shortTitle:'封面短标题',subTitle:'封面副标题',description:'简介 / 正文',topics:'话题',tags:'平台标签'};
const review=ref(false),expanded=ref(null);
const fieldConfig=computed(()=>({sections:sections.value,cover:{mode:coverMode.value,templateId:templateId.value,templateIds:templateIds.value}}));
const selectedRows=computed(()=>props.rows.filter(row=>capturedIds.value.includes(row.candidateId)));
const requiredTextFields=computed(()=>sections.value.includes('cover') ? [...new Set(selectedRows.value.flatMap(row=>configurationTextFields(row,fieldConfig.value,props.db)))] : []);
const availableFields=computed(()=>[...new Set(selectedRows.value.flatMap(row=>configurationCopyFields(row,fieldConfig.value,props.db)))]);
const platform=computed(()=>props.rows.find(row=>capturedIds.value.includes(row.candidateId))?.platform);
const images=computed(()=>props.db.images.filter(image=>image.enabled && image.uses?.includes(coverMode.value==='template'?'底图':'现成封面')));
const templates=computed(()=>props.db.templates.filter(template=>template.enabled && template.document));
const failures=computed(()=>results.value?.filter(item=>!item.applied&&configurationIssues(item).length) || []);
const retryable=computed(()=>failures.value.filter(item=>item.error));
const pending=computed(()=>results.value?.filter(item=>!item.applied&&!configurationIssues(item).length) || []);
const aiUsed=computed(()=>sections.value.includes('copy'));
const currentConfigKey=()=>JSON.stringify({sections:[...sections.value],commonFields:[...commonFields.value],declaration:declaration.value,visibility:visibility.value,fields:[...fields.value],style:style.value,overwrite:overwrite.value,coverMode:coverMode.value,imageId:imageId.value,templateId:templateId.value,templateIds:{...templateIds.value}});
const reviewedKey=ref('');
const configChanged=computed(()=>Boolean(results.value)&&currentConfigKey()!==reviewedKey.value);
watch(requiredTextFields,(next,previous)=>{
  const added=next.filter(key=>!previous.includes(key));
  if(added.length){if(!sections.value.includes('copy'))sections.value.push('copy');fields.value=[...new Set([...fields.value,...added])];}
});
watch(availableFields,next=>{
  const hadFields=fields.value.length>0;
  fields.value=fields.value.filter(key=>next.includes(key));
  if(hadFields && !fields.value.length)sections.value=sections.value.filter(key=>key!=='copy');
});
const asset=row=>props.db.assets.find(asset=>asset.id===row.assetId);
const account=row=>props.db.accounts.find(account=>account.id===row.accountId);
watch(()=>props.visible,visible=>{
  if(!visible)return;
  capturedIds.value=[...props.ids];sections.value=[];commonFields.value=[];fields.value=[];declaration.value='';visibility.value='公开';
  style.value='实战攻略';overwrite.value=false;coverMode.value='source';imageId.value=null;templateId.value=null;templateIds.value={};
  results.value=null;preparedConfig.value=null;error.value='';review.value=false;expanded.value=null;reviewedKey.value='';
});
watch(busy,value=>emit('working',value));
watch(coverMode,()=>imageId.value=null);
function config() {
  const common={};
  if(sections.value.includes('common')) {
    if(!commonFields.value.length)throw Error('请选择本次修改的公共字段');
    if(commonFields.value.includes('declaration')) {if(!declaration.value)throw Error('请选择创作声明');common.declaration=declaration.value;}
    if(commonFields.value.includes('visibility'))common.visibility=visibility.value;
  }
  const selectedFields=fields.value.filter(key=>availableFields.value.includes(key));
  if(sections.value.includes('copy')&&!selectedFields.length)throw Error('请选择本次生成的文案字段');
  if(sections.value.includes('cover')) {
    if(coverMode.value==='image'&&!imageId.value)throw Error('请选择封面图片');
    if(coverMode.value==='template') {
      if(templateSpecs.value.length) {for(const spec of templateSpecs.value)if(!templateIds.value[spec.key])throw Error(`请选择 ${spec.key} 封面模板`);}
      else if(!templateId.value)throw Error('请选择封面模板');
    }
  }
  return {sections:[...sections.value],common,copy:{fields:selectedFields,style:style.value,overwrite:overwrite.value},cover:{mode:coverMode.value,imageId:imageId.value,templateId:templateId.value,templateIds:{...templateIds.value},textPolicy:'keep',style:style.value}};
}
async function preview() {
  error.value='';
  try {preparedConfig.value=config();}catch(e){error.value=e.message;return;}
  const requestKey=currentConfigKey();
  busy.value=true;
  try {
    results.value=await prepareBatchConfiguration(props.rows,capturedIds.value,preparedConfig.value,props.db,{previous:results.value || []});
    reviewedKey.value=requestKey;review.value=true;expanded.value=results.value.find(item=>!item.applied)?.id;
    if(!results.value.length)error.value='所选内容已移出，请关闭后重新选择';
  }finally{busy.value=false;}
}
async function retry() {
  const conflicts=retryable.value.filter(item=>item.errorStage==='conflict');
  if(conflicts.length && !(await feedback.confirmWarning({title:'基于最新内容重新审阅？',message:'保留本次人工修改及已生成的选中字段，其他字段使用最新内容。重新审阅后，请检查再应用。',confirmText:'重新审阅'})))return;
  const ids=retryable.value.map(item=>item.id);busy.value=true;
  try {
    const next=await prepareBatchConfiguration(props.rows,ids,preparedConfig.value,props.db,{previous:results.value,rebase:conflicts.length>0});
    results.value=results.value.map(item=>ids.includes(item.id)?next.find(row=>row.id===item.id)||{...item,error:'该条内容已移出，请重新选择'}:item);
  }finally{busy.value=false;}
}
async function apply() {
  if(configChanged.value){error.value='批量配置已调整，请先更新审阅结果';return;}
  if(failures.value.length && !(await feedback.confirmWarning({title:`应用 ${pending.value.length} 条可用配置？`,message:`另有 ${failures.value.length} 条尚未通过检查，将保留在这里继续处理。`,confirmText:'应用可用配置'})))return;
  if(pending.value.some(item=>item.changedFields.length) && preparedConfig.value.copy.overwrite && preparedConfig.value.sections.includes('copy')) {
    if(!(await feedback.confirmWarning({title:'替换已有文案？',message:'仅替换本次选中的文案字段，应用后需要重新确认内容。',confirmText:'应用配置'})))return;
  }
  const result=applyBatchConfiguration(props.rows,results.value);
  if(result.changed || result.unchanged)feedback.toast.success(`已更新 ${result.changed} 条，${result.unchanged} 条无需修改`);
  if(result.failed){expanded.value=results.value.find(item=>!item.applied)?.id;error.value=`${result.failed} 条未应用，请处理后再应用`;}
  if(!result.failed)emit('update:visible',false);
}
function back() {review.value=false;error.value='';}
const bodyFields=item=>item.editableFields.filter(key=>!['shortTitle','subTitle'].includes(key)).sort((a,b)=>Number(a==='description')-Number(b==='description'));
function fieldSource(item,key){return item.manualFields?.includes(key)?'已人工修改':item.row[key]===JSON.parse(item.before || '{}')[key]?'沿用原内容':'AI 生成';}
function changeField(item,key,value){editConfigurationField(item,key,value);}
async function regenerate(item,key){
  if(item.manualFields?.includes(key) && !(await feedback.confirmWarning({title:`重新生成${labels[key]}？`,message:'将替换这项人工修改，其他内容保持原样。',confirmText:'重新生成'})))return;
  regenerateConfigurationField(item,key,{replaceManual:true});
}
async function updateCover(item){busy.value=true;try{if(await updateConfigurationCover(item))feedback.toast.success('封面已更新');}finally{busy.value=false;}}
function coverImages(item){
  if(item.coverNeedsUpdate)return item.previewCovers || {};
  return Object.keys(item.row.coverVariants || {}).length?item.row.coverVariants:{source:item.row.cover};
}
function resultStatus(item){return item.applied?'已应用':configurationIssues(item).length?'待处理':item.changedFields.length?'待应用':'无需修改';}

</script>
<template>
  <ResourceOverlay :visible="visible" title="批量配置" :width="960" :busy="busy" @update:visible="value=>emit('update:visible',value)">
    <div class="bulk-config">
      <div class="bulk-config-summary"><span>已选 <b>{{capturedIds.length}}</b> 条</span><t-tag v-if="aiUsed" theme="primary" variant="light" size="small">AI 本地演示</t-tag></div>
      <t-alert v-if="error" theme="warning" :message="error" />
      <template v-if="!review">
        <p class="bulk-config-hint">{{results?.some(item=>item.applied)?'仅调整尚未应用的内容，已应用项保持原样。':'选择本次修改项，未勾选的内容保持原样。'}}</p>
        <div class="bulk-config-sections" :inert="busy || undefined">
          <section>
            <t-checkbox :checked="sections.includes('common')" @change="checked=>sections=checked?[...sections,'common']:sections.filter(key=>key!=='common')">公共设置</t-checkbox>
            <div v-if="sections.includes('common')" class="bulk-config-fields">
              <t-checkbox-group v-model="commonFields" class="is-wide"><t-checkbox value="declaration">创作声明</t-checkbox><t-checkbox v-if="platform!=='百家号'" value="visibility">可见范围</t-checkbox></t-checkbox-group>
              <label v-if="commonFields.includes('declaration')"><span>创作声明 <em>*</em></span><t-select v-model="declaration" aria-label="批量创作声明" :options="declarationOptions(platform)" placeholder="按内容事实选择" /></label>
              <label v-if="commonFields.includes('visibility')"><span>可见范围</span><t-select v-model="visibility" aria-label="批量可见范围" :options="['公开','仅自己可见'].map(value=>({value,label:value}))" /></label>
            </div>
          </section>
          <section>
            <t-checkbox :checked="sections.includes('copy')" @change="checked=>sections=checked?[...sections,'copy']:sections.filter(key=>key!=='copy')">AI 文案</t-checkbox>
            <div v-if="sections.includes('copy')" class="bulk-config-fields">
              <div class="bulk-config-control is-wide"><span>生成字段 <em>*</em><ResourceFieldHelp label="生成字段" content="仅合成模板需要文字时显示封面短标题等字段。选择模板后自动勾选；取消勾选可沿用已有文字。" /></span><t-checkbox-group v-model="fields"><t-checkbox v-for="field in availableFields" :key="field" :value="field">{{labels[field]}}</t-checkbox></t-checkbox-group></div>
              <label><span>文案风格</span><t-select v-model="style" aria-label="批量文案风格" :options="['实战攻略','简洁'].map(value=>({value,label:value}))" /></label>
              <div class="is-wide"><t-checkbox v-model="overwrite">替换已有文案</t-checkbox><ResourceFieldHelp label="替换已有文案" content="不勾选只补空白；勾选后可替换所选字段的已有文案。审阅时的人工修改会保留，单项重新生成需确认。" /></div>
            </div>
          </section>
          <section>
            <t-checkbox :checked="sections.includes('cover')" @change="checked=>sections=checked?[...sections,'cover']:sections.filter(key=>key!=='cover')">封面设置</t-checkbox>
            <div v-if="sections.includes('cover')" class="bulk-config-fields">
              <label><span>封面方式</span><t-select v-model="coverMode" aria-label="批量封面方式" :options="[{label:'使用素材自带图片',value:'source'},{label:'使用已有图片',value:'image'},{label:'使用模板合成',value:'template'}]" /></label>
              <label v-if="coverMode!=='source'"><span>{{coverMode==='template'?'封面底图':'封面图片'}}</span><t-select v-model="imageId" aria-label="批量封面图片" :options="[...(coverMode==='template'?[{label:'素材自带图片',value:null}]:[]),...images.map(image=>({label:image.name,value:image.id}))]" placeholder="请选择图片" /></label>
              <template v-if="coverMode==='template'">
                <label v-for="spec in templateSpecs" :key="spec.key"><span>{{spec.label}} 模板 <em>*</em></span><t-select v-model="templateIds[spec.key]" :aria-label="`批量封面模板 ${spec.key}`" :options="templatesFor(spec).map(template=>({label:template.name,value:template.id}))" placeholder="请选择对应比例的模板" /></label>
                <label v-if="!templateSpecs.length"><span>封面模板 <em>*</em></span><t-select v-model="templateId" aria-label="批量封面模板" :options="templates.map(template=>({label:template.name,value:template.id}))" placeholder="请选择模板" /></label>
              </template>

            </div>
          </section>
        </div>
      </template>
      <template v-else>
        <div class="bulk-config-summary"><span>审阅与修改 · {{pending.length}} 条可应用<span v-if="failures.length">，{{failures.length}} 条待处理</span></span></div>
        <div class="bulk-config-results">
          <article v-for="item in results" :key="item.id" class="bulk-config-result">
            <button type="button" class="bulk-config-result-heading" :aria-expanded="expanded===item.id" @click="expanded=expanded===item.id?null:item.id">
              <component :is="expanded===item.id?ChevronDownIcon:ChevronRightIcon" /><span class="bulk-config-result-name"><strong>{{asset(item.row)?.title || '成片不可用'}}</strong><small>{{account(item.row)?.name || '账号不可用'}} · V-{{item.row.assetId || '—'}}</small></span><t-tag size="small" :theme="item.applied?'success':configurationIssues(item).length?'warning':'default'" variant="light">{{resultStatus(item)}}</t-tag>
            </button>
            <div v-if="expanded===item.id" class="bulk-config-result-body">
              <small v-if="item.applied">已应用，后续调整仅处理尚未应用的内容。</small>
              <template v-else>
              <p v-if="item.error" class="bulk-config-error" role="alert">{{item.error}}</p>
              <dl v-if="preparedConfig.sections.includes('common')" class="bulk-config-common"><template v-for="(_,key) in preparedConfig.common" :key="key"><dt>{{key==='declaration'?'创作声明':'可见范围'}}</dt><dd>{{item.row[key]}}</dd></template></dl>
              <div class="bulk-config-review-grid">
                <BatchConfigurationField v-for="field in bodyFields(item)" :key="field" :label="labels[field]" :value="item.row[field]" :multiline="field==='description'" :disabled="busy || item.applied" :error="configurationFieldError(item,field)" :source="fieldSource(item,field)" @change="value=>changeField(item,field,value)" @regenerate="regenerate(item,field)" />
              </div>
              <div v-if="preparedConfig.sections.includes('cover') || item.editableFields.some(key=>['shortTitle','subTitle'].includes(key)) || item.coverNeedsUpdate" class="bulk-config-cover-review">
                <div class="bulk-config-cover-fields">
                  <BatchConfigurationField v-for="field in item.editableFields.filter(key=>['shortTitle','subTitle'].includes(key))" :key="field" :label="labels[field]" :value="item.row[field]" :disabled="busy || item.applied" :error="configurationFieldError(item,field)" :source="fieldSource(item,field)" @change="value=>changeField(item,field,value)" @regenerate="regenerate(item,field)" />
                  <template v-if="isTemplateCover(item.row)">
                    <dl v-for="field in coverTextFieldsForRow(item.row).filter(field=>!item.editableFields.includes(field.key))" :key="field.key"><dt>{{field.label}}</dt><dd>{{item.row[field.key] || '未填写'}}</dd></dl>
                    <div v-if="item.coverNeedsUpdate" class="bulk-config-cover-update"><span>封面待更新</span><t-button size="small" theme="default" :loading="busy" :disabled="item.applied || coverTextFieldsForRow(item.row).some(field=>!String(item.row[field.key] || '').trim())" @click="updateCover(item)">更新封面</t-button></div>
                  </template>
                </div>
                <div v-if="preparedConfig.sections.includes('cover') || item.coverNeedsUpdate || (isTemplateCover(item.row) && item.editableFields.includes('shortTitle'))" class="bulk-config-covers" :class="{'is-stale':item.coverNeedsUpdate}">
                  <figure v-for="(src,key) in coverImages(item)" :key="key"><button class="bulk-cover-preview" type="button" aria-label="放大待应用封面" :disabled="item.coverNeedsUpdate" @click="emit('preview',{kind:'cover',row:item.row})"><img :src="src" :alt="`${key==='source'?'实际':key}封面`" /></button><figcaption>{{key==='source'?'实际封面':key}}<span v-if="item.coverNeedsUpdate"> · 待更新</span></figcaption></figure>
                </div>
              </div>
              <small v-if="!item.changedFields.length && !item.error">与当前内容一致，无需修改。</small>
              </template>
            </div>
          </article>
        </div>
      </template>
    </div>
    <template #footer>
      <t-button theme="default" :disabled="busy" @click="emit('update:visible',false)">取消</t-button>
      <t-button v-if="review" theme="default" :disabled="busy" @click="back">调整批量配置</t-button>
      <t-button v-if="!review && results && !configChanged" theme="default" :disabled="busy" @click="review=true">返回审阅结果</t-button>
      <t-button v-if="review && retryable.length" theme="default" :loading="busy" @click="retry">重试未成功项</t-button>
      <t-button v-if="!review" :disabled="!sections.length" :loading="busy" @click="preview">{{results && configChanged?'更新审阅结果':aiUsed?'生成并审阅':'预览配置'}}</t-button>
      <t-button v-else :disabled="!pending.length || busy || configChanged" @click="apply">应用配置</t-button>
    </template>
  </ResourceOverlay>
</template>
<style scoped>
.bulk-config{display:flex;flex-direction:column;gap:16px;color:var(--wt-color-text-primary)}.bulk-config-summary{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:8px;color:var(--wt-color-text-secondary);font-size:13px}.bulk-config-summary b{font-weight:500;color:var(--wt-color-primary)}.bulk-config-hint,.bulk-config small{color:var(--wt-color-text-secondary);font-size:12px;margin:0}.bulk-config-sections{display:flex;flex-direction:column;gap:16px;width:100%}.bulk-config-sections>section{border-bottom:1px solid var(--wt-color-border);padding-bottom:20px}.bulk-config-sections>section:last-child{border-bottom:0;padding-bottom:0}.bulk-config-sections>section>.t-checkbox{font-weight:500}.bulk-config-fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:20px 24px;margin-top:16px}.bulk-config-fields>.is-wide{grid-column:1 / -1}.bulk-config-fields>.bulk-config-control,.bulk-config-fields>label:not(.t-checkbox){display:flex;flex-direction:column;gap:8px;min-width:0}.bulk-config-fields>.bulk-config-control>span,.bulk-config-fields>label:not(.t-checkbox)>span{display:flex;align-items:center;gap:4px;font-size:14px}.bulk-config-fields .t-select__wrap{max-width:100%}.bulk-config em{color:var(--td-error-color);font-style:normal}.bulk-config-results{border:1px solid var(--wt-color-border);border-radius:6px;overflow:hidden}.bulk-config-result+.bulk-config-result{border-top:1px solid var(--wt-color-border)}.bulk-config-result-heading{display:flex;align-items:center;gap:12px;width:100%;padding:14px 16px;text-align:left;border:0;background:transparent;color:inherit;font:inherit;cursor:pointer}.bulk-config-result-heading:hover{background:var(--wt-color-bg-subtle)}.bulk-config-result-heading:focus-visible{outline:2px solid var(--wt-color-primary);outline-offset:-2px}.bulk-config-result-heading>.t-icon,.bulk-config-result-heading>.t-tag{flex-shrink:0}.bulk-config-result-name{flex:1;min-width:0;display:flex;flex-direction:column;gap:4px}.bulk-config-result-heading strong{font-weight:500;overflow-wrap:anywhere}.bulk-config-result-body{border-top:1px solid var(--wt-color-border);padding:20px;display:flex;flex-direction:column;gap:20px}.bulk-config-common{display:grid;grid-template-columns:auto minmax(0,1fr) auto minmax(0,1fr);gap:8px 16px;margin:0;font-size:13px}.bulk-config-result dt{color:var(--wt-color-text-secondary)}.bulk-config-result dd{margin:0;overflow-wrap:anywhere}.bulk-config-review-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:20px 24px}.bulk-config-review-grid:empty{display:none}.bulk-config-cover-review{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1fr);gap:24px}.bulk-config-cover-fields{display:flex;flex-direction:column;gap:16px;min-width:0}.bulk-config-cover-fields:empty{display:none}.bulk-config-cover-fields dl{margin:0;font-size:13px;display:flex;gap:12px}.bulk-config-covers{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));align-items:start;gap:12px;min-width:0}.bulk-config-cover-fields:empty+.bulk-config-covers{grid-column:1 / -1;max-width:480px}.bulk-config-covers figure{margin:0;min-width:0}.bulk-cover-preview{display:block;width:100%;border:0;padding:0;background:none;cursor:zoom-in}.bulk-cover-preview:disabled{cursor:default}.bulk-config-covers img{display:block;width:100%;aspect-ratio:4/3;object-fit:contain;background:var(--wt-color-bg-subtle);border:1px solid var(--wt-color-border);border-radius:4px}.bulk-config-covers figcaption{margin-top:6px;font-size:12px;color:var(--wt-color-text-secondary)}.bulk-config-cover-update{display:flex;align-items:center;justify-content:space-between;gap:12px;color:var(--td-warning-color);font-size:13px}.bulk-config-error{font-size:13px;color:var(--td-error-color);margin:0}.bulk-config-covers.is-stale img{opacity:.65}@media(max-width:680px){.bulk-config-fields,.bulk-config-review-grid,.bulk-config-cover-review{grid-template-columns:1fr;gap:16px}.bulk-config-common{grid-template-columns:auto minmax(0,1fr)}.bulk-config-result-body{padding:16px}.bulk-config-covers{max-width:100%}}
</style>
