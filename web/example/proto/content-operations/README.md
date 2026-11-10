# 内容运营原型与评审

本目录为内容生产、发布管理、作品管理、互动管理及爆款复刻的离线交互原型。内容生产的产品与原型已由用户确认完成，规则已回写[PRD 第五章](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/product/prd/详细文档/第五章_素材生产.md)；其他模块仍在设计评审，不因本次回写自动验收。

## 当前入口

- [完整原型](index.html#materials)
- [内容生产：产品／工程设计导航](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/design/content-production.md)
- [内容生产逐页评审](review.html?scope=production#page-review)
- [发布管理新版原型](/example/?view=publishing)：按发布设计稿与三平台字段取舍重建，包含任务、批次、计划、图片库和模板编辑；共享图片编辑器位于 `src/shared/ui/media/`。所有执行仍为本地演示。
- [发布管理逐页视觉评审](publishing-review.html)：以完整初版为底稿，参考社媒账号的场景目录，22 个页面与关键状态，复用 common；保留整页计划、完整选择器与任务执行区；支持本页意见保存与导出。当前为待评审视觉方案，不接真实发布。
- [设计索引](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/design/content-operations.md) · [发布管理](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/design/publishing.md) · [作品管理](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/design/works.md) · [互动管理](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/design/interaction.md)
- [完整评审及人工意见](review.html#page-review)

正式技术方案维护在[既有内容生产工程设计](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/engineering/specs/2026-09-24-m4-m5-cloud-content-production.md)，实施计划和验收进入对应 CHG。本目录不再保存产品讨论历史、技术方案副本或第二套交付状态。

## 页面范围

| 范围 | 当前原型内容 |
| --- | --- |
| 运营概览 | 按用户意见暂缓设计 |
| 素材库／我的素材 | 公共素材与个人关系分开，加入不自动准备；我的使用／团队使用按成功成片计算，下载依赖当前用户及设备事实 |
| 合成策略 | 多组成片设置、模板参数、选材条件、手动／定时、立即执行预览与未完成任务检查 |
| 合成任务 | 任务批次／任务列表，批次详情看全批进度；新建任务选择素材，仅调整模板开放参数；逐次执行记录、取消及受保护恢复 |
| 成片管理 | 按素材 ID 聚合与成片列表；基本信息、视频信息、制作来源和发布情况；下载、再次制作和勾选去发布 |
| 发布任务／计划 | 新建发布任务与成片去发布共用表单；多日编排由新建发布计划承担；完整方案仍在发布工作稿评审 |
| 作品／复刻 | 作品列表、表现观察、复刻流程及关联产物；后续以对应设计稿确认范围 |
| 主动互动／评论 | 账号与动作集合、异常核实、评论回复与连续处理；完整优化仍待讨论 |

内容生产采用公共高级筛选、单一日期范围、字段悬浮帮助、多选工具栏、窄选择列和统一行操作。新增／复制设置、增删条件保留草稿与滚动位置。参数调整使用按组弹窗，不重复展开整套策略编辑表单。

素材来源当前仅抖音，成片适用／发布目标为 B站、百度、小红书，不提供抖音发布账号候选。旧关联不改写，失效平台不能继续新建；真实账号与发布能力须由对应领域合同实现，候选样例不代表平台已接入。

## 浏览与目录

通过统一原型预览服务打开：`http://127.0.0.1:8768/example/proto/content-operations/`（Cloud 独立预览服务）。

`index.html` 为工作台，`workspace.*` 为公共导航，`production/`、`publishing/` 为业务模块，顶层脚本承接选择器、作品、复刻与互动。`review.html` 是统一评审入口，`review-production.*` 为内容生产评审展示；`compose-design.html` 是独立设计参考，当前可操作稿以工作台为准。

公共视觉与交互引用[正式规范](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/README.md)及 [common](../../common/index.html)，不在本目录复制设计系统。业务规则修改回写正式 PRD，工程约束修改回写现有工程设计。

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
