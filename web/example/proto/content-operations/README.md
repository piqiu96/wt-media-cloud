# 旧内容运营原型与评审

可操作工作台已收敛到 [统一原型](/example/operations/production/materials)，代码唯一位置为 `../publishing-design/`。运营概览和旧发布管理已删除；本目录只保留旧链接兼容入口、原评审工具和作品／互动／复刻的待重构参考，不再作为完整运营工作台运行。

内容生产的素材库、我的素材、合成策略、合成任务及成片管理已迁入新版。评审意见继续使用原键，设计及人工评审材料保留。旧发布存档不导入新版，也不再由本目录执行。

## 评审意见

沿用 `review.html` 和原有本地存储键；人工结论、评论、草稿不会随页面变更重置。历史评审证据按原样保留，不把旧评审结果当作新稿结论。JSON 可导出／合并导入，Markdown 仅供阅读；更换浏览器或服务地址后需用 JSON 交接。Git 不携带浏览器中的人工意见和演示业务数据。

## 演示与验证边界

原型中的账号、文件、合成、转存、下载、发布、互动与结果均为本地示例，无真实媒体传输或平台执行。规模演示可加载账号、成片、发布任务与评论样例，重复加载不重置原数据或评审意见。

任务批次关联用于演示，不能证明正式后端已有该字段；成片聚合按素材 ID，不能按名称或相近时间猜测。演示控制只用于观察状态与操作，不应进入正式运营界面。无真实视频、对象核验和系统文件桥时，入口明确提示演示，不外跳假地址或假装打开系统文件夹。

本次文档回写不修改运行软件、SQL、OpenAPI、账号或评审存储。真实 FFmpeg、存储完整性、并发与权限、本机下载和发布执行，须在正式落地后按工程设计和 CHG 验收；不以离线原型检查代替。

## 官方开关资源

`assets/ui-controls.entry.js` 使用应用现有 Vue 3.5.18、TDesign Vue Next 1.20.3 的 Switch 及官方主题样式。`assets/vendor/` 保留离线 bundle 和两项许可证，没有通过 CDN 加载依赖。从 Cloud `web/` 目录使用现有 esbuild 重新生成：

```js
const path = require('node:path');
const modules = path.resolve('node_modules');
require(path.join(modules, 'esbuild')).buildSync({
  entryPoints: ['example/proto/content-operations/assets/ui-controls.entry.js'],
  bundle: true, minify: true, format: 'iife',
  outfile: 'example/proto/content-operations/assets/vendor/ui-controls.js',
  nodePaths: [modules], define: {'process.env.NODE_ENV': '"production"'}
});
```
