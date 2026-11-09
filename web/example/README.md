# 原型与公共组件

本目录是原型代码的唯一维护位置。`common/` 提供公共参考和正式组件组合示例，`proto/` 保存具体业务原型。已确认的浏览器、账号、内容生产及文件管理体验不因迁移重新设计；已有 HTML 保留原有交互和样式，新原型按修改范围复用 `src/shared/ui`、正式主题和公共样式。

## 启动与分享

- 推荐从 Workspace 运行 `python3 scripts/wt.py dev example`，按已有仓库映射启动并挂载当前本地文档。
- 单独在本仓 `web/` 运行 `npm run dev:example`；文档未挂载时明确显示远端入口。需要挂载时通过本机环境变量 `WT_MEDIA_WORKSPACE_ROOT` 指定 Workspace 根目录，不把绝对路径提交到 Git。
- 统一入口 `http://127.0.0.1:8768/example/`，旧 `/example/common/`、`/example/proto/` 路径保留。沿用原域名、端口及评审存储键，不清理浏览器 localStorage；切换浏览器或来源前从原评审页导出意见。
- `npm run build:example` 单独输出 `dist-example/`；`npm run preview:example -- --open /example/` 查看构建结果。旧 HTML/脚本/图片按原路径输出，新 Vue 入口独立打包。
- `npm run test:example` 验证文档读取边界与已有原型模型。Cloud/Desktop 正式构建不包含本目录，不要求 Workspace 存在。

## 设计与引用

设计工作稿只维护在 [Workspace docs/design](https://github.com/piqiu96/wt-media-workspace/blob/main/docs/design/README.md)。PRD、规范、工程设计及 Delivery 仍由原 Owner 维护。同仓使用相对链接，跨仓使用 GitHub 完整链接；本地预览只将已知仓库 main 分支链接解析为本地文档或原型。具体提交和其他分支链接保留原版本语义。

本地文档只读，保存后刷新即可读取未提交修改。普通 GitHub 页面仍显示已推送版本。缺失本地文件会明确报错，不静默显示另一版本。原型不得发起真实上传、删除、合成或发布，也不接入正式登录和 API 代理。

## 公共资源

`common/ComponentDemo.vue` 直接导入正式组件；`common/index.html` 与原 CSS/控件脚本继续支撑已确认 HTML 原型。二者不混用全局样式，旧 HTML 通过独立页面打开。公共组件源码只在 `src/shared/ui/` 维护，本目录不复制一套实现。
