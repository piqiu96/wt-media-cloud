import { describe, it, expect } from 'vitest';
import { operationURL, readOperationRoute, navigationGroups, legacyOperationURL } from '../routes.js';
import { syncProductionResources, productionPublication } from '../production/bridge.js';
import { publishingCategories } from '../model.js';

describe('统一工作台路由', () => {
  it('区分两种任务，详情可直接恢复，完整保留成片来源', () => {
    expect(operationURL('production-tasks')).toBe('/example/operations/production/tasks');
    expect(readOperationRoute(new URL('http://localhost/example/operations/publishing/tasks/12'))).toMatchObject({page:'tasks',taskId:12});
    expect(operationURL('tasks', {taskId:12})).toBe('/example/operations/publishing/tasks/12');
    expect(legacyOperationURL(new URL('http://localhost/example/?view=publishing#templates'))).toBe(operationURL('templates'));
    expect(legacyOperationURL(new URL('http://localhost/example/proto/content-operations/#tasks'))).toBe(operationURL('production-tasks'));
    expect(navigationGroups.flatMap(g=>g.pages).map(p=>p.id)).toHaveLength(10);
    expect(readOperationRoute(new URL("http://localhost/example/operations/settings/publishing")).page).toBe("materials");
    expect(navigationGroups.flatMap(g=>g.pages).some(p=>/overview|works|interact|replication/.test(p.id))).toBe(false);
  });
});
describe('生产成片与新版发布关联', () => {
  const production = () => ({outputs:[{id:128,materialId:101,taskId:1,name:'测试成片',owner:'陈小雨',platforms:['百度','B站'],assignees:{百度:'陈小雨','B站':'林予安'},cloudFileExists:true,cloudUrlVerified:true}],materials:[{id:101,title:'测试素材',category:'剪辑教程'}],tasks:[{id:1,snapshot:{strategyName:'策略A'}}],accounts:[{id:301,name:'百度账号',platform:'百度',enabled:true}]});
  it('按来源稳定映射ID，不覆盖已有资源，重复同步不重复创建', () => {
    const db={assets:[{id:128,title:'既有成片'}],accounts:[],tasks:[]}, source=production();
    syncProductionResources(db,source,'陈小雨');
    const resource=db.assets.find(a=>a.sourceOutputId===128);
    expect(resource.id).not.toBe(128);
    expect(resource.publishingPlatforms).toEqual(['百家号']);
    expect(resource.category).toBe('剪辑教程');
    expect(db.accounts[0]).toMatchObject({platform:'百家号',categories:['剪辑教程']});
    syncProductionResources(db,source,'陈小雨');
    expect(db.assets).toHaveLength(2);expect(db.assets[0].title).toBe('既有成片');expect(db.accounts).toHaveLength(1);
    db.tasks.push({id:21,assetId:resource.id,platform:'百家号',status:'执行失败'});
    expect(productionPublication(db,128,'百度').key).toBe('reserved');
    db.tasks[0].status='已取消';expect(productionPublication(db,128,'百度').key).toBe('reserved');
    db.tasks[0].status='已放弃';expect(productionPublication(db,128,'百度').key).toBe('unarranged');
    db.tasks.push({id:22,assetId:resource.id,platform:'百家号',status:'已发布'});
    expect(productionPublication(db,128,'百度').task.id).toBe(22);
  });
  it('移除或失效成片阻止新引用，历史任务和快照保持原值', () => {
    const db={assets:[],accounts:[],tasks:[]},source=production();
    syncProductionResources(db,source,'陈小雨');const id=db.assets[0].id;
    db.tasks.push({id:1,assetId:id,status:'已发布',snapshot:{title:'历史标题'}});
    source.outputs=[];syncProductionResources(db,source,'陈小雨');
    expect(db.assets[0]).toMatchObject({ready:false,fileAvailable:false});
    expect(db.tasks[0].snapshot.title).toBe('历史标题');
  });
});

it('过期或缺失URL的成片与生产页一致，不能成为可发布资源',()=>{
  for(const fields of [{cloudUrl:'https://example.test/output.mp4',cloudUrlExpiresAt:'2020-01-01'},{}]){
    const source={outputs:[{id:1,materialId:1,taskId:1,platforms:['B站'],assignees:{'B站':'陈小雨'},cloudFileExists:true,cloudUrlVerified:true,...fields}],materials:[{id:1,category:'A'}],tasks:[],accounts:[]};
    const db={assets:[],accounts:[],tasks:[]};syncProductionResources(db,source,'陈小雨');expect(db.assets[0].fileAvailable).toBe(false);
  }
});

it('生产资源与演示类目共同出现在新版发布范围中',()=>{
  const db={assets:[{category:'拍摄技巧'}],accounts:[{categories:['剪辑教程']}]};
  expect(publishingCategories(db)).toEqual(expect.arrayContaining(['三角洲行动','拍摄技巧','剪辑教程']));
  expect(new Set(publishingCategories(db)).size).toBe(publishingCategories(db).length);
});
