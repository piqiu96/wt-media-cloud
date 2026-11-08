# 业务原型

> 本目录已迁入 Cloud。新原型参考 `common/ComponentDemo.vue`，直接使用 `src/shared/ui/` 和正式主题；以下 HTML 结构及 starter 继续服务已有原型维护。设计工作稿只维护在 Workspace `docs/design/`。

此目录保存具体业务流程的可操作HTML原型。公共视觉与交互统一参考 [common](../common/README.md)，规则引用 [frontend正式规范](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/standards/frontend/README.md)。

[打开起始页](starter.html)：一个精简资源信息区与新建Dialog，共用 `../common/example.css` 、`../common/controls.js`和 `../common/example.js`。复制到业务主题子目录时，先把公共资源路径调整为 `../../common/`，再替换标题、业务字段与场景。

## 组织方式

```text
example/
  common/                 公共参考与资源
  proto/
    starter.html          原型起始模板
    <业务主题>/
      index.html          业务原型入口
      README.md           关联正式需求、角色、场景入口与演示边界
      prototype.js        该业务特有的本地演示（确有需要时）
```

业务原型组合公共结构、扩展实际需求场景；不要每个原型重新维护颜色、基础控件和浮层。发现可复用的新模式时，先提出规范修订和公共影响，获批后写回common。业务特有权限、字段和异常语义应引用所属正式需求，不写进common。

每个仍在探索的业务主题可在模块内 `design/` 保留一份当前合并工作稿，记录尚未定稿的产品逻辑和已确认设计决策；不保存聊天全文、逐轮副本或截图堆积。里程碑结束时合并清理，验收后稳定事实回归正式 PRD/规范，工作稿按治理规则归档或移除。

所有演示数据明确标识，不填真实凭据，不连接业务服务，不将模拟结果当验收证据。starter只是起始模板；完整业务原型以对应主题的index.html为唯一当前入口。原型不等于真实业务已经完成落地。

[浏览器窗口与代理完整原型](browser-proxy/index.html)包含75个有效场景，按当前公共规范适配。只保留最终稿与必要资源，遵循[example约定](../README.md#当前稿保留)。

[内容运营完整原型](content-operations/index.html)已统一内容生产、发布管理、作品管理、互动管理的视觉、导航和演示数据。场景与操作方式见[原型说明](content-operations/README.md)；[逐页评审](content-operations/review.html#page-review)支持逐页结论、补充意见、本机保存与导出／导入交接。目前第 06 页新增合成任务及素材选择器已接入工作台，按“本页方案确认 → 实施 → 体验确认 → 下一页”的节奏细化。

[社媒账号完整原型](social-accounts/index.html)为已确认 V7，重构与走查统一引用此入口。

内容生产与发布互动的旧兼容目录已移除；当前原型、资源与评审入口统一维护在 `content-operations/`。

[内容挖掘评审原型](content-mining/index.html)覆盖内容池、挖掘策略与挖掘任务；[现状与设计分析](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/design/content-mining.md)记录当前源码、PRD 差异和设计建议，尚待用户评审。
