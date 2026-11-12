# 发布计划 V1 原型

来源：用户提供的《发布计划 V1 高保真原型全套》，本轮保留其页面与交互设计，复用 `example/common/example.css` 和 `controls.js` 对齐现有视觉与交互。所有数据均为本地演示，不调用发布、资源或 AI 服务。

- 入口：`/example/proto/publishing-design/plan/`；现有 `/example/?view=publishing#plans` 与发布管理导航跳转到此。
- 截图：[逐项走查图册](evidence/index.html)，附真实浏览器操作与校验结果 `evidence/browser-results.json`。
- 验证：在 `web/` 运行 `npm run test:publishing-plan`、`npm run test:publishing-prototype`、`npm run test:example`、`npm run build:example`。
- 独立保存键：`wt-publishing-plan-v1`。刷新恢复列表上下文与未完成表单；不读写旧任务、批次原型数据。损坏存档不会被静默覆盖。
- 发布候选仅 B站、百家号、小红书，与现有系统一致；用户输入稿中的抖音资源不作为当前可发布能力。
- 锁定批次保持完整快照和顺序；原始批次分解原目标；补量单独记录且受每个矩阵单元格缺口约束。每批由人工确认执行，不自动启动后续批次。
- 执行结果只有显式回填链接才增加成功数；重试和未知结果持续阻止结束。批次抽屉的「演示反馈控制」只用于模拟结果收敛，不构造成功记录。
- 取消先阻止后续批次；存在在途或未决任务时保留“停止中”。最终是否需要该状态，仍须结合执行器同步停止能力确认。本地演示不证明真实停止或真实作品发布。

Delivery 与验收事实只维护在 Workspace `CHG-20261113-101`，此文件不维护第二套交付状态。
