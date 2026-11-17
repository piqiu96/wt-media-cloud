# 公共视觉与交互参考

> 本目录已迁入 Cloud。新原型参考 `common/ComponentDemo.vue`，直接使用 `src/shared/ui/` 和正式主题；以下 HTML 结构及 starter 继续服务已有原型维护。设计工作稿只维护在 Workspace `docs/design/`。

通用设计协同方法见 [AI Product Design Workflow Skill](https://github.com/piqiu96/wt-media-workspace/blob/main/skills/common/ai-product-design-workflow/SKILL.md)。本目录仍是公共视觉/交互参考，不是该 Skill 的替代品，也不是运行时组件库。

[打开参考 HTML](index.html) · [正式规范入口](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/README.md) · [原型起始页](../proto/starter.html)

这是正式规范的可操作参考，使用原生HTML展示布局、样式与关键状态，便于人工和AI查看源码、复用结构。窗口与代理数据均为虚构示例，借具体信息展示公共模式；这里不定义该业务的正式字段、权限或执行能力。

## 文件与引用

| 文件 | 职责 |
|---|---|
| `index.html` | 七组公共模式、示例条款索引及Dialog/Drawer内容模板 |
| `example.css` | 同一份示例Token、密度、布局和状态样式；来源标注在文件顶部 |
| `controls.js` | 原生select的数据桥、统一选项浮层与更多菜单；原型演示专用 |
| `example.js` | 导航、筛选、选择、焦点/关闭、字段校验、手动结果切换等本地演示行为 |

本目录无构建、CDN、图片或业务服务依赖，支持直接打开 `index.html`。浏览器支持原生 `dialog`、Popover和容器查询；不支持Popover时选择器保留原生select降级；当前Cloud/Desktop实际组件的兼容性仍应在真实运行环境验证。

未来原型先找到对应模式，点击“查看 / 复制结构”，或直接读取HTML中的 `data-scene` / `template`。将所需结构与关联模板复制到原型，引用公共样式、控件脚本与演示脚本；不把整套七组展示目录复制到每个业务原型。

```html
<!-- example/proto/<业务主题>/index.html -->
<link rel="stylesheet" href="../../common/example.css">
<script src="../../common/controls.js" defer></script>
<script src="../../common/example.js" defer></script>
```

从 [starter.html](../proto/starter.html) 开始时，其相对路径适用于 `proto/` 这一层；放入主题子目录后调整为上面的路径。需要新建表单时保留 `tpl-window`、`data-local-form`、`name="name"`、字段错误节点和 `data-form-receipt`，并替换业务字段与标题。它们只驱动本地原型，不对应业务API。

## 模式与规范出处

| 模式 / 直接入口 | 可查看的内容与状态 | 规则事实源 |
|---|---|---|
| [资源列表](index.html#list) | 8行、独立ID、多平台关联账号、上置筛选、展开低频条件、勾选工具栏、5项文字操作、筛选无结果 | [AdminUI §2–§4](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/AdminUI页面规范.md)；[架构视觉 §3.7、§4.1](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/前端架构与视觉规范.md) |
| [自适应详情](index.html#details) | 字段密集/稀疏切换；800/560/380px内容容器；横向键值、长内容跨行、配置对比与真实关系表 | [AdminUI §7.2](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/AdminUI页面规范.md)；[架构视觉 §4.4.2](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/前端架构与视觉规范.md) |
| [新建与编辑](index.html#forms) | 短表单Dialog560/640px；长配置Drawer800px；相关短字段成对、地址备注满行；真实Radio选择卡片；校验与输入保留 | [AdminUI §6](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/AdminUI页面规范.md)；[交互 §8–§9](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/前端交互规范.md) |
| [选择与确认](index.html#overlays) | 资源选择960px、空选择560px；筛选、可用/不可用项、补充属性原位展开、完整详情及返回；风险确认、无Footer摘要与960px复杂详情 | [AdminUI §6.1、§7.4](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/AdminUI页面规范.md)；[交互 §9](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/前端交互规范.md) |
| [反馈与空态](index.html#feedback) | 帮助/影响/风险/错误分层；空资源、无匹配、不可用、加载失败、无权限、加载中 | [用户反馈规范](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/用户反馈规范.md) |
| [批量过程与结果](index.html#batch) | 居中摘要、同区七类计数、进度与8项原因；处理中/成功/失败/部分/待确认/中断；关闭保护、仅失败重试和待确认核实 | [AdminUI §7.7](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/AdminUI页面规范.md)；[交互 §11](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/前端交互规范.md) |
| [视觉基础](index.html#foundations) | 配色、字号、36px控件、按钮禁用/Loading、操作收纳、配置/事实/结果状态 | [架构视觉 §3.6–§3.9](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/前端架构与视觉规范.md) |

详情的容器查询以实际内容宽度生效，字段元信息区分短字段、长字段与稀疏内容。示例中的640px断点是当前字号和标签宽度下的实现选择，不升级为所有业务的固定断点；以正式规范的密度、长度、可读性要求判断。表格在窄屏内滚动，不缩小整页或隐藏关键列。

## 交互边界

- 所有按钮只作用于本地演示。没有请求、外部写入、业务持久化或定时自动成功；保存入口明确写“保存演示输入”。重试与核实只手动改变对应类别的演示项目。
- 原生Dialog管理焦点和下层不可交互；仅顶层绘制遮罩，返回时恢复触发入口。表单不通过遮罩关闭；处理中阻止X、Esc和关闭按钮。批量弹窗内“演示控制”可手动推进到结果，不是业务动作。
- `example.js` 是演示工具，不是公共业务组件库。真实实现继续使用TDesign Vue Next、TDesign Icons和所属运行仓的shared/ui，主题集中映射；不能将此CSS/脚本引入Cloud、Agent或Desktop Runtime。
- 演示校验仅覆盖模式所需的名称、端口等原生约束，不代表完整业务校验。示例“应用代理”等动作不证明真实服务或外部执行已成功。

## 维护与验收

正式规范维护规则，HTML维护示例，两者相互引用。规则变化先改所属正式文档，再同步当前示例；发现冲突时按事实源与审批记录修正，不以原型单方面覆盖规则。公共Token只在 `example.css` 一处映射，业务原型优先引用，不复制取值。

修改示例时检查：

1. Token与架构视觉§3.6.1一致，尺寸/密度与已审批档位一致。
2. 1600×1000、1366×900、窄屏/窄容器的内容和操作可达，列表含8行与长内容。
3. 键盘选择与焦点、名称校验、输入保留、嵌套返回、处理中关闭保护。
4. 批量总数等于六类互斥状态之和，失败重试与待确认核实互不串类。
5. `file://` 可运行，资源与规范链接有效，无控制台错误、联网依赖或假业务结果。

浏览器截图仅按评审需要保留最终效果，无需为每个原型场景批量导出；临时核验截图不进入example/evidence，规范、HTML和必要源码是日常复用入口。HTML不代替真实Vue组件回归、人工视觉审核或真实业务验收。

## 当前交互约定

公共分页保留总条数、每页条数、当前页和翻页入口，不展示“跳至第几页”输入框。真实页面使用 TDesign Pagination 并显式关闭跳页输入。

少量账号名单和代理补充属性在当前区域展开，完整详情才打开新层。窗口主要行操作“打开”置前，使用32px浅蓝轻按钮；详情/编辑/设置等常规动作使用清晰品牌蓝、悬停浅底，危险文字操作默认红字/悬停浅红底，禁用独立浅灰；超过5项才收纳，更多采用浮动菜单。筛选横距24px、行距20px、标签间8px，容器不足时整组换行。原型选择器展示统一选项面板与键盘状态，原始select继续承载值、重置和change事件。

动态插入原型内容后调用 `WTControls.enhance(container)`；现有示例弹层已自动接入。制作新原型时先加载controls.js再加载example.js。完整详情进入下一层时保留原先筛选/选择，Esc优先关闭当前选项或菜单，不连带关闭父Dialog。

当前操作层级：主要任务用浅底表达层级，其余可执行动作保持明确点击识别。复用时给该行的主要动作增加 `ex-row-primary`，不要通过按钮文案自动猜测业务优先级；每行至多一个主要轻按钮。

“打开”按钮使用纯文字，不保留前置图标；浅底与32px热区保留。完整业务原型见[窗口与代理](../proto/browser-proxy/index.html)。本目录只保留当前稿，存储边界见[example约定](../README.md#当前稿保留)。


## 下方展开式筛选（2026-11-04）

公共资源列表统一采用“基础条件＋查询／重置 → 下方左对齐高级筛选 → 展开条件”。入口不挤入查询按钮组，不因基础字段换行漂移；不增加第二套操作按钮。只改变可见性，不清空输入，不自动查询。入口按已应用条件显示“已生效 N 项”（日期范围为一项，未查询草稿不计入）；重置后同步恢复默认条件与计数。页面的校验错误不能被折叠隐藏。

引用 `example.css`，面板加 `ex-list-pattern`，筛选表单加 `ex-filters-stacked`：

```html
<section class="ex-panel ex-list-pattern">
  <form class="ex-filters ex-filters-stacked">
    <!-- ex-field / ex-filter-search：基础字段，标签在上 -->
    <div class="ex-actions"><button class="ex-button primary" type="submit">查询</button><button class="ex-button" type="reset">重置</button></div>
    <details class="ex-filter-advanced">
      <summary>高级筛选<span class="ex-filter-applied">（已生效 1 项）</span></summary>
      <div class="ex-filter-extra"><!-- 日期范围用 ex-field ex-filter-range；其他字段复用 ex-field --></div>
    </details>
    <!-- 校验反馈放在 details 外部 -->
  </form>
  <!-- ex-table / ex-selection -->
  <footer class="ex-pagination ex-pagination-compact"><!-- 总数、条数选择器、ex-page-number、当前页 --></footer>
</section>
```

`details/summary` 原生提供键盘与展开语义，页面只维护展开状态和已应用计数，不监听展开来发起查询。字号：标签13px/500、输入值及查询按钮14px、展开入口14px、列表正文与排序动作13px、辅助文字12px；控件36px，横距24px、行距20px、标签间8px、展开内容距入口16px。使用CSS逻辑像素，不能按高分辨率截图尺寸放大。

排序复用 `ex-sort-button` 和16px的 `ex-sort-icon`，活动字段用 `is-active`、升序用 `is-ascending`；表头维护 `aria-sort`，按钮说明当前状态和下一步操作。分页复用同一个紧凑模式，选择器显示完整“20 条 / 页”，无跳页框。多选、日期控件继续遵循各自原生／官方组件行为，不由本样式接管业务数据。

当前显式接入：本目录资源列表、内容运营素材库、我的素材。公共样式通过新类生效，其他原型不会自动迁移布局；运行时Vue页面须按所属工程规范另行实施，不引用离线脚本。

## 共享图片编辑器

`src/shared/ui/media/ImageEditor.vue` 提供与业务无关的图层编辑、背景图片填充／定位、画布规格、文字排版、坐标和拖动缩放、显隐／锁定、图层排序、撤销重做与 PNG 导出。通过 `v-model` 接收与返回设计文档；`ready` 返回初始化后的文档，便于建立未保存修改基线。`export` 返回用户主动导出的 PNG；`exportPng(false)` 可生成图片而不触发下载，`setBackground(src)` 支持资源选择器填充底图，`showPreview()` 打开成品预览，`hasOverflow` 暴露溢出状态。业务已确定输出比例时可传 `fixedSize` 固定规格。

排版规则在同目录 `textLayout.js`，预览与导出共用测量结果，溢出时阻止导出。默认使用系统字体，本地图片支持 PNG／JPEG／WebP；不包含业务素材查询、发布 API 或在线图片代理。可在[发布新版原型](/example/?view=publishing#templates)中实际操作。


`src/shared/ui/media/MediaPreview.vue` 提供共享图片预览：通过 `v-model:visible` 控制显示，`images` 接收 `{ src, label, note }` 列表，支持适应窗口、小图效果和实际尺寸；可用 `details` 插槽展示当前业务摘要。预览通过公共浮层关闭并返回上层编辑器，不要求先保存或下载。编辑器生成的 PNG 在该组件中直接检查，不用带选中框的画布替代成品。

`imageOutput.js` 提供无业务含义的图片适配输出，默认完整保留源图；平台比例由调用方提供。`ResourceWizardSteps.vue` 在宽屏使用公共步骤，在窄屏显示当前步骤、进度和下一步，避免步骤标题挤成逐字换行。二者不包含发布平台字段、资源占用或任务语义。

独立详情组合在 `/example/#components` 的“独立详情”入口演示：直接使用 `ResourceDetailPage`、`ResourceCard`、`ResourceSection` 和 `ResourceDescriptions`，未添加专属外层白卡。返回由公共外壳在底部统一提供，业务操作由页面提供；没有业务动作时仍保留“返回”，调用方用 `returnButton=false` 显式关闭才省略。字段布局支持名称在首行满行、短字段双列及窄容器单列。规则分别见 Workspace AdminUI §7、交互 §9.3–9.4、架构视觉 §4.1–4.2。发布任务、批次、计划是本轮业务接入样例，指标和执行逻辑均留在原模块。

`ResourceProgress` 统一标签、分子/分母、百分比和细轨道，仅展示调用方提供的值；多个指标的排列由页面组织。`src/shared/styles/resource-data-table.css` 通过显式的 `resource-data-table` 容器统一 TDesign 与既有原生表格的表头、边距、行高、边界和悬停；`resource-table-number` 标识右对齐数值，原生编辑页可使用 `resource-table-scope`。不接管排序、分页、输入、权限和保存逻辑，不影响未接入表格。公共组件示例可查看实际进度与表格样式。


## 公共字体与阅读节奏

公共参考与 Vue 主题同步使用 `--wt-font-family`、`--wt-font-size-*`、`--wt-line-height-*`。页面标题24/34，正文14/24，表格13/22，辅助说明12/20；表单标签13px/500，输入值14px/400。发布计划的白卡、浅灰页面和克制分区作为密集配置页面的视觉参考，具体布局仍按当前任务选择。限制与修复说明保持次级文字色，不使用装饰性浅灰；统计数字按需采用等宽数字。控件36px、行操作32px、按钮数量门槛及业务状态不随字体调整改变。


## 发布任务／批次／计划的整页模式

三类页面按同一组模式维护，新增页面和改动提交同时检查下表；业务字段、状态、权限及步骤仍归对应业务，不复制到公共组件。

| 页面 | 复用入口 | 一致性要求 |
|---|---|---|
| 列表 | ResourcePageHeader / Card / FilterBar / RowActions / resource-data-table.css | 标题24px/500，右上一个创建主操作；标签在上；表格13px/22px；分页10／20／50（默认10），无跳页；最多5项操作平铺、超过5项才收纳 |
| 完整新增／编辑 | ResourceEditorPage（复用 ResourceDetailPage 的页头和动态 footer 避让） | 单一页面标题、底部统一“返回”；白卡分区；字段13px标签＋14px输入；右下一个主操作；状态说明靠左；隐藏原列表，禁止用整页遮罩模拟工作区 |
| 详情 | ResourceDetailPage / Section / Descriptions / Notice / Progress | 与编辑共用外壳；正文只读；信息去重；全局动作仅 footer，始终保留底部“返回” |
| 短确认／资源选择 | ResourceOverlay | 18px标题、36px控件、标准560／640／960档位；只完成当前子任务；关闭回原位置，保留输入 |
| 流程步骤 | ResourceWizardSteps | 真正分阶段才用步骤条；整页分区跳转用锚点；不能把静态分区编号做成可点击假步骤 |

发布原型的 PrototypeOverlay 仅负责把既有业务表单放到页面宿主，业务保存、离开保护、资源锁定和权限函数保持原归属。完整新增／编辑使用 `page` 显式接入；候选选择与短确认仍使用弹窗；批次候选的局部内容精修按已确认设计使用 960px ResourceOverlay 弹窗，文案、封面与平台设置纵向排列，主体滚动且底部固定取消／保存；保留未保存保护及原列表筛选、勾选、分页。完整发布任务新增／编辑继续使用整页。计划保留现有原生页面与业务存储，通过公共 Token、表格适配及同一几何规范对齐，不复制一份计划逻辑。Vue 公共示例“页面编辑示例”直接引用同一组件。

每次回归覆盖：三类列表 → 创建／继续草稿 → 编辑内容 → 详情 → 对应确认；有异常、只读、无结果、长文案与窄屏时也要核验。检查主操作唯一、按钮层级、无重复标题、表格内滚动、footer 不遮内容、资源弹窗返回和未保存保护。截图与测试记录是验证证据，不代替用户验收。

整页编辑的卡片区使用 `resource-editor-section`，统一24px内边距、8px圆角和公共边界；窄屏改为20px/16px。计划的原生编辑页采用相同固定操作栏，并按实际栏高预留底部空间。密集列表的操作列保持可见，中间业务列在表格容器内横向滚动。


## 2026-11-14 common 统一落地

发布任务、发布批次、发布计划、图片库、封面模板同步采用本目录及 shared/ui 公共入口。列表显式接入 `resource-list-page` / `resource-list-panel` 和 `src/shared/styles/resource-list.css`；只涉及界面结构和视觉，不接管业务筛选、权限、状态或草稿。原生计划保留原生业务，用 common Token 和本地适配器对齐；图片、模板的短表单保留弹窗。

- 标题24px/500/34px；分区16px/500/24px；标签13px/500，表单值14px/400，表格13px/22px，辅助12px/20px。
- 标题到首块20px；区块间16px；分区标题到正文16px；卡片内边距24px，窄屏20px/16px。
- 详情标识3px × 14px、圆角1px，与标题间隔8px；表单无装饰横线。
- 表头 `#f1f5fb` 与边界 `#dce2e9` 区分；筛选、表格、分页共用外边界，表格首尾内缩与筛选保持一致。
- `ResourceAdvancedToggle` 固定“高级筛选”，右侧箭头，单独一行；不因展开自动查询。
- `ResourceFieldHelp` 将说明放到字段／分区名称旁的问号，向上弹出，支持聚焦与 Esc；真实校验错误仍在对应字段可见。原生参考使用 `ex-field-help`。
- `ResourceDetailPage` / `ResourceEditorPage` 顶部只有标题，底部内置“返回”，其余动作经 footer 槽传入；动态测量操作栏高度，窄屏换行也不遮正文。底部状态用 `resource-footer-status` 靠左。

公共推广不意味着同步未在本轮范围的业务页面布局，也不替代真实平台执行测试。截图与隔离浏览器走查见 Workspace CHG-20261111-101 本轮证据。


发布计划列表通过 `plan/PlanList.vue` 直接复用 `ResourcePageHeader`、`ResourceCard`、`ResourceFilterBar`、`ResourceAdvancedToggle`、`ResourceRowActions`、`ResourceEmptyState` 和 TDesign Table/Pagination；与发布批次共用 `resource-list.css` / `resource-data-table.css`。筛选区、表格与分页处于同一白卡，表头44px，控件36px，分页默认10条并支持20／50条；名称和编号同列分层显示。业务筛选、行操作、权限及列表上下文仍由计划 `app.js` 持有。低频条件按查询生效，已移除的“待办范围”不再参与筛选。

### 表单说明与保存操作

发布任务、发布批次、发布计划、图片库与封面模板使用同一规则：必填字段使用红色 `*`，不添加“选填／可选”标签。普通字段与按钮不加解释；涉及特殊口径、默认行为或容易混淆的业务概念时，在字段名旁使用 `ResourceFieldHelp` 问号悬浮说明。动态校验错误、异常原因与操作结果直接展示，不能藏进说明。单条发布内容编辑只保留一个“保存”，保存时完成内容校验与确认；失败时保留输入并显示原因。

### 表单按钮、内嵌列表与业务待办汇总

表单自身的操作（含页脚、弹窗和工具栏）统一实底：主操作品牌蓝，每个操作区最多一个；辅助操作中性浅灰底、深色字；危险最终确认红色实底。Vue 原型统一引用 `src/shared/styles/resource-form-actions.css`，不以页面私有样式逐个补按钮。名称链接、问号、分页与选择控件保留原形态；表单或详情内嵌表格的行操作仍复用 ResourceRowActions，不改为实心按钮。

独立列表与内嵌表格共享字体、状态语义及行操作层级，容器按以下两类区分：主要动作浅蓝底置前，其余蓝字，危险动作默认红字、悬浮浅红底；行热区32px，间距8px，五项以内平铺，超过五项时四项＋更多。任务行由发布原型的 TaskRowActions 与 taskActionItems 同时供发布任务和批次详情使用；批次上下文权限仍由原生命周期规则判断。结束流程的列表入口称“终止”，表单关闭仍称“取消”，“停止发布”保留既有语义。

业务待办汇总位于所属工作区标题下、筛选及列表之前，不能放在列表尾部；单项异常仍留在对应行。汇总与“查看待处理”筛选使用同一口径，处理后自动更新，不重复在页脚展示提示框。批次详情的执行情况展示发布目标、已发布、发布失败、待处理四项，下方仅保留发布成功率；未处置失败也计入待处理，两项不可相加。批次列表发布进度仅显示完成数／总数，不显示百分比。

### 两类表格与提示反馈的适用边界

| 类型 | 接入方式 | 容器与布局 |
| --- | --- | --- |
| 独立列表页 | 既有 resource-list-page / resource-list-panel | 筛选、表格和分页共用外层面板；表格本身不重复加圆角与外框。本轮保留设计，仅回归。 |
| 详情、表单和选择弹窗内嵌表格 | resource-data-table resource-embedded-table | 浅灰表头、白色数据行，无阴影、无竖向网格线。与外层保持留白的独立小表格保留8px圆角细边框；贴边工作区及弹窗主体加 is-flush，取消表格外围边框与圆角，由外层提供边界。 |

内嵌表格表头44px，普通行48px，双行摘要加 is-summary、编辑配置加 is-editable 后以64px为基准，内容可自然增高。字号沿用公共13px/22px，辅助12px/20px。名称列弹性伸展，数量、状态、操作列按内容收紧；数量列显式加 resource-table-number。编辑控件36px，数字控件默认128px。固定列保持轻边界，横向滚动发生在表格内，浮层正常挂载，不被圆角裁切。同一工作区仅保留一层主要外边界，不能同时出现贴边卡片框与表格框。Tabs与独立有框表格之间保留16px间距，避免Tabs基线与表格圆角相碰形成突出短线。ComponentDemo 的详情“关联资源”展示 is-flush 衔接方式；详情目标矩阵展示有留白的独立小表格。横向滚动、固定操作列和分页的边界必须连续，不裁掉实际内容或浮层。

表格内资源数量和名称等查看入口使用蓝色文字，无默认底色；普通数据不做按钮块，只有可编辑字段显示控件。行操作继续使用 ResourceRowActions，不能被外层表单实底按钮规则覆盖。

ResourceNotice 提醒条使用紧凑布局，单行含操作约50px，操作为32px热区的蓝色文字按钮，悬浮浅蓝底。Toast保持短时反馈，辅助操作也使用文字入口。表单自身保存、提交、页脚以及最终确认仍按实底规则，提示条和表格入口是显式排除项。
