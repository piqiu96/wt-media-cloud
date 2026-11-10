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

少量账号名单和代理补充属性在当前区域展开，完整详情才打开新层。窗口主要行操作“打开”置前，使用32px浅蓝轻按钮；详情/编辑/设置等常规动作使用清晰品牌蓝、悬停浅底，停用用深色/悬停红色，禁用独立浅灰；超过5项才收纳，更多采用浮动菜单。筛选横距24px、行距20px、标签间8px，容器不足时整组换行。原型选择器展示统一选项面板与键盘状态，原始select继续承载值、重置和change事件。

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
