import { describe, it, expect } from 'vitest'
import { assignableOwners, ownershipIssue } from '../plan/ownership.js'
const self={id:'u1',name:'自己',role:'运营',team:'A',categories:['游戏攻略','精彩剪辑']}
const senior={...self,role:'高级运营'}
const members=[self,{id:'u2',name:'同组',role:'运营',team:'A',categories:['游戏攻略','精彩剪辑']},{id:'u3',name:'其他组',role:'运营',team:'B',categories:['游戏攻略','精彩剪辑']},{id:'u4',name:'类目不符',role:'运营',team:'A',categories:['精彩剪辑']},{id:'u5',name:'停用',role:'运营',team:'A',categories:['游戏攻略'],disabled:true}]
describe('计划负责人遵循角色、团队与负责类目',()=>{
 it('普通运营只能选择自己',()=>{expect(assignableOwners(self,members,['游戏攻略']).map(x=>x.id)).toEqual(['u1'])})
 it('高级运营只能指派同团队且覆盖计划类目的有效成员',()=>{
 expect(assignableOwners(senior,members,['游戏攻略']).map(x=>x.id)).toEqual(['u1','u2'])
 expect(assignableOwners({...senior,categories:['精彩剪辑']},members,['游戏攻略'])).toEqual([])
 })
 it('管理员可跨团队指定覆盖计划类目的有效成员',()=>{expect(assignableOwners({...self,role:'管理员'},members,['游戏攻略']).map(x=>x.id)).toEqual(['u1','u2','u3'])})
 it('阻止越权改派、无效成员和伪造名称',()=>{
 expect(ownershipIssue({owner:'其他组',ownerId:'u3',categories:['游戏攻略']},null,senior,members)).toBeTruthy()
 expect(ownershipIssue({owner:'同组',ownerId:'u2',categories:['游戏攻略']},null,self,members)).toBeTruthy()
 expect(ownershipIssue({owner:'伪造',ownerId:'u2',categories:['游戏攻略']},null,senior,members)).toBeTruthy()
 })
 it('编辑未改派时保留历史负责人，不随当前身份覆盖',()=>{
 const original={owner:'历史负责人',categories:['游戏攻略']}
 expect(ownershipIssue({...original,name:'修改名称'},original,self,members)).toBe('')
 expect(ownershipIssue({...original,owner:'其他人'},original,self,members)).toBeTruthy()
 })
})
