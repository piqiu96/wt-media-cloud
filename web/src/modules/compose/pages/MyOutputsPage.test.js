import { readFileSync } from 'node:fs'
import { describe,expect,it } from 'vitest'
import MyOutputsPage from './MyOutputsPage.vue'
import ComposeOutputTable from '../components/ComposeOutputTable.vue'
import ComposeOutputMaterialDetail from '../components/ComposeOutputMaterialDetail.vue'
const read=file=>readFileSync(new URL(file,import.meta.url),'utf8')
const source=read('./MyOutputsPage.vue'),table=read('../components/ComposeOutputTable.vue'),material=read('../components/ComposeOutputMaterialDetail.vue')
describe('output asset workspace',()=>{
 it('compiles shared pages and separate material output detail',()=>{expect(MyOutputsPage).toBeTruthy();expect(ComposeOutputTable).toBeTruthy();expect(ComposeOutputMaterialDetail).toBeTruthy()})
 it('defaults to material aggregation and requests server pages with real totals',()=>{
  expect(source).toContain("activeView=ref('batches')")
  expect(source).toContain('client.listComposeOutputMaterials(params):client.listComposeOutputs(params)')
  expect(source).toContain('page:pagination.value.current,page_size:pagination.value.pageSize')
  expect(source).toContain('total.value=data.total||0')
  expect(source).not.toContain('.slice(start,')
  expect(material).toContain('material_id:props.material.material_id,page:page.value,page_size:pageSize.value')
 })
 it('keeps material rows detail-only and flat asset rows at most three actions',()=>{
  const batch=source.slice(source.indexOf('<template v-if="activeView'),source.indexOf('<ComposeOutputTable v-else'))
  expect(batch).toContain('openMaterialGroup(row)')
  for(const label of ['下载','再次制作','去发布'])expect(batch).not.toContain(`>${label}<`)
  for(const label of ['详情','下载','再次制作'])expect(table).toContain(`>${label}<`)
  expect(table).not.toContain('file_size_bytes')
  expect(table).not.toContain('formatBytes')
  expect(table).not.toContain('<video')
  expect(table).not.toContain('>去发布<')
 })
 it('preserves cross-page selection with narrow checkboxes and truthful disabled publishing',()=>{
  expect(table).toContain("type:'multiple', width:44")
  expect(table).toContain('reserve-selected-row-on-paginate')
  expect(source).toContain('knownRows.value[row.id]=row')
  for(const text of [source,material]){expect(text).toContain('class="wt-selection-toolbar"');expect(text).toContain('<t-button variant="outline" disabled>去发布</t-button>')}
  expect(source).toContain('发布能力尚未交付')
 })
 it('keeps role category boundaries and advanced date/video filters without local-page filtering',()=>{
  expect(source).toContain('isAdmin.value||isSenior.value||own.has(category.id)')
  expect(source).toContain('missingOperatorOption(filters.value.operator,operatorFilterOptions.value)')
  expect(source).toContain('operatorIdFromOption(filters.value.operator)')
  expect(source).toContain('params.operator_id=namedOperator')
  expect(source).toContain('class="wt-advanced-filters"')
  expect(source).toContain('min-width:280px')
  expect(source).toContain('video_status:filters.value.video_status')
  expect(source).not.toContain('filters.download')
  expect(source).not.toContain('listComposeTasks({})')
 })
 it('opens fresh verified video URLs and passes original source context to the shared creation dialog',()=>{
  expect(source).toContain('client.getComposeOutputVideoUrl(id)')
  expect(source).toContain('publicObjectReadUrl(link?.url)')
  expect(source).toContain(':source-output="createSource"')
  expect(source).toContain('createSource.value=data')
  expect(source).toContain("router.push({name:'Compose',query:{batch_id:id}})")
  expect(source).not.toContain('archiveComposeOutput')
 })
})
