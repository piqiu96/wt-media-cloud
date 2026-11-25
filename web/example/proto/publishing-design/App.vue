<script setup>
import "./operations-shell.css";
import "./operations-tools.css";
import ImageLibraryDetail from "./ImageLibraryDetail.vue";
import "../../../src/shared/styles/resource-list.css";
import ResourceNameLink from '../../../src/shared/ui/resource/ResourceNameLink.vue';
import OperationsNavigation from './OperationsNavigation.vue';
import OperationsAccountMenu from './OperationsAccountMenu.vue';
import PersonalSettingsPage from '../personal-settings/PersonalSettingsPage.vue';
import AccountPreview from './AccountPreview.vue';
import PublishingExecutionSettings from './PublishingExecutionSettings.vue';
import PlanPage from './PlanPage.vue';
import {prototypeSession,currentPrototypeUser} from './prototypeSession.js';
import {formatDateTime} from '../../../src/shared/utils/datetime.js';
import ProductionPage from './ProductionPage.vue';
import {loadProduction} from './production/loader.js';
import {syncProductionResources,productionPublication,publishingPlatform} from './production/bridge.js';
import { reconcileExecutionGroup } from "./batchExecutionSettings.js";
import { addExecutionReviewScenarios, addPlanReviewScenarios } from "./scenarios.js";
import {navigationGroups,productionPages,operationURL,readOperationRoute,updateOperationURL,preservePlanReview} from './routes.js';
import {
  ref,
  shallowRef,
  reactive,
  computed,
  watch,
  onMounted,
  onBeforeUnmount,
  nextTick,
  provide,
} from "vue";
import { feedback } from "../../../src/shared/ui/feedback/index.js";
import {
  ChevronDownIcon,
} from "tdesign-icons-vue-next";
import ResourceDateRange from "../../../src/shared/ui/resource/ResourceDateRange.vue";
import ResourceFilterBar from "../../../src/shared/ui/resource/ResourceFilterBar.vue";
import ResourceAdvancedToggle from "../../../src/shared/ui/resource/ResourceAdvancedToggle.vue";
import ResourceSelectionToolbar from "../../../src/shared/ui/resource/ResourceSelectionToolbar.vue";
import ResourceRowActions from "../../../src/shared/ui/resource/ResourceRowActions.vue";
import ResourceEmptyState from "../../../src/shared/ui/resource/ResourceEmptyState.vue";
import ResourceDescriptions from "../../../src/shared/ui/resource/ResourceDescriptions.vue";
import ResourceNotice from "../../../src/shared/ui/resource/ResourceNotice.vue";
import MediaPreview from "../../../src/shared/ui/media/MediaPreview.vue";
import CoverPreview from "./CoverPreview.vue";
import CoverTemplateManager from "./CoverTemplateManager.vue";
import { imageReferences, templateReferences, deleteImage } from "./library.js";
import BatchManager from "./BatchManager.vue";
import { migrateBatchLifecycle } from "./batchLifecycle.js";
import PlanWizard from "./PlanWizard.vue";
import PlanWorkbench from "./PlanWorkbench.vue";
import { defaultDocument, renderDocument } from "./covers.js";
import {
  pausePlan,
  queueTask,
  nextId,
  takeBatchControl,
  cancelBatchSafely,
  cancellationImpact,
  cancelPlan,
  claimTask,
  setResult,
  decision,
  targetTotal,
} from "./relationships.js";
import { coverSpecs, ensureCoverResults } from "./covers.js";
import { loadPrototype, savePrototype } from "./prototypeStorage.js";
import {
  batchTasks,
  batchSummary,
  planItems,
  planTasks as collectPlanTasks,
  itemTasks,
  itemRecord,
  associatedPlan,
  canLink,
  linkItem,
  moveItem,
  dispatchBatch,
  dispatchPlan,
  advance,
  syncPlan,
  migrateRelationships,
} from "./relationships.js";
import PublishComposer from "./PublishComposer.vue";
import QuickPublish from "./QuickPublish.vue";
import TaskRowActions from "./TaskRowActions.vue";
import { opColumnWidth } from "../../../src/shared/ui/resource/opColumnWidth.js";
import TaskDetail from "./TaskDetail.vue";
import RelatedBatchPreview from "./RelatedBatchPreview.vue";
import ResourceStatGrid from "../../../src/shared/ui/resource/ResourceStatGrid.vue";
import PublishingTaskPage from "./PublishingTaskPage.vue";
import RequiredLabel from "./RequiredLabel.vue";
import TaskActionDialog from "./TaskActionDialog.vue";
import {
  taskPresentation,
  taskActionItems,
  allowedActions,
  actionLabels,
  canEditTask,
  canCancelTask,
  displayStatus,
  taskContent, taskTitle,
  taskChecks,
} from "./taskExperience.js";
import { useMediaQuery } from "../../../src/shared/composables/useMediaQuery.js";
import ResourceOverlay from "./PrototypeOverlay.vue";
import ResourcePageHeader from "../../../src/shared/ui/resource/ResourcePageHeader.vue";
import ResourceFieldHelp from "../../../src/shared/ui/resource/ResourceFieldHelp.vue";
import ResourceForm from "../../../src/shared/ui/resource/ResourceForm.vue";
import ResourceCard from "../../../src/shared/ui/resource/ResourceCard.vue";
import PrototypeEditor from "./PrototypeEditor.vue";
import {
  platforms,
  backgroundPicture,
  states,
  publishingCategories,
  seed,
  validate,
  stats,
  nextBatch,
  canRetry,
  declarationOptions,
  resourceStatus,
} from "./model.js";
const compact = useMediaQuery("(max-width:700px)");
const clone = (x) => JSON.parse(JSON.stringify(x));
const db = reactive(seed()),
  storageError = ref(""),
  loadError = ref(""),
  hydrated = ref(false),
  savingSnapshot = ref(false);
const categories=computed(()=>publishingCategories(db));
let saveTimer;
let saveQueue = Promise.resolve();
async function persistNow() {
  if (!hydrated.value) return false;
  clearTimeout(saveTimer);
  const snapshot = clone(db);
  savingSnapshot.value = true;
  saveQueue = saveQueue
    .catch(() => {})
    .then(() => {
      return savePrototype(snapshot);
    });
  try {
    await saveQueue;
    storageError.value = "";
    return true;
  } catch (e) {
    storageError.value = e.message?.includes("409")
      ? "409：其他窗口已更新，本次改动未写入。请先保留未保存的文案，再重新加载最新记录。"
      : e.message?.includes("首次执行锁定")
        ? `${e.message}。本次改动未写入，请先保留未保存的文案，再重新加载最新记录。`
        : `任务信息未能保存：${e.message || "请重试"}。当前内容仍在本页，请先重试保存。`;
    return false;
  } finally {
    savingSnapshot.value = false;
  }
}
async function commitTemplates(templates) {
  if (!hydrated.value) throw new Error('模板尚未加载，请稍后重试');
  clearTimeout(saveTimer);
  saveQueue = saveQueue.catch(() => {}).then(() => savePrototype({...clone(db),templates:clone(templates)}));
  await saveQueue;
  db.templates = clone(templates);
}
async function reloadLatest() {
  if (
    await feedback.confirmWarning({
      title: "重新加载最新记录？",
      message:
        "将读取已保存的最新版本，本页尚未保存的修改会丢失。请先复制需要保留的文案。",
      confirmText: "重新加载",
    })
  )
    location.reload();
}
async function leaveEditor(event) {
  if (!editorPages.value.length) return;
  event.preventDefault();
  const href = event.currentTarget.href;
  await editorPages.value.at(-1).close();
  await nextTick();
  if (!editorPages.value.length) location.assign(href);
}
function retryLoading() {
  location.reload();
}
onMounted(async () => {

  try {
    const saved = await loadPrototype();
    if (saved) Object.assign(db, saved);
    migrateRelationships(db);
    migrateBatchLifecycle(db);
    if (planReview && new URLSearchParams(location.search).get("samples") !== "0") addPlanReviewScenarios(db);
    if (executionReview && new URLSearchParams(location.search).get("samples") !== "0") addExecutionReviewScenarios(db);
    await nextTick();
    if (!db.images.some((i) => i.pureBackground))
      db.images.push({
        id: id(),
        name: "通用纯净底图",
        category: "通用",
        uses: ["底图"],
        enabled: true,
        src: backgroundPicture(),
        pureBackground: true,
        format: "SVG",
        width: 1200,
        height: 900,
        source: "内置图形",
        version: 1,
        updatedAt: new Date().toISOString(),
      });
    for (const t of db.templates) {
      const d=t.document;
      if (!t.contentModel && !t.draft && d && t.name===`通用动态短标题 ${d.width}×${d.height}` &&
          db.assets.some(a=>a.cover===d.image) && JSON.stringify(d)===JSON.stringify(defaultDocument(d.width,d.height,d.image))) {
        d.image=db.images.find(i=>i.pureBackground)?.src || "";
        t.preview="";
      }
    }
    if (!db.templates.length) {
      for (const [width, height] of [
        [1200, 900],
        [1600, 900],
        [1080, 1440],
      ]) {
        const doc = defaultDocument(
          width,
          height,
          db.images.find((i) => i.pureBackground)?.src || "",
        );
        db.templates.push({
          id: id(),
          name: `通用动态短标题 ${width}×${height}`,
          document: doc,
          ratio: `${width}×${height}`,
          version: 1,
          enabled: true,
          preview: doc.image,
        });
      }
    }
  } catch (e) {
    loadError.value = e.message || "任务记录暂时无法读取";
    return;
  }
  productionContext.value = await loadProduction({
    sync: (production,actor) => syncProductionResources(db,production,actor),
    publication: (id,platform) => productionPublication(db,id,platform),
    navigate,
    taskDetail: id => view(db.tasks.find(t=>t.id===id)),
    publish: publishOutputs,
  });
  productionContext.value.sync();
  hydrated.value = true;
  for(const p of db.plans)try{syncPlan(db,p);}catch(error){if(p.control!=="等待配置"){p.control="已暂停";warning(error.message);}}
  await nextTick();
  hash();
  await persistNow();
});
watch(
  db,
  () => {
    if (hydrated.value) {
      clearTimeout(saveTimer);
      saveTimer = setTimeout(persistNow, 200);
    }
  },
  { deep: true, flush: "post" },
);
const editorPages = ref([]);
const relatedBatch = ref(null);
const taskMetricItems = computed(() => {
  const rows = db.tasks.filter(t =>
    (!batchFilter.value || t.batchId === batchFilter.value) &&
    (!planFilter.value || planTasks(db.plans.find(p => p.id === planFilter.value) || { items: [] }).some(row => row.id === t.id)) &&
    (!appliedFilters.platform || t.platform === appliedFilters.platform) &&
    (!appliedFilters.category || t.category === appliedFilters.category) &&
    (!appliedFilters.from || String(t.createdAt || t.created || '').slice(0, 10) >= appliedFilters.from) &&
    (!appliedFilters.to || (!!(t.createdAt || t.created) && String(t.createdAt || t.created).slice(0, 10) <= appliedFilters.to)) &&
    taskTitle(t).concat(' #' + t.id).includes(applied.value)
  );
  return [{ key: "", label: "全部任务", value: rows.length }, ...["等待人工", "执行中", "结果待确认"].map(status => ({ key: status, label: status, value: rows.filter(t => t.status === status).length, tone: status === "执行中" ? "info" : "warning" }))];
});
function selectTaskMetric(status) { filterStatus.value = status; appliedFilters.status = status; pageNo.value = 1; }
provide('publishing-editor-pages', editorPages);
const productionContext=shallowRef(null);
const page=ref(readOperationRoute(new URL(location.href)).page), pageNo=ref(1), size=ref(10);
const isProduction=computed(()=>productionPages.includes(page.value));
const accountRef=ref(null),accountSceneKey=ref(location.search),accountAcceptedURL=ref(location.pathname+location.search+location.hash);
const previewMode = new URLSearchParams(location.search).get("review");
const executionReview = previewMode === "chg108";
const planReview = previewMode === "chg109";
const preparationPreview = executionReview || planReview || previewMode === "chg107";
if (preparationPreview) {
  const role = new URLSearchParams(location.search).get("role");
  prototypeSession.actor = role === "admin" ? "周予宁" : role === "operator" ? "林予安" : "陈小雨";
}
const isAccountPage=computed(()=>['personal','settings'].includes(page.value));
const localEnabled=computed(()=>currentPrototypeUser().role!=='管理员'&&new URLSearchParams(accountSceneKey.value).get('client')!=='web');
function applyAccountSceneIdentity(){const params=new URLSearchParams(location.search);if(isAccountPage.value&&(params.get('review')==='account'||params.get('role')==='admin'))prototypeSession.actor=params.get('role')==='admin'?'周予宁':'陈小雨'}
applyAccountSceneIdentity();

async function publishOutputs(ids,platform){
  productionContext.value.sync();
  const resources=ids.map(id=>db.assets.find(a=>a.sourceOutputId===id));
  if(resources.some(a=>!a)){toast('部分成片尚未同步，请重试');return;}
  const targetPlatform=publishingPlatform(platform);
  if(!await persistNow())return;
  const returnURL=operationURL('outputs');
  await navigate(ids.length===1?'tasks':'batches');await nextTick();
  if(ids.length===1)await quickRef.value?.open({...resources[0],assetId:resources[0].id,platform:targetPlatform,returnURL});
  else await composerRef.value?.open({assetIds:resources.map(a=>a.id),platform:targetPlatform,returnURL});
}
let closingForNavigation=false;
async function navigate(p, updateURL = true, destinationURL = null, replaceDestination = true) {
  if(isAccountPage.value&&accountRef.value&&!(await accountRef.value.confirmLeave()))return false;
  if(page.value==='plans'){
    if(!(await planRef.value?.confirmLeave()))return false;
    if(!await persistNow())return false;
  }
  const C=window.CP;
  if(isProduction.value&&C?.dialogs.length){
    if(C.dialogs.some(d=>d.minePending?.())){toast('下载演示处理中，请先完成本次过程');return false;}
    if(C.dialogs.some(d=>d.dirty)&&!(await productionContext.value.confirmLeave()))return false;
    productionContext.value.closeAll();
  }
  if (editorPages.value.length) {
    closingForNavigation=true;
    await editorPages.value.at(-1).close();
    await nextTick();
    closingForNavigation=false;
    if (editorPages.value.length) return false;
  }
  if(isProduction.value)productionContext.value?.sync();
  if(destinationURL)history[replaceDestination?'replaceState':'pushState'](null,'',destinationURL);
  page.value = p;
  detailOpen.value = false;
  if (updateURL) updateOperationURL(p);
  accountSceneKey.value=location.search;accountAcceptedURL.value=location.pathname+location.search+location.hash;applyAccountSceneIdentity();
  window.scrollTo(0,0);
  if(p==='plans')await nextTick().then(()=>planRef.value?.readRoute());
  pageNo.value = 1;
  chosen.value = [];
  query.value = "";
  filterFrom.value = "";
  filterTo.value = "";
  applied.value = "";
  filterStatus.value = "";
  filterPlatform.value = "";
  filterCategory.value = "";
  Object.assign(appliedFilters, { platform: "", category: "", status: "" });
  batchFilter.value = null;
  planFilter.value = null;
}
const hash = async () => {
  const route=readOperationRoute(new URL(location.href));
  if(route.page!==page.value||(isAccountPage.value&&location.search!==accountSceneKey.value)){const previous=page.value;const destinationURL=location.pathname+location.search+location.hash;const moved=await navigate(route.page,false,destinationURL);if(moved===false){if(['personal','settings'].includes(previous))history.replaceState(null,'',accountAcceptedURL.value);else if(previous==='plans')history.replaceState(null,'',planRef.value?.currentURL()||operationURL(previous));else updateOperationURL(previous,{},true);return;}}
  if(isAccountPage.value){accountSceneKey.value=location.search;applyAccountSceneIdentity();}
  if(route.page==='plans')await planRef.value?.readRoute();
  if(route.page==='batches'){await nextTick();batchManagerRef.value?.directURL();}
  if(route.page==='tasks'){
    detail.value=db.tasks.find(t=>t.id===route.taskId)||null;
    detailOpen.value=!!detail.value;
  }
};
window.addEventListener('popstate',hash);
onBeforeUnmount(()=>window.removeEventListener('popstate',hash));
const planRef=ref(null),planPageView=ref('list');
async function changePrototypeActor(name){
 if(name===prototypeSession.actor)return;
 if(isAccountPage.value&&accountRef.value&&!(await accountRef.value.confirmLeave()))return;
 if(page.value==='plans' && !(await planRef.value?.confirmLeave()))return;
 if(isProduction.value&&window.CP?.dialogs.length){if(!(await productionContext.value.confirmLeave()))return;productionContext.value.closeAll();}
 prototypeSession.actor=name;
 if(isProduction.value){productionContext.value.actor=name;productionContext.value.sync();await nextTick();productionContext.value.render();}
}
const names={...Object.fromEntries(navigationGroups.flatMap(g=>g.pages).map(p=>[p.id,p.label])),personal:'我的账号'};
const query = ref(""),
  applied = ref(""),
  filterStatus = ref(""),
  filterPlatform = ref(""),
  filterCategory = ref(""),
  filterFrom = ref(""),
  filterTo = ref(""),
  advanced = ref(false),
  chosen = ref([]),
  batchFilter = ref(null),
  planFilter = ref(null);
const libraryFilter = reactive({ use: "", ratio: "", source: "" });
const referenceRows = ref(null);
const imageDetailId = ref(null);
const imageDetail = computed(() => db.images.find(i => i.id === imageDetailId.value));
const appliedFilters = reactive({
  platform: "",
  category: "",
  status: "",
  use: "", ratio: "", source: "",
  from: "",
  to: "",
});
function applyQuery() {
  applied.value = query.value;
  Object.assign(appliedFilters, {
    platform: filterPlatform.value,
    category: filterCategory.value,
    status: page.value === "images" ? "" : filterStatus.value,
    use: libraryFilter.use, ratio: libraryFilter.ratio, source: libraryFilter.source,
    from: page.value === "images" ? "" : filterFrom.value,
    to: page.value === "images" ? "" : filterTo.value,
  });
  pageNo.value = 1;
  chosen.value = [];
}
function resetQuery() {
  query.value = "";
  if (page.value === "tasks") { filterFrom.value = ""; filterTo.value = ""; }
  Object.assign(libraryFilter, { use: "", ratio: "", source: "" });
  filterPlatform.value = "";
  filterCategory.value = "";
  filterStatus.value = "";
  batchFilter.value = null;
  planFilter.value = null;
  applyQuery();
}
const records = computed(() => {
  let rows = db[page.value] || [];
  if (page.value === "tasks") {
    if (batchFilter.value)
      rows = rows.filter((t) => t.batchId === batchFilter.value);
    if (planFilter.value)
      rows = rows.filter((t) =>
        planTasks(
          db.plans.find((p) => p.id === planFilter.value) || { items: [] },
        ).some((r) => r.id === t.id),
      );
  }
  return rows.filter(
    (r) =>
      (page.value === "images" || !appliedFilters.from ||
        String(r.createdAt || r.created || "").slice(0, 10) >=
          appliedFilters.from) &&
      (page.value === "images" || !appliedFilters.to ||
        (!!(r.createdAt || r.created) &&
          String(r.createdAt || r.created).slice(0, 10) <=
            appliedFilters.to)) &&
      (page.value === "tasks" ? taskTitle(r) : r.name || r.title || db.assets.find((a) => a.id === r.assetId)?.title || "")
        .concat(" #" + r.id)
        .includes(applied.value) &&
      (!["images", "templates"].includes(page.value) ||
        ((!appliedFilters.use || r.uses?.includes(appliedFilters.use)) &&
          (!appliedFilters.ratio ||
            Math.abs(
              (r.document?.width || r.width || 1200) /
                (r.document?.height || r.height || 900) -
                Number(appliedFilters.ratio),
            ) < 0.01) &&
          (!appliedFilters.source ||
            (r.source || "").includes(appliedFilters.source)))) &&
      (!appliedFilters.platform || r.platform === appliedFilters.platform) &&
      (!appliedFilters.category ||
        r.category === appliedFilters.category ||
        r.categories?.includes(appliedFilters.category)) &&
      (page.value === "images" || !appliedFilters.status ||
        r.status === appliedFilters.status ||
        r.control === appliedFilters.status ||
        String(r.enabled) === appliedFilters.status),
  );
});
const paged = computed(() =>
  records.value.slice(
    (pageNo.value - 1) * size.value,
    pageNo.value * size.value,
  ),
);
watch([records, size], () => {
  pageNo.value = Math.min(
    pageNo.value,
    Math.max(1, Math.ceil(records.value.length / size.value)),
  );
  chosen.value = chosen.value.filter((id) =>
    records.value.some((r) => r.id === id),
  );
});
const asset = (id) => db.assets.find((a) => a.id === id),
  account = (id) => db.accounts.find((a) => a.id === id),
  id = () => nextId(db);
const toast = (text) => feedback.toast.success(text),
  warning = (text) => feedback.toast.error(text);
const stateTheme = (s) =>
  s === "已发布"
    ? "success"
    : ["执行失败", "结果待确认", "等待人工"].includes(s)
      ? "warning"
      : s === "执行中"
        ? "primary"
        : "default";
const batchManagerRef = ref(null);
const detail = ref(null),
  detailOpen = ref(false),
  batchDetailOpen = ref(false),
  evidence = ref(""),
  bulkResult = ref(null);
let listScroll = null;
const taskDetailPage = computed(() => detailOpen.value && !!detail.value?.assetId);
async function returnToTaskList() {
 const origin=new URL(location.href).searchParams,task=detail.value;
 const b=db.batches.find(b=>String(b.id)===origin.get('fromBatch')&&b.id===task?.batchId);
 detailOpen.value=false;
 if(b){page.value='batches';await nextTick();batchManagerRef.value?.open(b,false,true,true);}
 else{page.value='tasks';updateOperationURL('tasks');try{const saved=JSON.parse(sessionStorage.getItem('publishing-task-list')||'null');if(saved){query.value=saved.query;applied.value=saved.appliedQuery||'';size.value=saved.size||20;filterFrom.value=saved.from||'';filterTo.value=saved.to||'';pageNo.value=saved.page;filterPlatform.value=saved.platform;filterCategory.value=saved.category;filterStatus.value=saved.status;Object.assign(appliedFilters,saved.applied);}}catch{}await nextTick();for(const {el,top,left} of listScroll||[]){if(el.isConnected)el.scrollTo(left,top);}if(!listScroll)try{window.scrollTo(0,JSON.parse(sessionStorage.getItem('publishing-task-list')||'{}').scroll||0);}catch{}}
}
async function navigateContext(href,batchAction){
 const url=new URL(href,location.origin),route=readOperationRoute(url);
 if(url.origin!==location.origin)return;
 preservePlanReview(url);
 const moved=await navigate(route.page,false,url.pathname+url.search,false);
 if(moved===false)return;
 await nextTick();
 if(route.page==='batches'){
  const batch=db.batches.find(row=>row.id===route.batchId);
  if(batch&&batchAction==='configure')return batchManagerRef.value?.action(batch,batchAction);
  batchManagerRef.value?.directURL();
  if(batch&&batchAction)await batchManagerRef.value?.action(batch,batchAction);
 }
}

function view(row) {
  if (row && !row.assetId && !row.items) {
    detailOpen.value = false;
    page.value = "batches";
    batchManagerRef.value?.open(row);
    return;
  }
  if (!detailOpen.value && page.value === 'tasks')try{sessionStorage.setItem('publishing-task-list',JSON.stringify({query:query.value,appliedQuery:applied.value,page:pageNo.value,size:size.value,from:filterFrom.value,to:filterTo.value,scroll:window.scrollY,platform:filterPlatform.value,category:filterCategory.value,status:filterStatus.value,applied:appliedFilters}));}catch{}
  if (!detailOpen.value && page.value === "tasks")
    listScroll = [
      ...new Set(
        [
          document.scrollingElement,
          ...document.querySelectorAll(".publish-main, .t-table__content"),
        ].filter(Boolean),
      ),
    ].map((el) => ({ el, top: el.scrollTop, left: el.scrollLeft }));
  if (row?.items) planView.value = "总览";
  detail.value = row;
  detailOpen.value = true;
  evidence.value = row.evidence || "";
  if (row?.assetId) {
    page.value = "tasks";
    const origin=new URL(location.href),fromBatch=readOperationRoute(origin).page==='batches'&&row.batchId?row.batchId:null;
    const b=db.batches.find(b=>b.id===fromBatch),fromPlan=b&&String(b.planId)===origin.searchParams.get('fromPlan')?b.planId:null;
    updateOperationURL("tasks",{taskId:row.id,fromBatch,fromPlan});
    window.scrollTo(0, 0);
  }
}
watch(detailOpen, async (open) => {
  if (!open && listScroll && page.value === "tasks") {
    await nextTick();
    requestAnimationFrame(() => {
      for (const s of listScroll || [])
        if (s.el.isConnected) {
          s.el.scrollTop = s.top;
          s.el.scrollLeft = s.left;
        }
    });
  }
});
function cancelTask(t) {
  actionDialog.value?.open(t, "cancel");
}
function taskMenu(t, value) {
  if (value === "detail") return view(t);
  if (value === "edit") {
    detailOpen.value = false;
    return quickRef.value?.edit(t);
  }
  if (["repair", "abandon"].includes(value)) return batchManagerRef.value?.operateTask(t, value);
  if (value === "start") return start(t);
  if (value === "copy") return cloneTask(t);
  actionDialog.value?.open(t, value);
}
function related(batchId, planId) {
  navigate("tasks");
  batchFilter.value = batchId;
  planFilter.value = planId;
  detailOpen.value = false;
}
function demoTransitions(t) {
  return t.status === "排队中"
    ? ["执行中"]
    : t.status === "执行中"
      ? ["等待人工", "结果待确认", "平台审核中", "已发布", "执行失败"]
      : t.status === "等待人工"
        ? ["执行中", "结果待确认"]
        : ["结果待确认", "平台审核中"].includes(t.status)
          ? ["已发布", "执行失败"]
          : [];
}
function taskResult(t, status) {
  try {
    if (status === "执行中" && t.status === "等待人工") {
      db.accounts.find((a) => a.id === t.accountId).loginValid = true;
      t.status = "排队中";
    }
    if (status === "执行中") {
      if (!claimTask(db, t))
        throw Error("领取受阻：检查账号串行、批次暂停、定时时间或并发上限");
      return;
    }
    if (
      status === "已取消" &&
      (!["待执行", "排队中"].includes(t.status) || t.claimed)
    )
      throw Error("在途或未知任务不可直接取消");
    if (["已取消", "已放弃"].includes(t.status))
      throw Error("已终结任务不可恢复");
    if (
      ["等待人工", "结果待确认", "平台审核中", "已发布", "执行失败"].includes(
        status,
      ) &&
      !["执行中", "等待人工", "结果待确认", "平台审核中"].includes(t.status)
    )
      throw Error("请先合法领取任务再更新演示结果");
    if (
      ["结果待确认", "平台审核中"].includes(t.status) &&
      ["已发布", "执行失败"].includes(status) &&
      !t.verifiedAt
    )
      throw Error("先保存演示核验依据，再确认结果");
    setResult(t, status, {
      evidence:
        status === "执行失败"
          ? "本地演示明确拒绝，确认原尝试结束且未发布"
          : t.evidence || "",
    });
  } catch (e) {
    warning(e.message);
  }
}
async function takeControl(item) {
  const b = db.batches.find((b) => b.id === item.id),
    p = associatedPlan(db, item);
  if (!p) return true;
  if (
    !(await feedback.confirmWarning({
      title: "人工接管计划批次？",
      message: `将取得计划“${p.name}”当前批次调度权并永久锁定结构，暂停后续自动调度；不能跳过前序批次。`,
      confirmText: "确认接管并锁定",
    }))
  )
    return false;
  return true;
}
async function start(t, silent = false) {
  if (t.batchId && !canRetry(t)) {
    const batch = db.batches.find((b) => b.id === t.batchId);
    if (batch) {
      batchManagerRef.value?.action(batch, "execute");
      return false;
    }
  }
  const errors = taskChecks(t, db);
  if (errors.length) {
    warning(errors.join("；"));
    return false;
  }
  if (t.batchId && !canRetry(t)) {
    if (!(await takeControl({ kind: "batch", id: t.batchId }))) return false;
    try {
      takeBatchControl(
        db,
        db.batches.find((b) => b.id === t.batchId),
        true,
      );
    } catch (e) {
      warning(e.message);
      return false;
    }
  }
  try {
    const ok = queueTask(db, t, { retry: canRetry(t) });
    if (ok) t.manualDispatch = true;
    if (ok && !silent) toast("任务已进入执行队列");
    return ok;
  } catch (e) {
    warning(e.message);
    return false;
  }
}

async function batchAction(b) {
  batchManagerRef.value?.action(
    b,
    b.phase !== "内容预检通过" ? "configure" : "execute",
  );
}
function configureBatch(b) {
  if (b.planFrozenAt) return;
  detailOpen.value = false;
  page.value='batches';
  if (b.planReview || b.phase !== "内容预检通过") composerRef.value?.openBatch(b);
  else view(b);
}
watch(
  () => db.tasks.map((t) => `${t.id}:${t.status}`).join("|"),
  () => {
    if (!hydrated.value) return;
    for (const b of db.batches)
      if (b.executionReview) reconcileExecutionGroup(db, b, Date.now(), { dispatch: false });
      else if (
        b.control === "执行中" &&
        batchTasks(db, b).length &&
        !batchTasks(db, b).some(
          (t) => !["已发布", "已取消", "已放弃"].includes(t.status),
        )
      )
        b.control = "执行已结束";
    for (const p of db.plans)
      try {
        syncPlan(db, p);
      } catch (error) {
        if (p.control !== "等待配置") p.control = "已暂停";
        if(p.control !== "等待配置")warning(error.message);
      }
  },
  { flush: "post" },
);
const listColumns = computed(() => {
  const first = [
    ...(["images"].includes(page.value)
      ? [{ colKey: "row-select", type: "multiple", width: 44 }]
      : []),
    { colKey: "id", title: page.value === "tasks" ? "任务 ID" : "ID", width: page.value === "tasks" ? 88 : 85 },
    {
      colKey: "content",
      title: page.value === "tasks" ? "发布内容" : page.value === "images" ? "图片" : "名称",
      minWidth: page.value === "tasks" ? 220 : 240,
      ellipsis: false,
    },
  ];
  const middle =
    page.value === "tasks"
      ? [
          { colKey: "scope", title: "平台 / 类目", width: 110 },
          { colKey: "account", title: "发布账号", width: 120 },
          { colKey: "status", title: "任务状态", width: 120 },
          { colKey: "createdAt", title: "创建时间", width: 132 },
        ]
      : page.value === "batches"
        ? [
            { colKey: "scope", title: "平台 / 类目", minWidth: 175 },
            { colKey: "resourceCount", title: "成片 / 任务", width: 110 },
            { colKey: "progress", title: "已公开 / 待执行 / 在途", width: 180 },
            { colKey: "problems", title: "失败 / 待处理", width: 130 },
            { colKey: "control", title: "调度状态", width: 125 },
          ]
        : page.value === "plans"
          ? [
              { colKey: "target", title: "目标 / 已发布", width: 150 },
              { colKey: "gap", title: "目标差额", width: 110 },
              { colKey: "control", title: "调度状态", width: 120 },
            ]
          : page.value === "images"
            ? [
                { colKey: "category", title: "类目", width: 130 },
                { colKey: "uses", title: "用途", minWidth: 140 },
                { colKey: "dimensions", title: "尺寸", width: 132 },
                { colKey: "enabled", title: "启用", width: 88 },
              ]
            : [
                { colKey: "ratio", title: "画布规格", width: 140 },
                { colKey: "layers", title: "图层", width: 100 },
                { colKey: "references", title: "引用 / 版本", width: 110 },
                { colKey: "enabled", title: "状态", width: 95 },
              ];
  return [
    ...first,
    ...middle,
    {
      colKey: "operation",
      title: "操作",
      width:
        page.value === "tasks" ? Math.max(100, ...db.tasks.map(t => {
          const a = taskActionItems(t, db);
          return opColumnWidth(a.length > 5 ? [...a.slice(0, 4).map(x => x.label), '更多⌄'] : a.map(x => x.label), {variant: 'text', primary: a.some(x => x.primary)});
        })) : page.value === "templates" ? 285 : 215,
      fixed: compact.value ? undefined : "right",
    },
  ];
});
const actionDialog = ref(null);
const accountPreview = ref(null);
const quickRef = ref(null),
  taskDetailRef = ref(null);
const composerRef = ref(null),
  planWizardRef = ref(null);
const rowEdit = ref(null),
  rowEditOpen = ref(false),
  editingTaskId = ref(null),
  rowError = ref("");
const rowOriginal = ref("");
const platformTopics = computed({
  get: () =>
    rowEdit.value?.platform === "B站"
      ? rowEdit.value?.tags
      : rowEdit.value?.topics,
  set: (v) => {
    if (rowEdit.value)
      rowEdit.value[rowEdit.value.platform === "B站" ? "tags" : "topics"] = v;
  },
});
function newPublish(mode = "快速发布", planId = null) {
  if (mode === "快速发布") quickRef.value?.open();
  else composerRef.value?.open(mode, planId);
}
async function returnAfterConfiguration(){await nextTick();if(closingForNavigation)return;const p=new URL(location.href).searchParams.get('fromPlan');if(p&&db.plans.some(x=>String(x.id)===p))navigateContext(operationURL('plans',{planId:p}));else updateOperationURL('batches');}
async function onCreated({ batch, tasks, start: shouldStart }) {
  const saved = await persistNow();
  const fromPlan=new URL(location.href).searchParams.get('fromPlan');
  if(batch&&fromPlan&&String(batch.planId)===fromPlan)await navigateContext(batch.planReview?operationURL('plans',{planId:fromPlan}):operationURL('batches',{batchId:batch.id,fromPlan}));
  else{await navigate(batch ? 'batches' : 'tasks');view(batch || tasks[0]);}
  if (saved) toast(batch?.preparationOnly ? "批次配置已保存" : `已保存 ${tasks.length} 条任务`);
  if (saved && shouldStart && batch) await batchManagerRef.value?.action(batch, "execute");
}
const coverPreviewOpen = ref(false),
  coverPreviewReadOnly = ref(false),
  coverPreviewRow = ref(null);
const mediaPreviewOpen = ref(false),
  previewImages = ref([]),
  previewRow = ref(null),
  previewAsset = ref(null),
  previewTitle = ref("");
async function openPreview(payload) {
  if (payload.kind === "cover" && payload.row) {
    coverPreviewReadOnly.value = !!payload.readOnly;
    const cover = clone(payload.row);
    if (cover.coverMode === "现成封面") {
      try {
        await ensureCoverResults(cover);
      } catch {
        /* The viewer shows the unavailable image state. */
      }
    }
    coverPreviewRow.value = cover;
    coverPreviewOpen.value = true;
    return;
  }
  previewAsset.value = payload.asset || null;
  if (previewAsset.value) {
    previewRow.value = null;
    previewTitle.value = `来源成片 · ${previewAsset.value.title}`;
    previewImages.value = [
      {
        src: previewAsset.value.cover,
        label: "成片封面",
        note: "来源成片的当前封面",
      },
    ];
    mediaPreviewOpen.value = true;
    return;
  }
  previewRow.value = payload.row || null;
  previewTitle.value = payload.row
    ? "最终发布内容预览"
    : payload.title || "图片预览";
  const r = payload.row;
  if (r) {
    try {
      await ensureCoverResults(r);
    } catch (e) {
      warning(e.message);
    }
  }
  previewImages.value = r
    ? coverSpecs(r.platform).length
      ? coverSpecs(r.platform).map((spec) => ({
          src: r.coverVariants?.[spec.key] || r.cover,
          label: spec.label,
          note: "当前实际生成结果，请检查裁切与文字可读性。",
        }))
      : [
          {
            src: r.cover,
            label: "原始封面",
            note: "百家号横竖封面规格待核验，当前仅预览已有图片。",
          },
        ]
    : [{ src: payload.src, label: payload.title || "图片" }];
  mediaPreviewOpen.value = true;
}
async function closeRow() {
  if (
    JSON.stringify(rowEdit.value) !== rowOriginal.value &&
    !(await feedback.confirmWarning({
      title: "放弃本条修改？",
      message: "尚未保存的修改将丢失，其他任务配置不受影响。",
      confirmText: "放弃修改",
    }))
  )
    return;
  rowEditOpen.value = false;
}
async function editRow(row, task = false) {
  if (task && !canEditTask(row, db)) {
    warning("当前状态不允许修改已提交快照");
    return;
  }
  rowEdit.value = clone(row);
  try {
    await ensureCoverResults(rowEdit.value);
  } catch (e) {
    warning(e.message);
  }
  rowOriginal.value = JSON.stringify(rowEdit.value);
  editingTaskId.value = task ? row.id : null;
  rowError.value = "";
  rowEditOpen.value = true;
}
async function saveRow() {
  rowError.value = "";
  try {
    await ensureCoverResults(rowEdit.value);
  } catch (e) {
    rowError.value = e.message;
    return;
  }
  rowEdit.value.contentApproved = true;
  rowEdit.value.coverApproved = true;
  const errors = validate(rowEdit.value, db, editingTaskId.value);
  if (errors.length) {
    rowError.value = errors.join("；");
    return;
  }
  if (editingTaskId.value) {
    const current = db.tasks.find((t) => t.id === editingTaskId.value);
    if (!current || !canEditTask(current, db)) {
      rowError.value = "任务已进入不可编辑状态，修改未应用，请重新查看执行记录";
      return;
    }
    Object.assign(
      db.tasks.find((t) => t.id === editingTaskId.value),
      Object.fromEntries(
        [
          "title",
          "description",
          "shortTitle",
          "topics",
          "tags",
          "partition",
          "declaration",
          "visibility",
          "schedule",
          "cover",
          "coverMode",
          "coverSource",
          "coverVariants",
          "coverApproved",
          "contentApproved",
          "coverSignature",
          "coverRender",
          "templateId",
          "templateVersion",
          "templateDocument",
          "templateDocuments",
          "templateSnapshots",
          "imageId",
          "imageVersion",
          "fieldVersions",
        ]
          .filter((k) => rowEdit.value[k] !== undefined)
          .map((k) => [k, clone(rowEdit.value[k])]),
      ),
      {
        version: (current.version || 1) + 1,
        snapshot: Object.fromEntries(
          [
            "platform",
            "accountId",
            "assetId",
            "category",
            "title",
            "description",
            "shortTitle",
            "topics",
            "tags",
            "partition",
            "declaration",
            "visibility",
            "schedule",
            "cover",
            "coverMode",
            "coverSource",
            "coverVariants",
            "templateId",
            "templateVersion",
            "templateDocument",
            "templateDocuments",
            "templateSnapshots",
          ]
            .filter((k) => rowEdit.value[k] !== undefined)
            .map((k) => [k, clone(rowEdit.value[k])]),
        ),
      },
    );
    if (
      current.status === "排队中" &&
      !current.claimed &&
      current.attemptHistory?.at(-1)?.status === "排队中"
    ) {
      const pendingAttempt = current.attemptHistory.at(-1);
      pendingAttempt.snapshot = clone(current.snapshot);
      pendingAttempt.version = current.version;
    }
  } else {
    composerRef.value?.updateRow(rowEdit.value);
  }
  await nextTick();
  if (!(await persistNow())) {
    rowError.value = "修改尚未保存，请重试；当前输入已保留";
    return;
  }
  rowEditOpen.value = false;
}
const imagePicker = ref(false),
  imageSearch = ref(""),
  imageCategory = ref(""),
  coverTarget = ref("row"),
  selectedImage = ref(null);
const imageDraft = reactive({ search: "", category: "" });
const imageContextCategory = computed(() =>
  coverTarget.value === "row" || editorPurpose.value === "cover"
    ? rowEdit.value?.category
    : "",
);
const availableImages = computed(() =>
  db.images.filter(
    (i) =>
      i.enabled &&
      i.uses.includes(coverTarget.value === "editor" ? "底图" : "现成封面") &&
      i.name.includes(imageSearch.value) &&
      (!imageCategory.value ||
        (imageCategory.value === "适用"
          ? [imageContextCategory.value, "通用"].includes(i.category)
          : i.category === imageCategory.value)),
  ),
);
function openImages(target) {
  coverTarget.value = target;
  imagePicker.value = true;
  selectedImage.value = null;
  imageSearch.value = "";
  imageCategory.value = imageContextCategory.value ? "适用" : "";
  imageDraft.search = "";
  imageDraft.category = imageCategory.value;
}
function useImage() {
  const im = db.images.find((i) => i.id === selectedImage.value);
  if (!im) return;
  if (coverTarget.value === "editor") {
    imageEditorRef.value?.setBackground(im.src);
    editorDoc.value.imageId = im.id;
  } else {
    rowEdit.value.imageId = im.id;
    rowEdit.value.imageVersion = im.version || 1;
    rowEdit.value.cover = im.src;
    rowEdit.value.coverSource = im.src;
    rowEdit.value.coverVariants = {};
    ensureCoverResults(rowEdit.value).catch((e) => warning(e.message));
  }
  imagePicker.value = false;
}
function randomCover() {
  const rows = db.images.filter(
    (i) => i.enabled && i.uses.includes("现成封面"),
  );
  let pool = rows.filter((i) => i.category === rowEdit.value.category);
  if (!pool.length) pool = rows.filter((i) => i.category === "通用");
  if (!pool.length) {
    warning("没有匹配类目或通用的可用封面");
    return;
  }
  rowEdit.value.cover = pool[Math.floor(Math.random() * pool.length)].src;
  rowEdit.value.coverSource = rowEdit.value.cover;
  rowEdit.value.coverVariants = {};
  ensureCoverResults(rowEdit.value).catch((e) => warning(e.message));
}
async function removeImage(im) {
  if (imageReferences(db, im).length) {
    warning("图片已被引用，只能停用，历史快照保留");
    return;
  }
  if (
    await feedback.confirmWarning({
      title: "删除图片？",
      message: "图片将不再用于新任务，已创建任务的封面快照保留。",
      confirmText: "删除图片",
      danger: true,
    })
  )
    deleteImage(db, im);
}
const imageOriginal = ref("");
let imageReadToken = 0;
const imageReading = ref(false);
const imageForm = reactive({
    open: false,
    id: null,
    name: "",
    category: "通用",
    uses: ["现成封面"],
    enabled: true,
    src: "",
  }),
  imageError = ref("");
const imageUsageBusy = ref(null);
async function toggleImageUsage(imageId, value) {
  const image = db.images.find((i) => i.id === imageId);
  const enabled = !!value;
  if (!hydrated.value || !image || imageUsageBusy.value !== null || enabled === !!image.enabled) return;
  imageUsageBusy.value = imageId;
  try {
    if (!await feedback.confirmWarning({
      title: `${enabled ? "启用" : "停用"}“${image.name}”？`,
      message: enabled ? "启用后可用于新的封面选择和生成。" : "停用后不再用于新的图片选择或随机底图，已确认封面和历史任务快照保留。",
      confirmText: enabled ? "启用" : "停用",
    })) return;
    clearTimeout(saveTimer);
    const updatedAt = new Date().toISOString();
    saveQueue = saveQueue.catch(() => {}).then(() => savePrototype({
      ...clone(db),
      images: db.images.map((i) => i.id === imageId ? { ...clone(i), enabled, updatedAt } : clone(i)),
    }));
    await saveQueue;
    const current = db.images.find((i) => i.id === imageId);
    if (current) Object.assign(current, { enabled, updatedAt });
    feedback.toast.success(enabled ? "图片已启用" : "图片已停用");
  } catch (e) {
    feedback.toast.error("图片启停失败，原状态保持不变，请重试");
  } finally {
    imageUsageBusy.value = null;
  }
}
function imageEdit(i) {
  Object.assign(
    imageForm,
    i
      ? clone(i)
      : {
          id: null,
          name: "",
          category: "通用",
          uses: ["现成封面"],
          enabled: true,
          src: "",
        },
  );
  imageForm.open = true;
  imageOriginal.value = JSON.stringify(imageForm);
  imageError.value = "";
}
async function closeImage() {
  if (
    JSON.stringify(imageForm) !== imageOriginal.value &&
    !(await feedback.confirmWarning({
      title: "放弃图片修改？",
      message: "当前未保存的名称、用途和图片将丢失。",
      confirmText: "放弃修改",
    }))
  )
    return;
  imageReadToken++;
  imageReading.value = false;
  imageForm.open = false;
}
function readImage(files) {
  const f = files[0];
  if (!f) return;
  if (
    !["image/png", "image/jpeg", "image/webp"].includes(f.type) ||
    f.size > 3 * 1024 * 1024
  ) {
    imageError.value = "请选择不超过 3MB 的 PNG、JPEG 或 WebP";
    return;
  }
  const token = ++imageReadToken;
  imageReading.value = true;
  const reader = new FileReader();
  reader.onerror = () => {
    if (token !== imageReadToken) return;
    imageReading.value = false;
    imageError.value = "读取失败，请重新选择图片";
  };
  reader.onload = () => {
    if (token !== imageReadToken) return;
    const im = new Image();
    im.onload = () => {
      if (token !== imageReadToken) return;
      imageReading.value = false;
      imageError.value = "";
      imageForm.src = reader.result;
      imageForm.name ||= f.name;
      imageForm.format = f.type.split("/")[1].toUpperCase();
      imageForm.size = f.size;
      imageForm.source = "人工上传";
      imageForm.width = im.naturalWidth;
      imageForm.height = im.naturalHeight;
    };
    im.onerror = () => {
      if (token !== imageReadToken) return;
      imageReading.value = false;
      imageError.value = "图片损坏，请重新选择，原图片已保留";
    };
    im.src = reader.result;
  };
  reader.readAsDataURL(f);
}
async function saveImage() {
  if (imageReading.value) {
    imageError.value = "正在读取图片尺寸，请稍后保存";
    return;
  }
  if (!imageForm.name.trim() || !imageForm.src || !imageForm.uses.length) {
    imageError.value = "请填写名称、选择图片和至少一种用途";
    return;
  }
  const row = clone(imageForm);
  delete row.open;
  row.id ||= id();
  row.version = (db.images.find((i) => i.id === row.id)?.version || 0) + 1;
  row.updatedAt = new Date().toISOString();
  const at = db.images.findIndex((i) => i.id === row.id);
  if (at < 0) db.images.push(row);
  else db.images[at] = row;
  imageForm.id = row.id;
  if (!(await persistNow())) {
    imageError.value = "图片尚未保存，请重试；当前输入已保留";
    return;
  }
  imageForm.open = false;
}
const imageEditorRef = ref(null);
const editorOpen = ref(false),
  editorDoc = ref(null),
  editorName = ref(""),
  editorPlatforms = ref([]),
  editorCategories = ref([]),
  editorId = ref(null),
  editorPurpose = ref("template"),
  editorError = ref(""),
  editorPreview = ref(""),
  editorCoverSpec = ref(null),
  editorOriginal = ref(""),
  editorSaving = ref(false);
function editorReady(doc) {
  editorOriginal.value = JSON.stringify({
    name: editorName.value,
    tags: [editorPlatforms.value, editorCategories.value],
    document: doc,
  });
}
function openEditor(template = null, purpose = "template", spec = null) {
  editorCoverSpec.value = spec ||
    coverSpecs(rowEdit.value?.platform)[0] || {
      key: "source",
      width: 1200,
      height: 900,
      label: "原始封面",
    };
  editorPurpose.value = purpose;
  editorId.value = template?.id || null;
  editorDoc.value = template?.document
    ? clone(template.document)
    : purpose === "cover"
      ? {
          width: 1200,
          height: 900,
          background: "#172f4c",
          image: rowEdit.value.coverSource || rowEdit.value.cover,
          imageFit: "meet",
          layers: [],
        }
      : defaultDocument(
          1200,
          900,
          (
            db.images.find((i) => i.enabled && i.pureBackground) ||
            db.images.find((i) => i.enabled && i.uses.includes("底图"))
          )?.src || "",
        );
  if (purpose === "cover" && template?.document)
    editorDoc.value.layers
      .filter((l) => l.kind === "text" && l.id === "title")
      .forEach((l) => (l.text = rowEdit.value.shortTitle || ""));
  if (purpose === "cover") {
    const { width, height } = editorCoverSpec.value;
    const doc = editorDoc.value;
    const sx = width / doc.width,
      sy = height / doc.height;
    doc.layers.forEach((l) => {
      l.x *= sx;
      l.w *= sx;
      l.y *= sy;
      l.h *= sy;
      if (l.kind === "text") {
        l.fontSize *= Math.min(sx, sy);
        l.minFont *= Math.min(sx, sy);
      }
    });
    doc.width = width;
    doc.height = height;
  }
  editorName.value = template?.name || "新封面模板";
  editorPlatforms.value = clone(template?.tags?.platforms || []);
  editorCategories.value = clone(template?.tags?.categories || []);
  editorPreview.value = template?.preview || "";
  editorError.value = "";
  editorOpen.value = true;
}
async function cancelEditor() {
  if (editorSaving.value) return;
  if (
    JSON.stringify({
      name: editorName.value,
      tags: [editorPlatforms.value, editorCategories.value],
      document: editorDoc.value,
    }) !== editorOriginal.value &&
    !(await feedback.confirmWarning({
      title: "放弃封面修改？",
      message: "尚未保存的设计将丢失，已应用的封面和其他任务不受影响。",
      confirmText: "放弃修改",
    }))
  )
    return;
  editorOpen.value = false;
}
async function saveEditor() {
  if (!editorName.value.trim() || !editorDoc.value) {
    editorError.value = "请输入模板名称并准备画布";
    return;
  }
  editorSaving.value = true;
  let rendered;
  try {
    rendered = await renderDocument(editorDoc.value);
  } catch (e) {
    editorError.value = e.message;
    return;
  } finally {
    editorSaving.value = false;
  }
  if (!rendered) {
    editorError.value = "封面未能生成，请检查溢出、字体及底图";
    return;
  }
  editorPreview.value = rendered;
  if (editorPurpose.value === "cover") {
    const template = db.templates.find((t) => t.id === editorId.value);
    if (template) {
      rowEdit.value.templateId = template.id;
      rowEdit.value.templateVersion = template.version;
      rowEdit.value.templateDocuments = {};
    }
    rowEdit.value.coverSource = editorDoc.value.image;
    rowEdit.value.imageId = editorDoc.value.imageId || null;
    rowEdit.value.imageVersion =
      db.images.find((i) => i.id === rowEdit.value.imageId)?.version || null;
    rowEdit.value.coverVariants ||= {};
    rowEdit.value.coverVariants[editorCoverSpec.value.key] =
      editorPreview.value;
    if (
      !coverSpecs(rowEdit.value.platform).length ||
      editorCoverSpec.value.key === coverSpecs(rowEdit.value.platform)[0]?.key
    )
      rowEdit.value.cover = editorPreview.value;
    rowEdit.value.templateDocument = clone(editorDoc.value);
    rowEdit.value.templateDocuments ||= {};
    rowEdit.value.templateDocuments[editorCoverSpec.value.key] = clone(
      editorDoc.value,
    );
    rowEdit.value.coverMode = "规则生成封面";
    editorOpen.value = false;
    return;
  }
  const item = {
    id: editorId.value || id(),
    imageId: editorDoc.value.imageId || null,
    updatedAt: new Date().toISOString(),
    name: editorName.value,
    tags: {
      platforms: clone(editorPlatforms.value),
      categories: clone(editorCategories.value),
    },
    version:
      (db.templates.find((t) => t.id === editorId.value)?.version || 0) + 1,
    document: clone(editorDoc.value),
    preview: editorPreview.value,
    ratio: `${editorDoc.value.width}×${editorDoc.value.height}`,
    enabled: db.templates.find((t) => t.id === editorId.value)?.enabled ?? true,
  };
  const index = db.templates.findIndex((t) => t.id === item.id);
  if (index < 0) db.templates.push(item);
  else db.templates[index] = item;
  editorId.value = item.id;
  editorSaving.value = true;
  const saved = await persistNow();
  editorSaving.value = false;
  if (!saved) {
    editorError.value = "模板尚未保存，请重试；当前设计已保留";
    return;
  }
  editorOpen.value = false;
  toast("模板已保存，可重新打开编辑");
}
const planView = ref("总览");
function editPlan(p) {
  if (p?.lockedAt || ["已取消", "执行已结束"].includes(p?.control)) {
    warning("计划结构已锁定");
    return;
  }
  planWizardRef.value?.open(p);
}
const planTasks = (p) => collectPlanTasks(db, p),
  planTarget = (p) => p.targets.reduce((sum, t) => sum + Number(t.count), 0);
function currentDecision(p) {
  return decision(db, p);
}
function returnToPlan(p) {
  view(p);
}
const planStats = computed(() =>
  detail.value?.items
    ? stats(planTasks(detail.value), planTarget(detail.value))
    : null,
);
const planGroups = computed(() => {
  const p = detail.value;
  if (!p?.items) return [];
  const groups = new Map();
  for (const target of p.targets) {
    const key =
      planView.value === "按平台"
        ? target.platform
        : planView.value === "按类目"
          ? target.category
          : target.platform + " · " + target.category;
    if (!groups.has(key))
      groups.set(key, {
        name: key,
        platform: planView.value === "按类目" ? null : target.platform,
        category: planView.value === "按平台" ? null : target.category,
        target: 0,
      });
    groups.get(key).target += Number(target.count);
  }
  return [...groups.values()].map((g) => ({
    ...g,
    ...stats(
      planTasks(p).filter(
        (t) =>
          (!g.platform || t.platform === g.platform) &&
          (!g.category || t.category === g.category),
      ),
      g.target,
    ),
  }));
});
function startPlan(p) {
  try {
    dispatchPlan(db, p);
  } catch (error) {
    warning(error.message);
  }
}
async function advancePlan(p) {
  const decision = currentDecision(p);
  if (decision === "等待核实") {
    warning("请先处理当前项目中的待执行、在途、人工和未知结果");
    return;
  }
  if (
    decision !== "自动继续" &&
    !(await feedback.confirmWarning({
      title:
        decision === "等待跳过确认" ? "跳过当前项目调度？" : "继续下一项目？",
      message:
        decision === "等待跳过确认"
          ? "不会取消任务或释放占用。"
          : "当前批次失败率超过 10%，失败任务保留在原批次。",
      confirmText: "确认并继续",
    }))
  )
    return;
  try {
    advance(db, p, true);
  } catch (error) {
    warning(error.message);
  }
}
function linkEvidence() {
  if (!/^https:\/\//.test(evidence.value)) {
    warning("请输入完整的 HTTPS 作品链接作为演示核验依据");
    return;
  }
  detail.value.evidence = evidence.value;
  detail.value.verifiedAt = new Date().toISOString();
  toast("已保存核验依据；请用下方演示控制明确核验结果");
}
function cloneTask(t) {
  if (!allowedActions(t).includes("copy")) return;
  quickRef.value?.open(t);
  detailOpen.value = false;
}
</script>

<template>
  <div
    v-if="hydrated"
    class="publish-shell wt-resource-page resource-visual-approved"
  >
    <OperationsNavigation :page="page" :actor="prototypeSession.actor" :local-enabled="localEnabled" @navigate="navigate" @actor-change="changePrototypeActor" />
    <OperationsAccountMenu @navigate="navigate" />
    <div id="operations-detail-layer" class="operations-detail-layer"></div>
    <main
      v-show="!editorPages.length"
      class="publish-main"
      :class="{ 'resource-list-page': !isAccountPage && !taskDetailPage && !batchDetailOpen && (page!=='plans'||planPageView==='list'), 'publishing-task-list': page === 'tasks' && !taskDetailPage, 'resource-page-host': taskDetailPage || batchDetailOpen }"
    >
      <t-alert v-if="storageError" theme="warning" :message="storageError"
        ><template #operation
          ><t-button
            variant="text"
            :loading="savingSnapshot"
            @click="
              storageError.includes('409') ||
              storageError.includes('首次执行锁定') ||
              !hydrated
                ? reloadLatest()
                : persistNow()
            "
            >{{
              storageError.includes("409") ||
              storageError.includes("首次执行锁定") ||
              !hydrated
                ? "重新加载最新记录"
                : "重试保存"
            }}</t-button
          ></template
        ></t-alert
      >
      <PersonalSettingsPage v-if="isAccountPage" :key="page+prototypeSession.actor+accountSceneKey" :page="page" ref="accountRef" @navigate="navigateContext" />
      <PlanPage v-if="page === 'plans' && hydrated" :db="db" :persist="persistNow" :plan-review="planReview" :key="prototypeSession.actor" ref="planRef" @navigate="navigateContext" @view-change="value=>planPageView=value" />
      <ProductionPage v-if="isProduction && productionContext" :context="productionContext" :page="page" />
      <div v-if="taskDetailPage" class="breadcrumbs" aria-label="当前位置">内容运营 / 发布任务 / 详情</div>
      <PublishingTaskPage v-if="taskDetailPage" title="发布任务详情" @back="returnToTaskList">
        <TaskDetail
          ref="taskDetailRef"
          :key="detail.id"
          :task="detail"
          :db="db"
          @preview="openPreview"
          @account="accountPreview = $event"
          @batch="relatedBatch = $event"
        />

      </PublishingTaskPage>
      <CoverTemplateManager v-if="page === 'templates' && hydrated" :db="db" :persist="commitTemplates" />
      <template v-if="!isAccountPage && !isProduction && page !== 'plans' && page !== 'templates' && !taskDetailPage && !batchDetailOpen">
        <div class="breadcrumbs">
          <span>{{ page === "images" ? "运营工具" : "内容运营" }} / {{ names[page] }}</span>
        </div>
        <ResourcePageHeader
          :title="names[page]"
          :description="''"
          ><template #actions
            ><t-button v-if="page === 'tasks'" @click="newPublish('快速发布')"
              >快速发布</t-button
            ><t-button v-if="page === 'batches' && (!preparationPreview || currentPrototypeUser().role !== '管理员')" @click="newPublish('批量发布')"
              >{{ preparationPreview ? "新建批次" : "批量发布" }}</t-button
            ><t-button v-if="page === 'plans'" @click="editPlan()"
              >新建发布计划</t-button
            ><t-button v-if="page === 'images'" @click="imageEdit()"
              >上传图片</t-button
            ><t-button v-if="page === 'templates'" @click="openEditor()"
              >新建模板</t-button
            ></template
          ></ResourcePageHeader
        >
        <t-alert
          v-if="db.legacyPublishDraft"
          theme="info"
          message="旧多平台未完成配置已保留为兼容记录；请按单平台重新创建批次，避免混合提交。"
        />


        <ResourceStatGrid v-if="page === 'tasks'" :items="taskMetricItems" interactive :active-key="appliedFilters.status" @select="selectTaskMetric" />
      </template>
      <div v-if="batchDetailOpen" class="breadcrumbs" aria-label="当前位置">内容运营 / 发布批次 / 详情</div>
      <BatchManager
        ref="batchManagerRef"
        :db="db"
        :persist="persistNow"
        :active="page === 'batches'"
        :preparation-only="preparationPreview"
        :execution-review="executionReview || planReview"
        @detail-change="batchDetailOpen = $event"
        @navigate="navigateContext"
        @configure="configureBatch"
        @copy-batch="composerRef?.open({ copyBatch: $event })"
        @recover="composerRef?.open()"
        @task="view"
        @edit-task="taskMenu($event, 'edit')"
        @preview="openPreview"
      />
      <ResourceCard class="resource-list-panel" v-if="!isAccountPage && !isProduction && page !== 'plans' && page !== 'batches' && page !== 'templates'" v-show="!taskDetailPage && !batchDetailOpen">
        <ResourceFilterBar :class="{'operations-tool-filters': page === 'images'}">
          <label class="wt-filter-field wt-filter-field--wide"
            ><span class="wt-filter-field__label">{{ page === 'tasks' ? '综合搜索' : '名称' }}</span
            ><t-input
              v-model="query"
              :placeholder="page === 'tasks' ? '搜索任务名称或 ID' : '输入名称查询'"
              clearable
              @enter="applyQuery"
          /></label>
          <label
            v-if="['tasks', 'batches'].includes(page)"
            class="wt-filter-field"
            ><span class="wt-filter-field__label">发布平台</span
            ><t-select
              :popup-props="{
                hideEmptyPopup: false,
                attach: 'body',
                zIndex: 5500,
              }"
              v-model="filterPlatform"
              :options="[
                { label: '全部', value: '' },
                ...platforms.map((p) => ({ label: p, value: p })),
              ]"
          /></label>
          <label v-if="page === 'tasks'" class="wt-filter-field"
            ><span class="wt-filter-field__label">任务状态</span
            ><t-select
              :popup-props="{
                hideEmptyPopup: false,
                attach: 'body',
                zIndex: 5500,
              }"
              v-model="filterStatus"
              :options="[
                { label: '全部', value: '' },
                ...states.map((s) => ({
                  label: displayStatus({ status: s }),
                  value: s,
                })),
              ]"
          /></label>
          <label v-if="page === 'images'" class="wt-filter-field"><span class="wt-filter-field__label">内容类目</span><t-select v-model="filterCategory" :options="[{label:'全部',value:''},...categories.map(value=>({label:value,value}))]" /></label>
          <label v-if="page === 'images'" class="wt-filter-field"
            ><span class="wt-filter-field__label">用途</span><t-select v-model="libraryFilter.use" :options="[{label:'全部',value:''},{label:'现成封面',value:'现成封面'},{label:'底图',value:'底图'}]" /></label
          >
          <label v-if="['images', 'templates'].includes(page)" class="wt-filter-field"
            ><span class="wt-filter-field__label">比例</span><t-select v-model="libraryFilter.ratio" :options="[{label:'全部',value:''},{label:'4:3',value:4/3},{label:'16:9',value:16/9},{label:'3:4',value:3/4},{label:'1:1',value:'1'}]" /></label
          >
            <label v-if="page === 'tasks'" class="wt-filter-field"
              ><span class="wt-filter-field__label">内容类目</span
              ><t-select
                :popup-props="{
                  hideEmptyPopup: false,
                  attach: 'body',
                  zIndex: 5500,
                }"
                v-model="filterCategory"
                :options="[
                  { label: '全部', value: '' },
                  ...categories.map((c) => ({ label: c, value: c })),
                ]"
            /></label>
          <div class="wt-filter-actions">
            <t-button @click="applyQuery">查询</t-button
            ><t-button theme="default" variant="outline" @click="resetQuery"
              >重置</t-button
            >
          </div>
          <ResourceAdvancedToggle v-model="advanced" :count="page === 'images' ? Number(!!appliedFilters.source) : Number(page !== 'tasks' && !!appliedFilters.category) + Number(!!appliedFilters.from || !!appliedFilters.to)" controls="publishing-advanced" />
          <div
            v-if="advanced"
            id="publishing-advanced"
            class="wt-advanced-filters"
          >
            <label v-if="page === 'tasks'" class="wt-filter-field"
              ><span class="wt-filter-field__label">创建时间</span
              ><ResourceDateRange
                :value="[filterFrom, filterTo]"
                @change="
                  (v) => {
                    filterFrom = v?.[0] || '';
                    filterTo = v?.[1] || '';
                  }
                "
            /></label>
            <label v-if="page === 'images'" class="wt-filter-field"><span class="wt-filter-field__label">来源</span><t-select v-model="libraryFilter.source" :options="[{label:'全部',value:''},{label:'初始资源',value:'种子资源'},{label:'人工上传',value:'人工上传'},{label:'内置图形',value:'内置图形'}]" /></label>
            <label v-else-if="page !== 'tasks'" class="wt-filter-field"
              ><span class="wt-filter-field__label">内容类目</span
              ><t-select
                :popup-props="{
                  hideEmptyPopup: false,
                  attach: 'body',
                  zIndex: 5500,
                }"
                v-model="filterCategory"
                :options="[
                  { label: '全部', value: '' },
                  ...categories.map((c) => ({ label: c, value: c })),
                ]"
            /></label>
          </div>
        </ResourceFilterBar>
        <ResourceNotice v-if="batchFilter || planFilter"
          >当前范围：{{
            batchFilter ? "批次 " + batchFilter : "计划 " + planFilter
          }}
          <t-button
            variant="text"
            @click="
              batchFilter = null;
              planFilter = null;
            "
            >清除范围</t-button
          ></ResourceNotice
        >
        <ResourceSelectionToolbar
          v-if="page !== 'tasks'"
          :count="page === 'tasks' ? chosen.length : 0"
          unit="条任务"
          @clear="chosen = []"
          ><t-button variant="text" @click="bulk('启动')">启动待执行项</t-button
          ><t-button variant="text" @click="bulk('重试')"
            >重试确认失败项</t-button
          ><t-button variant="text" @click="bulk('取消')"
            >取消未执行项</t-button
          ></ResourceSelectionToolbar
        >
        <ResourceSelectionToolbar
          v-if="page === 'images'"
          :count="chosen.length"
          unit="张图片"
          @clear="chosen = []"
          ><t-button
            variant="text"
            @click="
              db.images
                .filter((i) => chosen.includes(i.id))
                .forEach((i) => (i.uses = ['现成封面', '底图']))
            "
            >批量标记双用途</t-button
          ><t-button
            variant="text"
            @click="
              db.images
                .filter((i) => chosen.includes(i.id))
                .forEach((i) => (i.enabled = true))
            "
            >批量启用</t-button
          ><t-button
            variant="text"
            @click="
              db.images
                .filter((i) => chosen.includes(i.id))
                .forEach((i) => (i.enabled = false))
            "
            >批量停用</t-button
          ></ResourceSelectionToolbar
        >
        <t-table
          class="wt-resource-table resource-data-table"
          :class="{ 'task-list-table': page === 'tasks' }"
          row-key="id"
          :data="paged"
          :columns="listColumns"
          :selected-row-keys="chosen"
          @select-change="(keys) => (chosen = keys)"
          :pagination="{
            current: pageNo,
            pageSize: size,
            total: records.length,
            showJumper: false,
            pageSizeOptions: [10, 20, 50],
          }"
          @page-change="
            (p) => {
              pageNo = p.current;
              size = p.pageSize;
            }
          "
        >
          <template #createdAt="{ row }">{{
            row.createdAt || row.created
              ? new Date(row.createdAt || row.created).toLocaleString("zh-CN", {
                  year: "numeric", month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit", hour12: false,
                })
              : "历史记录未保留"
          }}</template>

          <template #content="{ row }">
            <div class="title-cell">
              <button v-if="page === 'tasks' ? taskContent(row).cover : row.cover || row.src || row.preview" class="media-thumb" :aria-label="page === 'tasks' ? '查看完整发布内容 ' + row.id : '查看图片'" @click="page === 'tasks' ? view(row) : openPreview({src:row.src || row.preview,title:row.name})">
                <img :src="page === 'tasks' ? taskContent(row).cover : row.cover || row.src || row.preview" alt="" />
              </button>
              <div><ResourceNameLink v-if="page === 'tasks'" class="task-title-link" :title="taskTitle(row)" @click="view(row)">{{ taskTitle(row) }}</ResourceNameLink><strong v-else>{{ row.name || row.title }}</strong>
                <small v-if="page === 'tasks'">成片 {{ row.assetId }} · {{ asset(row.assetId)?.duration }}</small>
              </div>
            </div>
          </template>
          <template #scope="{ row }"
            >{{ row.platform
            }}<small class="cell-secondary">{{
              row.category || row.categories?.join("、")
            }}</small></template
          >
          <template #account="{ row }">
            <ResourceNameLink v-if="account(row.accountId)" :aria-label="`查看发布账号 ${account(row.accountId).name}`" @click="accountPreview = account(row.accountId)">{{ account(row.accountId).name }}</ResourceNameLink><span v-else>账号记录缺失</span>
            <small class="cell-secondary">{{ account(row.accountId)?.window ? '窗口 ' + account(row.accountId).window : '未绑定窗口' }}</small>
          </template>
          <template #association="{ row }"
            ><t-button
              v-if="row.batchId"
              variant="text"
              @click="view(db.batches.find((b) => b.id === row.batchId))"
              >{{
                db.batches.find((b) => b.id === row.batchId)?.name
              }}</t-button
            ><span v-else>独立任务</span
            ><small class="cell-secondary">{{
              associatedPlan(db, {
                kind: row.batchId ? "batch" : "task",
                id: row.batchId || row.id,
              })?.name || "未关联计划"
            }}</small></template
          >
          <template #status="{ row }"
            ><t-tag :theme="stateTheme(row.status)" variant="light">{{
              displayStatus(row)
            }}</t-tag
            ><small v-if="page !== 'tasks'" class="cell-secondary">{{ row.result }}</small></template
          >
          <template #resourceCount="{ row }"
            >{{ batchSummary(db, row).videos }} /
            {{ batchSummary(db, row).total }}</template
          >
          <template #progress="{ row }"
            >{{ batchSummary(db, row).published }} /
            {{ batchSummary(db, row).waiting }} /
            {{ batchSummary(db, row).running }}</template
          >
          <template #problems="{ row }"
            >{{ batchSummary(db, row).failed }} /
            {{ batchSummary(db, row).unresolved }}</template
          >
          <template #target="{ row }"
            >{{ planTarget(row) }} /
            {{ stats(planTasks(row)).published }}</template
          >
          <template #gap="{ row }">{{
            stats(planTasks(row), planTarget(row)).gap
          }}</template>
          <template #dimensions="{ row }">{{row.width && row.height ? `${row.width} × ${row.height}` : '—'}}</template>
          <template #references="{ row }"
            ><t-button
              variant="text"
              @click="
                referenceRows =
                  page === 'images'
                    ? imageReferences(db, row)
                    : templateReferences(db, row).map((t) => ({
                        kind: '正式任务',
                        id: t.id,
                        name: t.title,
                      }))
              "
              >{{
                page === "images"
                  ? imageReferences(db, row).length
                  : templateReferences(db, row).length
              }}
              次 / v{{ row.version || 1 }}</t-button
            ></template
          >
          <template #uses="{ row }">{{ row.uses.join("、") }}</template>
          <template #enabled="{ row }">
            <t-tooltip :content="row.enabled ? '已启用' : '已停用'">
              <t-switch :value="!!row.enabled" :disabled="imageUsageBusy !== null" :loading="imageUsageBusy === row.id"
                role="switch" :aria-label="`启用图片${row.name}`" :aria-checked="!!row.enabled" :aria-disabled="imageUsageBusy !== null"
                :tabindex="imageUsageBusy !== null ? -1 : 0"
                @keydown.enter.prevent="toggleImageUsage(row.id, !row.enabled)"
                @keydown.space.prevent="toggleImageUsage(row.id, !row.enabled)"
                @change="v => toggleImageUsage(row.id, v)" />
            </t-tooltip>
          </template>
          <template #layers="{ row }"
            >{{ row.document.layers.length }} 个图层<small
              class="cell-secondary"
              >动态槽
              {{
                row.document.layers.filter((l) => l.slot || l.id === "title")
                  .length
              }}
              ·
              {{
                [
                  ...(row.tags?.platforms || []),
                  ...(row.tags?.categories || []),
                ].join(" / ") || "跨平台通用"
              }}</small
            ></template
          >
          <template #operation="{ row }">
            <TaskRowActions v-if="page === 'tasks'" :task="row" :db="db" @action="taskMenu(row, $event)" />
            <ResourceRowActions v-else>
              <template v-if="['batches', 'plans'].includes(page)"
                ><t-button variant="text" @click="view(row)">详情</t-button
                ><t-button
                  v-if="
                    page === 'batches' &&
                    !['已取消', '执行已结束'].includes(row.control)
                  "
                  variant="text"
                  @click="batchAction(row)"
                  >{{
                    row.phase !== "内容预检通过"
                      ? "配置资源 / 内容"
                      : row.control === "执行中"
                        ? "暂停"
                        : "启动 / 恢复"
                  }}</t-button
                ><t-button
                  v-else-if="
                    page === 'plans' &&
                    !row.lockedAt &&
                    !['已取消', '执行已结束'].includes(row.control)
                  "
                  variant="text"
                  @click="editPlan(row)"
                  >编辑目标</t-button
                ></template
              ><template v-else-if="page === 'images'"
                ><t-button variant="text" @click="imageDetailId=row.id">详情</t-button
                ><t-button variant="text" @click="imageEdit(row)">编辑</t-button
                ><t-tooltip :content="imageReferences(db,row).length ? '图片已被引用，请在详情中查看；可通过列表开关停用' : '删除图片'"><t-button
                  variant="text" theme="danger" :disabled="!!imageReferences(db, row).length" @click="removeImage(row)"
                  >删除</t-button></t-tooltip></template
              ><template v-else
                ><t-button variant="text" @click="openEditor(row)"
                  >编辑</t-button
                ><t-button
                  variant="text"
                  @click="
                    db.templates.push({
                      ...clone(row),
                      id: id(),
                      name: row.name + ' 副本',
                    })
                  "
                  >复制</t-button
                ><t-button variant="text" @click="row.enabled = !row.enabled">{{
                  row.enabled ? "停用" : "启用"
                }}</t-button></template
              >
            </ResourceRowActions></template
          >
          <template #empty
            ><ResourceEmptyState
              :title="
                page === 'templates' ? '还没有封面模板' : '没有符合条件的记录'
              "
              :description="
                page === 'templates'
                  ? '设计一次版式，在发布时替换标题和底图即可复用。'
                  : '调整筛选条件，或通过快速发布创建任务。'
              "
              ><template v-if="page === 'tasks'"
                ><t-button variant="outline" @click="resetQuery"
                  >清空筛选</t-button
                ><t-button @click="quickRef.open()"
                  >快速发布</t-button
                ></template
              ><t-button v-if="page === 'templates'" @click="openEditor()"
                >创建第一个模板</t-button
              ></ResourceEmptyState
            ></template
          >
        </t-table>
      </ResourceCard>
    </main>
    <main id="publishing-editor-root" v-show="editorPages.length" class="publish-main resource-page-host" aria-label="发布配置工作区">
      <div class="breadcrumbs" aria-label="当前位置">{{["images","templates"].includes(page)?"运营工具":"内容运营"}} / {{ names[page] }} / {{ editorPages.at(-1)?.title || "发布配置" }}</div>
      <t-alert v-if="storageError" theme="warning" :message="storageError"><template #operation><t-button variant="text" @click="reloadLatest">重新加载最新记录</t-button></template></t-alert>
    </main>
    <PlanWizard
      ref="planWizardRef"
      :db="db"
      :persist="persistNow"
      @saved="view"
    />
    <AccountPreview :account="accountPreview" @close="accountPreview = null" />
    <RelatedBatchPreview :batch="relatedBatch" :db="db" @close="relatedBatch = null" @navigate="batch => { relatedBatch = null; view(batch); }" />
    <TaskActionDialog ref="actionDialog" :db="db" :persist="persistNow" />
    <QuickPublish
      ref="quickRef"
      :db="db"
      :persist="persistNow"
      @preview="openPreview"
      @return-source="navigate('outputs')"
      @created="onCreated"
    />
    <PublishComposer
      ref="composerRef"
      :db="db"
      :preparation-only="preparationPreview"
      :execution-review="executionReview || planReview"
      :persist="persistNow"
      @edit="editRow"
      @preview="openPreview"
      @return-source="navigate('outputs')"
      @created="onCreated"
      @closed="returnAfterConfiguration"
    />
    <CoverPreview
      v-model:visible="coverPreviewOpen"
      :row="coverPreviewRow"
      :read-only="coverPreviewReadOnly"
    />
    <MediaPreview
      v-model:visible="mediaPreviewOpen"
      :title="previewTitle"
      :images="previewImages"
    >
      <template v-if="previewRow || previewAsset" #details>
        <div v-if="previewAsset" class="preview-publishing-details">
          <h3>{{ previewAsset.title }}</h3>
          <dl>
            <dt>成片 ID</dt>
            <dd>{{ previewAsset.id }}</dd>
            <dt>内容类目</dt>
            <dd>{{ previewAsset.category }}</dd>
            <dt>时长</dt>
            <dd>{{ previewAsset.duration || "尚无记录" }}</dd>
            <dt>成片质量</dt>
            <dd>{{ previewAsset.quality || "尚无记录" }}</dd>
          </dl>
          <video
            v-if="previewAsset.videoUrl"
            :src="previewAsset.videoUrl"
            controls
            preload="metadata"
            style="width: 100%"
          />
          <p v-else>当前成片暂无可播放的视频文件。</p>
        </div>
        <div v-else class="preview-publishing-details">
          <t-tag variant="light">{{ previewRow.platform }}</t-tag>
          <h3>
            {{
              previewRow.platform === "百家号"
                ? previewRow.description
                : previewRow.title
            }}
          </h3>
          <p class="preview-description">
            {{
              previewRow.platform === "百家号"
                ? ""
                : previewRow.description || "未填写正文 / 简介"
            }}
          </p>
          <dl>
            <dt>发布账号</dt>
            <dd>
              {{ account(previewRow.accountId)?.name }} · 窗口
              {{ account(previewRow.accountId)?.window }}
            </dd>
            <dt>成片</dt>
            <dd>
              {{ previewRow.assetId }} ·
              {{ asset(previewRow.assetId)?.duration }} ·
              {{ previewRow.category }}
            </dd>
            <dt>创作声明</dt>
            <dd>{{ previewRow.declaration || "待确认" }}</dd>
            <template v-if="previewRow.platform === 'B站'"
              ><dt>分区 / 标签</dt>
              <dd>
                {{ previewRow.partition }} / {{ previewRow.tags }}
              </dd></template
            ><template v-if="previewRow.platform !== '百家号'"
              ><dt>可见范围</dt>
              <dd>{{ previewRow.visibility }}</dd></template
            ><template v-if="previewRow.platform !== 'B站' && previewRow.tags"
              ><dt>话题</dt>
              <dd>{{ previewRow.tags }}</dd></template
            >
            <dt>提交时间</dt>
            <dd>{{ previewRow.schedule || "启动后尽快提交" }}</dd>
          </dl>
          <ResourceNotice v-if="previewRow.platform === '百家号'"
            >{{ previewRow.mount ? "挂载待核验" : "不挂载" }} ·
            {{ previewRow.watermark ? "水印设置待核验" : "不添加水印" }} ·
            地点和活动未选。横竖封面的具体规格仍待核验。</ResourceNotice
          >
          <t-alert
            v-if="validate(previewRow, db, previewRow.id).length"
            theme="warning"
            :message="validate(previewRow, db, previewRow.id).join('；')"
          />
          <t-button variant="outline" @click="mediaPreviewOpen = false"
            >返回</t-button
          >
        </div>
      </template>
    </MediaPreview>
    <ResourceOverlay
      :visible="rowEditOpen"
      @update:visible="closeRow"
      title="编辑发布内容"
      kind="drawer"
      :width="800"
    >
      <template v-if="rowEdit"
        ><t-alert v-if="rowError" theme="warning" :message="rowError" />
        <p>
          {{ rowEdit.platform }} · {{ rowEdit.category }} · 成片
          {{ rowEdit.assetId }}
        </p>
        <div class="form-grid">
          <label
            >发布账号<t-select
              :popup-props="{
                hideEmptyPopup: false,
                attach: 'body',
                zIndex: 5500,
              }"
              v-model="rowEdit.accountId"
              :disabled="!!editingTaskId"
              :options="
                db.accounts
                  .filter(
                    (a) =>
                      a.enabled &&
                      a.platform === rowEdit.platform &&
                      a.categories.includes(rowEdit.category),
                  )
                  .map((a) => ({ label: a.name, value: a.id }))
              " /></label
          ><label v-if="rowEdit.platform !== '百家号'"
            ><RequiredLabel>标题</RequiredLabel
            ><t-input
              v-model="rowEdit.title"
              :maxlength="rowEdit.platform === '小红书' ? 20 : 80"
              show-limit-number /></label
          ><label class="full"
            ><RequiredLabel :required="rowEdit.platform === '百家号'">{{
              rowEdit.platform === "百家号" ? "作品描述" : "正文 / 简介"
            }}</RequiredLabel
            ><t-textarea
              v-model="rowEdit.description"
              :maxlength="
                rowEdit.platform === '百家号'
                  ? 50
                  : rowEdit.platform === '小红书'
                    ? 1000
                    : 2000
              "
              :autosize="{ minRows: 3, maxRows: 6 }"
          /></label>
        </div>

        <label class="block-field"
          >独立封面短标题<t-input
            v-model="rowEdit.shortTitle"
            aria-label="独立封面短标题"
        /></label>
        <h3>最终封面</h3>
        <div class="cover-previews">
          <figure
            v-for="spec in coverSpecs(rowEdit.platform).length
              ? coverSpecs(rowEdit.platform)
              : [{ key: 'source', label: '原始封面 · 规格待核验' }]"
            :key="spec.key"
          >
            <button
              class="media-thumb"
              @click="
                openPreview({
                  src: rowEdit.coverVariants?.[spec.key] || rowEdit.cover,
                  title: spec.label,
                })
              "
            >
              <img
                :src="rowEdit.coverVariants?.[spec.key] || rowEdit.cover"
                :alt="spec.label"
              />
            </button>
            <figcaption>{{ spec.label }}</figcaption>
            <t-button
              v-if="spec.key !== 'source'"
              variant="text"
              @click="
                openEditor(
                  {
                    document: rowEdit.templateDocuments?.[spec.key] ||
                      rowEdit.templateDocument || {
                        width: spec.width,
                        height: spec.height,
                        background: '#172f4c',
                        image: rowEdit.coverSource || rowEdit.cover,
                        imageFit: 'meet',
                        layers: [],
                      },
                    name: '本条封面',
                  },
                  'cover',
                  spec,
                )
              "
              >调整此封面</t-button
            >
          </figure>
        </div>
        <div class="inline-bar">
          <t-button variant="outline" @click="openImages('row')"
            >从图片库选择</t-button
          ><t-button variant="outline" @click="randomCover"
            >按类目随机选择</t-button
          ><t-button
            variant="outline"
            @click="
              openEditor(
                rowEdit.templateDocument
                  ? { document: rowEdit.templateDocument, name: '本条封面' }
                  : null,
                'cover',
              )
            "
            >调整当前封面</t-button
          >
        </div>
        <label class="block-field"
          >应用封面模板<t-select
            :popup-props="{
              hideEmptyPopup: false,
              attach: 'body',
              zIndex: 5500,
            }"
            :value="rowEdit.templateId"
            clearable
            placeholder="选择已保存模板"
            :options="
              db.templates
                .filter((t) => t.enabled)
                .map((t) => ({ label: t.name, value: t.id }))
            "
            @change="
              (v) => {
                const t = db.templates.find((t) => t.id === v);
                if (t) openEditor(t, 'cover');
                else {
                  rowEdit.templateId = null;
                  rowEdit.templateVersion = null;
                  rowEdit.templateDocument = null;
                  rowEdit.templateDocuments = {};
                  rowEdit.coverMode = '直接应用';
                }
              }
            "
        /></label>
        <div class="form-grid">
          <label
            ><RequiredLabel>创作声明</RequiredLabel><ResourceFieldHelp v-if="rowEdit.platform === '百家号'" label="创作声明" content="当前仅确认无需声明，其他内容类型需要先核验平台选项。"
            /><t-select
              :popup-props="{
                hideEmptyPopup: false,
                attach: 'body',
                zIndex: 5500,
              }"
              v-model="rowEdit.declaration"
              placeholder="按实际内容选择"
              :options="declarationOptions(rowEdit.platform)" /></label
          ><label v-if="rowEdit.platform === 'B站'"
            ><RequiredLabel>分区</RequiredLabel
            ><t-select
              :popup-props="{
                hideEmptyPopup: false,
                attach: 'body',
                zIndex: 5500,
              }"
              v-model="rowEdit.partition"
              :options="
                ['游戏', '知识', '生活'].map((s) => ({ label: s, value: s }))
              " /></label
          ><label
            ><RequiredLabel :required="rowEdit.platform === 'B站'">{{
              rowEdit.platform === "B站" ? "标签" : "话题"
            }}</RequiredLabel
            ><t-input
              v-model="platformTopics"
              placeholder="仅填相关标签，多个用逗号分隔"
          /></label>
        </div>
        <t-alert
          v-if="rowEdit.declaration === '内容为转载'"
          theme="warning"
          message="转载的来源及条件字段尚未核验，请补充来源和授权信息后再提交。"
        />
        <PublishingExecutionSettings :row="rowEdit" @change="(field, value) => rowEdit[field] = value" /></template
      >
      <template #footer
        ><t-button variant="outline" @click="closeRow">取消修改</t-button
        ><t-button @click="saveRow">保存本条内容</t-button></template
      >
    </ResourceOverlay>
    <ResourceOverlay
      v-model:visible="imagePicker"
      :title="coverTarget === 'editor' ? '选择封面底图' : '选择现成封面'"
      :width="960"
    >
      <div class="wt-resource-page resource-visual-approved">
        <ResourceFilterBar
          ><label class="wt-filter-field wt-filter-field--wide"
            ><span class="wt-filter-field__label">名称</span
            ><t-input
              v-model="imageDraft.search"
              clearable
              placeholder="搜索图片名称"
              @enter="
                imageSearch = imageDraft.search;
                imageCategory = imageDraft.category;
                selectedImage = null;
              " /></label
          ><label class="wt-filter-field"
            ><span class="wt-filter-field__label">类目</span
            ><t-select
              :popup-props="{
                hideEmptyPopup: false,
                attach: 'body',
                zIndex: 5500,
              }"
              v-model="imageDraft.category"
              :options="[
                ...(imageContextCategory
                  ? [{ label: '当前类目及通用', value: '适用' }]
                  : []),
                { label: '全部', value: '' },
                ...categories.map((c) => ({ label: c, value: c })),
              ]"
          /></label>
          <div class="wt-filter-actions">
            <t-button
              @click="
                imageSearch = imageDraft.search;
                imageCategory = imageDraft.category;
                selectedImage = null;
              "
              >查询</t-button
            ><t-button
              theme="default"
              variant="outline"
              @click="
                imageDraft.search = '';
                imageDraft.category = '';
                imageSearch = '';
                imageCategory = '';
                selectedImage = null;
              "
              >重置</t-button
            >
          </div></ResourceFilterBar
        >
        <ResourceNotice
          >仅显示已启用且可用作{{
            coverTarget === "editor" ? "底图" : "现成封面"
          }}的图片。点击缩略图可查看完整效果。</ResourceNotice
        >
        <div class="image-picker-grid">
          <button
            v-for="im in availableImages"
            :key="im.id"
            class="image-choice"
            :class="{ selected: selectedImage === im.id }"
            @click="selectedImage = im.id"
          >
            <img :src="im.src" :alt="im.name" /><strong>{{ im.name }}</strong
            ><small
              >{{ im.category }} · {{ im.uses.join(" / ") }}</small
            ></button
          ><ResourceEmptyState
            v-if="!availableImages.length"
            title="没有可用图片"
            description="到图片库补充相应用途的图片。"
          />
        </div>
      </div>
      <template #footer
        ><t-button
          theme="default"
          variant="outline"
          @click="imagePicker = false"
          >返回</t-button
        ><t-button :disabled="!selectedImage" @click="useImage"
          >使用所选图片</t-button
        ></template
      >
    </ResourceOverlay>
    <ImageLibraryDetail :image="imageDetail" :references="imageDetail ? imageReferences(db,imageDetail) : []" @close="imageDetailId=null" />
    <ResourceOverlay
      :visible="imageForm.open"
      @update:visible="closeImage"
      :title="imageForm.id ? '编辑图片' : '上传图片'"
      :width="640"
      ><t-alert v-if="imageError" theme="warning" :message="imageError" />
      <ResourceForm>
        <t-form-item label="名称" :required-mark="true"><t-input v-model="imageForm.name" placeholder="请输入图片名称" /></t-form-item>
        <t-form-item label="内容类目"><t-select v-model="imageForm.category" :options="categories.map(value=>({label:value,value}))" /></t-form-item>
        <t-form-item label="图片文件" :required-mark="true"><t-upload theme="file" accept="image/png,image/jpeg,image/webp" :auto-upload="false" :files="[]" :file-list-display="null" :upload-button="null" :disabled="imageReading" :trigger-button-props="{content:'选择图片',theme:'default',variant:'outline',loading:imageReading}" :input-attributes="{'aria-label':'选择图片文件'}" tips="支持 PNG、JPEG、WebP，单个文件不超过 3MB" @select-change="readImage" /></t-form-item>
        <t-form-item :required-mark="true"><template #label>图片用途 <ResourceFieldHelp label="图片用途" content="现成封面直接用于发布；底图用于模板合成，可同时选择两种用途。" /></template><t-checkbox-group v-model="imageForm.uses" :options="['现成封面','底图']" /></t-form-item>
      </ResourceForm>
      <img
        v-if="imageForm.src"
        :src="imageForm.src"
        class="image-large"
        alt="上传预览"
      /><template #footer
        ><t-button variant="outline" @click="closeImage">取消</t-button
        ><t-button @click="saveImage">保存图片</t-button></template
      ></ResourceOverlay
    >
    <ResourceOverlay
      :visible="editorOpen"
      :title="editorPurpose === 'cover' ? '调整当前封面' : '封面模板设计'"
      :width="1360"
      :busy="editorSaving"
      @update:visible="cancelEditor"
      ><t-alert v-if="editorError" theme="warning" :message="editorError" />
      <div class="editor-heading">
        <label v-if="editorPurpose === 'template'"
          >模板名称<t-input v-model="editorName" /></label
        ><label v-if="editorPurpose === 'template'"
          >平台标签<select multiple v-model="editorPlatforms">
            <option v-for="p in platforms" :key="p">{{ p }}</option>
          </select></label
        ><label v-if="editorPurpose === 'template'"
          >类目标签<select multiple v-model="editorCategories">
            <option v-for="c in categories" :key="c">{{ c }}</option>
          </select></label
        ><t-button variant="outline" @click="openImages('editor')"
          >从图片库选底图</t-button
        >
      </div>
      <PrototypeEditor
        ref="imageEditorRef"
        v-if="editorOpen"
        v-model="editorDoc"
        :fixed-size="editorPurpose === 'cover'"
        @ready="editorReady"
      /><template #footer
        ><t-button variant="outline" @click="cancelEditor">取消</t-button
        ><t-button
          v-if="editorPurpose === 'template'"
          variant="outline"
          @click="
            editorId = null;
            editorName += ' 副本';
            saveEditor();
          "
          >另存为模板</t-button
        ><t-button :loading="editorSaving" @click="saveEditor">{{
          editorPurpose === "cover" ? "应用到本条封面" : "保存模板"
        }}</t-button></template
      ></ResourceOverlay
    >
    <ResourceOverlay
      :visible="detailOpen && !detail?.assetId"
      @update:visible="detailOpen = $event"
      :title="
        detail?.items ? '计划详情' : detail?.assetId ? '任务详情' : '批次详情'
      "
      kind="drawer"
      :width="1040"
      :workspace="!!detail?.assetId"
    >
      <template v-if="detail?.items"
        ><h2>{{ detail.name }}</h2>
        <div class="metric-grid">
          <div>
            <small>目标任务</small><strong>{{ planTarget(detail) }}</strong>
          </div>
          <div>
            <small>已公开发布</small><strong>{{ planStats.published }}</strong>
          </div>
          <div>
            <small>完成率</small><strong>{{ planStats.rate }}%</strong>
          </div>
          <div>
            <small>目标差额</small><strong>{{ planStats.gap }}</strong>
          </div>
        </div>
        <p>
          {{ detail.control }} ·
          {{ planStats.gap ? "目标未达成" : "目标已达成" }} · 自动衔接阈值
          10%（只读）
        </p>
        <div class="tabs">
          <button
            v-for="v in ['总览', '按平台', '按类目', '平台 × 类目']"
            :key="v"
            :class="{ active: planView === v }"
            @click="planView = v"
          >
            {{ v }}
          </button>
        </div>
        <PlanWorkbench
          v-if="planView === '总览'"
          :db="db"
          :plan="detail"
          @batch="configureBatch"
          @edit="editPlan"
        />
        <div v-else class="wt-resource-page">
          <t-table
            class="wt-resource-table resource-data-table resource-embedded-table is-summary"
            row-key="name"
            :data="planGroups"
            :columns="[
              { colKey: 'name', title: '维度', minWidth: 200 },
              { colKey: 'target', title: '目标', width: 85 },
              { colKey: 'total', title: '已创建', width: 90 },
              { colKey: 'published', title: '已公开', width: 100 },
              { colKey: 'reviewing', title: '审核中', width: 90 },
              { colKey: 'failed', title: '确认失败', width: 95 },
              { colKey: 'pending', title: '待执行及在途', width: 120 },
              { colKey: 'unresolved', title: '人工 / 未知', width: 120 },
              { colKey: 'gap', title: '差额', width: 80 },
            ]"
            ><template #published="{ row }"
              ><t-button
                variant="text"
                @click="
                  related(null, detail.id);
                  filterPlatform = row.platform || '';
                  filterCategory = row.category || '';
                  filterStatus = '已发布';
                  applyQuery();
                "
                >{{ row.published }}</t-button
              ></template
            ><template #unresolved="{ row }"
              >{{ row.human }} / {{ row.unknown }}</template
            ></t-table
          >
        </div></template
      >
      <template #footer
        ><t-button
          v-if="
            detail &&
            !detail.items &&
            !detail.assetId &&
            associatedPlan(db, {
              kind: detail.assetId
                ? detail.batchId
                  ? 'batch'
                  : 'task'
                : 'batch',
              id: detail.batchId || detail.id,
            })
          "
          variant="text"
          @click="
            returnToPlan(
              associatedPlan(db, {
                kind: detail.assetId
                  ? detail.batchId
                    ? 'batch'
                    : 'task'
                  : 'batch',
                id: detail.batchId || detail.id,
              }),
            )
          "
          >返回关联计划</t-button
        ><template v-if="detail?.items"
          ><t-button variant="outline" @click="related(null, detail.id)"
            >查看所属任务</t-button
          >
          <t-button
            v-if="detail.control === '执行中'"
            variant="outline"
            @click="pausePlan(detail)"
            >暂停后续调度</t-button
          >
          <t-button
            v-else-if="!['已取消', '执行已结束'].includes(detail.control)"
            @click="startPlan(detail)"
            >启动 / 恢复计划</t-button
          >
          <t-button
            v-if="
              ['执行中', '等待继续确认', '等待跳过确认', '等待配置'].includes(
                detail.control,
              )
            "
            :disabled="currentDecision(detail) === '等待核实'"
            @click="advancePlan(detail)"
            >确认处理当前批次并衔接</t-button
          >
          <t-button
            v-if="!['已取消', '执行已结束'].includes(detail.control)"
            variant="outline"
            @click="cancelCurrentPlan(detail)"
            >取消计划</t-button
          > </template
        ><t-button theme="default" variant="outline" @click="detailOpen = false"
          >关闭</t-button
        ></template
      >
    </ResourceOverlay>
    <ResourceOverlay
      :visible="!!referenceRows"
      title="历史引用"
      @update:visible="referenceRows = null"
      ><p v-if="!referenceRows?.length">暂无正式引用；草稿不预占图片。</p>
      <p v-for="r in referenceRows || []" :key="r.kind + r.id">
        {{ r.kind }} #{{ r.id }} · {{ r.name }}
      </p>
      <template #footer
        ><t-button @click="referenceRows = null">关闭引用</t-button></template
      ></ResourceOverlay
    >
    <ResourceOverlay
      :visible="!!bulkResult"
      title="批量操作结果"
      :width="640"
      @update:visible="bulkResult = null"
      ><template v-if="bulkResult"
        ><p>
          {{ bulkResult.action }}：成功 {{ bulkResult.done.length }}，跳过
          {{ bulkResult.skip.length }}，失败 0，待确认 0。
        </p>
        <p v-for="r in bulkResult.skip" :key="r.id">
          {{ r.id }} · {{ r.reason }}
        </p></template
      ><template #footer
        ><t-button @click="bulkResult = null">完成</t-button></template
      ></ResourceOverlay
    >
  </div>
  <main v-else-if="loadError" class="publish-load-error publish-shell" role="alert">
    <ResourceEmptyState
      :title="page==='images'?'图片库加载失败':page==='templates'?'封面模板加载失败':'发布管理加载失败'"
      :description="`${loadError}。已有记录未被修改，请重新加载。`"
    >
      <t-button @click="retryLoading">重新加载</t-button>
    </ResourceEmptyState>
  </main>
  <t-loading v-else fullscreen :text="['images','templates'].includes(page)?`正在加载${names[page]}…`:'正在加载任务…'" />
</template>
<style scoped src="./publishing.css"></style>
<style scoped src="./taskList.css"></style>
<style scoped>
</style>
