# 发布交互原型：实现与真实验证记录

本记录对应 2026-11-10～11-11 的本地实施。四项重点 P0 及 AI、封面、资源库的核心闭环已通过规则回归和实际浏览器操作，现提交产品验收；**产品是否接受仍由用户裁定，不使用自评分代替验收**。所有发布结果及 AI 输出均为明确标注的本地演示。

## 范围、入口与存量审计

- 在 Cloud `web/` 启动：`npm run dev:example -- --host 127.0.0.1 --port 8768`。
- 入口：[任务](http://127.0.0.1:8768/example/?view=publishing#tasks)、[批次](http://127.0.0.1:8768/example/?view=publishing#batches)、[计划](http://127.0.0.1:8768/example/?view=publishing#plans)、[图片](http://127.0.0.1:8768/example/?view=publishing#images)、[模板](http://127.0.0.1:8768/example/?view=publishing#templates)。详情和编辑器由列表进入。
- 映射：既有 `example/` publishing 入口仍加载 `example/proto/publishing-design/App.vue`，内容生产工作台链接保留，没有建立另一套原型。
- 保留：原导航、任务/批次/计划列表、本地存储命名空间及用户已有原型数据。验收场景只新增独立虚构数据，不清库。
- 复用：`ResourceOverlay`、`ResourceWizardSteps`、`MediaPreview`、feedback、三栏 `ImageEditor`、`textLayout.js`、`imageOutput.js`。`PrototypeOverlay` 仅包装共享弹层，修复动画结束后的焦点恢复；`PrototypeEditor` 包装共享编辑器，增加统一业务校验与渲染，没有复制共享实现。
- 修复：计划/批次对象识别、旧关联迁移、预检先于锁定、推进前复查、独立任务空目标、取消占用与重试权限、AI 过期响应、封面依赖/模板版本/编辑底图一致性、重复确认、零数量类目造成的误判。
- 新增：四步计划向导、同平台多批拆分、批次与计划工作台、多字段 AI 演示区、真实封面渲染、资源引用保护、可重复验收场景和规则回归。
- 按用户补充，本轮所有修改都在 Cloud `web/example/proto/publishing-design/`，包含测试和本证据目录；不新增 CHG/Milestone，不改 Workspace 文档、正式业务源码、共享组件、Agent 或 Desktop。

## 事实源、Git 与本地保护

已读取 `/Users/aqiuye/Downloads/wtmedia-new/Codex_最终执行Prompt.md`、十张完整高保真图、本地最新版 `docs/design/publishing.md` 及两仓规则。业务规则依最终 Prompt、用户裁定和本地文档，图片用作布局参考。

Workspace 初始及最终分支为 `codex/publishing-design-boundaries`；HEAD/缓存 `origin/main` 为 `3ccd7114650e8d8c0fa9cf65814e16805d41cb41`。唯一用户修改 `docs/design/publishing.md` 原样保留，SHA256 始终为 `efd9f6795f8bc039372a94ad849186f50dd86ee2249e7c7b8c0b189fe32a3891`。

Cloud 初始工作树干净，基于 `4358ce5`，工作分支 `codex/publishing-interaction-closure`。两仓 fetch 均在 20 秒超时，未 pull/merge/reset/stash，也未用远端覆盖文档；因此远端最新状态未核实。交付为本地 Git 提交，未推送或创建 PR。提交信息为 `feat(prototype): complete publishing interaction lifecycle and acceptance`；实际 hash 见最终回复或 `git log -1`。

## 逐页实现及实际证据

| 页面 / 工作区 | 实现结果、权限与反馈 | 实际浏览器证据 |
| --- | --- | --- |
| 任务新建 | 单平台单成片、合格账号、独立短标题、平台字段、声明、时间、实际封面；草稿不占用，最终原子创建待执行任务，无计划归属 | XHS [25](25-quick-task-content.jpg)→[26](26-independent-task-waiting.jpg)；B站任务1558 [59](59-bilibili-independent-created.png)，成片1137跨平台复用，双比例修复后 [63](63-bilibili-pure-background-repaired.png) |
| 任务详情 | 固定身份、经批次追溯计划、最终快照、尝试/日志/回执；状态限制启动、核实、原任务重试、取消、放弃释放与复制 | 未知仅核实 [17](17-unknown-task-restricted.jpg)，取消不恢复 [27](27-task-cancel-no-restore.jpg)，同账号阻断 [28](28-queued-account-guard.jpg)，原任务两次尝试 [29](29-safe-retry-two-attempts.jpg) |
| 批次新建 / 配置 | 手动目标或计划继承只读目标共用三阶段；资源分配、软冲突、AI 内容、明确部分创建；创建不启动 | 继承目标 [04](04-plan-before.jpg)；30/28 [20](20-partial-confirm.jpg)；草稿软冲突 [54](54-draft-soft-conflict.png)；手动批次1556待执行 [56](56-manual-batch-created-pending.png) |
| 计划新建 / 编辑 | 盘点→矩阵→按平台拆批→确认骨架；精确分项合计，候选保留、正式目标保护、保存失败保留、重试幂等 | 骨架 [06](06-plan-skeleton.jpg)；影响确认 [41](41-plan-edit-impact.png)、人工值保留 [42](42-candidate-manual-preserved.png)、正式批次目标阻断 [44](44-finalized-batch-edit-block.png)；失败/刷新/再次确认 [52](52-plan-save-failure-input-preserved.png)、[53](53-save-retry-refresh-one-skeleton.png)、[62](62-plan-reconfirm-one-skeleton.png) |
| 批次执行工作台 | 同账号串行、跨账号最多3在途、故障隔离、搜索/状态/分页；暂停后续领取，取消未领取并保留在途和占用 | 未知账号隔离 [15](15-account-isolation-unknown.jpg)；领取竞态取消 [46](46-claimed-cancel-impact.png)→[47](47-cancel-retains-claimed.png)；28/28收口 [21](21-partial-28-closed.jpg)；未来任务 [50](50-future-unresolved-blocks-next.png) |
| 计划执行工作台 | 当前/下一批、首次锁定、分组/结果统计/日志；仅计划调度动作，≤10%自动、>10%人工、零分母人工跳过；审核与公开分离，迟到不回滚 | [12](12-exact-10-auto.jpg)、[13](13-over-10-manual.jpg)→[14](14-schedule-ended-goal-gap.jpg)、[22](22-review-not-public-ended.jpg)、[23](23-late-audit-no-rewind.jpg)、[24](24-platform-category-results.jpg)；取消跳过 [48](48-cancelled-batch-human-skip.png)、零分母 [49](49-zero-results-manual-skip.png) |
| AI 内容工作区 | 全部/勾选/仅缺失、字段/风格、异步进度、失败/过期、显式采用、单字段重生成、仅失败重试；人工编辑保护，封面重新合成与审核 | 等待中人工编辑后保留；失败实际显示原因 [55](55-ai-failed-fields.png)，重试4字段并采用后创建 [56](56-manual-batch-created-pending.png)；版本/乱序/重复采用另有规则回归 |
| 图片库 | 列表管理、搜索/筛选/分页、上传、双用途、启停/编辑、引用历史；被引用禁止删除；损坏/格式/大小与未保存退出反馈 | 上传PNG1200×900、双用途 [33](33-image-upload-dual-use.png)，9条引用且删除禁用 [34](34-image-reference-history.png)，损坏文件 [35](35-image-corrupt-error.png)；无效格式与放弃未保存已操作 |
| 封面模板库 | 文档/标签/版本/比例/引用、复制、默认、启停、保存/另存、重新打开；正式任务保存版本与文档快照 | 模板1032保存v1并重开 [39](39-template-reopened.png)，另存副本1033；升级v2后任务1557成品src哈希不变 [57](57-template-v2-historical-task-unchanged.png)、[browser-results.json](browser-results.json) |
| 模板编辑器 | 复用三栏编辑器；拖动/控制点尺寸、排序/显隐/锁定、精确属性、撤销/重做、动态槽位、底图、长短标题/小图、真实PNG、保存/另存；缺底图/安全区/溢出阻断 | 拖动与尺寸 [38](38-template-drag-resize.png)，保存重开保留坐标 [39](39-template-reopened.png)；长标题阻断 [37](37-title-overflow-block.png)，纯底图预览 [36](36-pure-background-preview.png)，最终双比例成品 [63](63-bilibili-pure-background-repaired.png) |

## 四项重点 P0：操作与结论

| P0 | 实际操作 | 结果及证据 |
| --- | --- | --- |
| 首次合法调度锁 | 未准备首批启动；合法计划启动→暂停；从批次入口明确人工接管；越序/重复启动规则回归 | 预检失败无锁 [43](43-unready-first-no-lock.png)；合法锁暂停后保留 [45](45-first-lock-survives-pause.png)；接管明确确认，首批全取消后人工跳过 [49](49-zero-results-manual-skip.png) |
| 批次取消收口 | 10条首批中领取2条，保留8条未领取；确认取消；分别完成2条已领取；恢复计划并确认跳过 | 弹窗8未领取/2在途 [46](46-claimed-cancel-impact.png)，取消后8取消、2在途 [47](47-cancel-retains-claimed.png)，完成后计划人工跳过仅推进一次 [48](48-cancelled-batch-human-skip.png) |
| 部分创建与目标缺口 | 目标30，实际28候选，渲染审核，明确同意缺2创建，执行28条 | 目标保持30、正式28、任务28/28收口、公开28/30；按0%失败率衔接，没有虚构2条待执行 [20](20-partial-confirm.jpg)→[21](21-partial-28-closed.jpg) |
| 未启动计划安全编辑 | 30/28场景人工修改首候选；目标+1影响确认；资源重预检；尝试修改已有正式任务的另一批目标 | 28候选及人工标题保留 [41](41-plan-edit-impact.png)→[42](42-candidate-manual-preserved.png)，正式批次目标修改被阻断且原计划不变 [44](44-finalized-batch-edit-block.png)；修复零数量类目/排序导致的误判 |

## 其他关键闭环操作

1. 10%首批9公开/1失败自动下一批；20%首批8公开/2失败等待人工，确认后结束，目标差额仍显示。早期10%场景下一批继承错误演示结果已修复，[12](12-exact-10-auto.jpg)只证明首批阈值；到点重验 [51](51-future-due-advances-once.png) 再次证明恰好10%衔接。
2. 未知场景1未知阻断本账号3排队，其他账号6公开；取消只处理未领取，未知以演示核实链接确认公开，计划等待跳过。计划取消不批量取消既有任务。[15](15-account-isolation-unknown.jpg)～[19](19-plan-cancel-impact.jpg)。
3. 失败任务1250原任务重试，尝试1失败和尝试2公开并存，已结束计划不复活。[29](29-safe-retry-two-attempts.jpg)。
4. 未来任务1543设上海时间2026-11-11 00:55；此前处理9条，未来1条仍阻断下一批 [50](50-future-unresolved-blocks-next.png)；UTC16:55:09之后再处理1条才自动推进 [51](51-future-due-advances-once.png)。
5. 保存失败开关下确认计划1554，输入及一骨架保留；关闭开关、重试、刷新，仍一计划一骨架0任务。再次编辑原样确认，仍0/1骨架关系 [52](52-plan-save-failure-input-preserved.png)、[53](53-save-retry-refresh-one-skeleton.png)、[62](62-plan-reconfirm-one-skeleton.png)。
6. 图片双用途、引用禁止删除、损坏PNG与不支持格式、放弃未保存操作均实际验证。引用的是虚构演示数据，没有删除用户文件。
7. 模板图层锁定禁用属性、显隐、排序、拖动至142/530、控制点尺寸969.0687×256.9008、保存重开一致；缺底图保存阻断并撤销恢复，超长标题阻断，另存副本成功。模板升级不改变已创建任务1557的PNG，前后SHA相同，见 [browser-results.json](browser-results.json)。
8. 手动批次与另一计划草稿候选出现软冲突；创建正式任务1557后才产生百家号占用。相同成片1137在B站仍能创建独立任务1558；正式身份不可修改。
9. 1366×900与390×844实际操作。窄屏导航与表单可达、表格在容器内横向滚动，页面document宽度390无整体横向溢出。键盘Enter打开、Tab进入表单、Escape关闭；修复退出动画抢走焦点后，再验可回到入口。两层预览Escape只关闭上层、下层任务详情保留，按钮“查看任务最终内容”恢复active。
10. 最后检查浏览器warn/error日志为 `[]`。原生Chrome中途受锁屏影响，后续使用IAB真实Chromium继续；没有以锁屏为由省略验收。IAB截图和实际DOM/PNG尺寸记录均在本目录。

## PNG 实物与证据有效性

- [纯底图1200×900](rendered-pure-cover-1200x900.png)：从浏览器实际渲染预览读取PNG，已逐图检查。
- [B站1200×900](rendered-bili-1200x900.png)、[B站1600×900](rendered-bili-1600x900.png)：正式待执行任务1558修正后最终预览的实际PNG，naturalWidth/naturalHeight分别验证。最终画面 [63](63-bilibili-pure-background-repaired.png)。
- `11-plan-locked.jpg` 是“预检失败仍锁定”的修复前证据，**不能作为通过证据**，修复后以43/45/49为准。
- `exported-cover-1200x900.png` 是早期Chrome下载实物，证明导出动作和1200×900尺寸；旧底图带文字，不作为视觉通过证据。
- `58-bilibili-dual-ratio.png` 暴露了编辑器底图与最终合成底图不一致：最终错误沿用素材封面。已修复为编辑器底图及引用来源，并修复场景底图的烘焙文字；正式任务1558修改后以63和上述两张PNG为最终证据，58保留用于缺陷追踪。
- 截图证明操作当时的状态；规则回归、实际状态变化、PNG实物和哈希共同构成证据，不把截图当作全部验收。

## D01–D16 覆盖

| 项 | 实现与验证 |
| --- | --- |
| D01 | 上海业务时区、publicAt今日公开、登录资格、未知额度、跨类目联合去重容量；[40](40-inventory-timezone.png)及流量分配回归 |
| D02 | 仅计划→批次→任务；独立任务不计入计划，矩阵分项严格一致；实际计划/批次/独立任务创建 |
| D03 | 一次性骨架、幂等key、保存失败保留及重试/刷新/再次确认；52/53/62及回归 |
| D04 | 兼容候选保留、重预检、正式目标保护；41/42/44 |
| D05 | 先预检再永久锁，显式接管，越序和重复阻断；43/45/49及回归 |
| D06 | 目标/候选/正式/公开/创建缺口分离；20/21的30/28实操 |
| D07 | 下一批派发前重查控制权、游标、就绪和已有尝试；阈值/配置阻断实操及回归 |
| D08 | 同平台唯一、跨平台复用、草稿软冲突、正式原子抢占、取消保留/放弃释放；27/54/56/59及冲突回归 |
| D09 | 同账号串行、最多3在途、登录/未知隔离、平台故障开关、安全重试不上层复活；15/28/29/46实操及回归 |
| D10 | 所有未决包括未来时间都阻断、零分母人工跳过；49/50/51 |
| D11 | 取消未领取、在途安全收尾、未知核实、计划取消仅终止后续调度；16/19/46/47/48 |
| D12 | 配置、调度、平台事实、AI采用和封面审核独立；待执行创建及审核不计公开实操 |
| D13 | candidateId+输入/字段版本、乱序、手改保护、重试及重复采用；55/56实操及回归 |
| D14 | 当前封面签名、版本/各比例文档、编辑器底图一致、统一布局和真实PNG；36/37/39/57/63及回归 |
| D15 | 审核接收只计提交衔接、后到拒绝不倒退、未来到点；22/23/50/51 |
| D16 | 本轮独立操作、测试、缺陷修复、有效/无效证据区分；现提请用户产品验收，无模型自评分 |

## 最终自动化

执行位置为Cloud `web/`，命令原始输出随目录提交：

| 命令 | 实际结果 | 输出 |
| --- | --- | --- |
| `npm run test:publishing-prototype` | 5文件67测试通过：model10、relationships37、content7、libraries6、covers7 | [publishing-tests.txt](publishing-tests.txt) |
| `npm run test:example` | 22测试通过 | [example-tests.txt](example-tests.txt) |
| `npx vitest run --config vite.config.example.js src/shared/ui/resource/overlayFocus.test.js src/shared/ui/resource/overlayStack.test.js src/shared/ui/feedback/index.test.js` | 7测试通过 | [shared-tests.txt](shared-tests.txt) |
| `npm run build:example` | 构建通过，现有大包警告保留 | [example-build.txt](example-build.txt) |
| `git diff --check` | 无空白错误；路径范围仅example | [repository-checks.txt](repository-checks.txt) |

封面单测使用共享文字布局和受控图像输出替身，验证规则与尺寸；不将单元测试当作真实像素输出验收。浏览器PNG另外留存。

## 未完成项及边界

本轮已确认的原型P0核心闭环无已知阻断。产品视觉和体验接受与否待用户验收。未执行真实Cloud/Agent/Desktop发布、真实模型调用、付费接口、真实平台链接核实、生产API/数据库迁移，符合本轮授权边界；百家号横竖精确规格仍标“待核验”，保留原始尺寸，不编造限制。

远端最新提交因fetch超时未核实，未推送/开PR。自动化覆盖的所有竞态和异常组合没有逐个在浏览器穷举；浏览器已实操关键主线、重点P0、故障恢复、资源库、模板版本、多层弹窗及窄屏。测试结果表述仅适用于本地原型，不能推出生产平台可发布。
