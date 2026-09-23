# 内置的 TDesign 图标精灵

本目录存放**原样内置**的 TDesign 图标精灵文件，供桌面端与 Web 端同源加载。

## 为什么要内置

`tdesign-icons-vue-next@0.4.6` 的 `t-icon` 走的是 **svg-sprite** 分支
（`esm/index.js:5` → `esm/svg-sprite/index.js`）。它渲染的是

```html
<svg class="t-icon t-icon-<name>"><use href="#t-icon-<name>"></use></svg>
```

符号本身不在 JS 里，而是由 `esm/svg-sprite/svg-sprite.js:15` 的常量

```
CDN_ICONFONT_URL = "https://tdesign.gtimg.com/icon/0.4.3/fonts/index.js"
```

在组件 `onMounted` 时通过**注入一个远程 `<script>`** 拉回来
（同文件 `:29-41`，`checkScriptAndLoad`）。`props/props.js` 里 `loadDefaultIcons` 默认 `true`，
而 `esm/global-config.js` 是**空文件**——包内没有任何全局入口可以改掉这个 URL。

桌面端 `tauri.conf.json` 的 CSP 没有 `script-src`，回落到 `default-src 'self'`，
因此这个远程脚本被 WKWebView 拒绝，`#t-icon-*` 符号从未定义，
每个 `<use>` 都解析为空 —— **桌面端所有 TDesign 图标整片空白**。
Web 端没有 CSP，所以照常显示。

内置这份文件并**同源**加载后，符号在任何 `t-icon` 挂载前就已存在；
无需放松 CSP，也无需改动任何 `<t-icon>` 用法。桌面端因此**离线也能显示图标**。

影响面不止侧栏菜单：TDesign 内部的 `t-icon`（弹窗/抽屉关闭按钮、分页箭头、表格排序箭头、
子菜单展开箭头等）走的是同一条路径，所以只能靠补齐精灵来解决，
逐个改用内联图标组件覆盖不到这些内部图标。

## 来源与校验

| 项 | 值 |
| --- | --- |
| 来源 | `https://tdesign.gtimg.com/icon/0.4.3/fonts/index.js` |
| 对应包 | `tdesign-icons-vue-next@0.4.6`（该包内硬编码的版本号是 `0.4.3`） |
| 大小 | 978,202 字节 |
| SHA256 | `8e63bb1e65e0bca2a65c2c30af53bf5760b3c444df161e114a18a4538f7eedae` |
| symbol 数 | 2352 |

文件首行自称由 `useSvgSpriteTemplate.ts` 自动生成，**不要手工编辑**。

## 安全自查（内置第三方代码前做过）

- 无 `eval` / `new Function` / `fetch` / `XMLHttpRequest` / `WebSocket` / `importScripts`
- 不读 `document.cookie`、不碰 `localStorage`
- 脚本主体只有一句：`document.body.insertAdjacentHTML('afterbegin', svgCode)`，
  且 `svgCode` 是**静态字面量**（无插值），不会引入注入面
- 文件内出现的 `http://` 全部是 SVG 命名空间声明（`xmlns`），不是网络请求；`https://` 出现 0 次

## 如何更新

1. 取回对应版本的 `https://tdesign.gtimg.com/icon/<版本>/fonts/index.js`
2. 放到 `public/tdesign-icons/<版本>/index.js`
3. 更新本文件的「来源与校验」表，并同步 `index.*.html` 里引用的版本路径
