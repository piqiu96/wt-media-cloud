import { reactive } from 'vue'
// Demo identities only. These fixtures never grant production API permissions.
const categories = ['三角洲行动','火影忍者','通用','游戏攻略','精彩剪辑','游戏资讯','游戏评测','赛事复盘','新手教学','剪辑教程','拍摄技巧']
export const prototypePeople = [
  { id:'chen', name:'陈小雨', role:'高级运营', team:'内容运营一组', categories },
  { id:'lin', name:'林予安', role:'运营', team:'内容运营一组', categories:['三角洲行动','火影忍者','游戏攻略','精彩剪辑','剪辑教程','拍摄技巧'] },
  { id:'zhou', name:'周予宁', role:'管理员', team:null, categories },
]
export const prototypeSession = reactive({ actor:'陈小雨' })
export const currentPrototypeUser = () => prototypePeople.find(p => p.name === prototypeSession.actor) || prototypePeople[0]
