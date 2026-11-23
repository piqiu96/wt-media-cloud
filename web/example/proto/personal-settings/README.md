# 个人信息与桌面设置原型页面

本目录只保存页面内容，统一导航、顶栏、路由与演示身份由 `../publishing-design/` 工作台维护。

- 个人信息：`/example/operations/account/profile`，从右上角头像进入，不占侧栏菜单。
- 桌面设置：`/example/operations/desktop/settings`，侧栏左下常驻入口，内含下载与存储、本地日志，不属于普通菜单分组。
- 账号全部状态评审：`/example/operations/account/profile?review=account`，通过页头“原型状态”选择，仍展示完整统一工作台。
- `PersonalSettingsPage.vue` 是唯一页面实现；`scenes.js` 保存演示场景；`index.html`、`entry.js`、`review.html` 仅兼容旧链接，不再渲染第二套框架。

比特浏览器和账号资料统一使用ResourceSection详情标题（字号、字重、蓝色标记与间距同源）；本机使用其表单分区标题，保留用户确认的浅蓝账号提示与逐行字段，不维护另一套标题样式。绑定、本机资料、文件及导出均为演示，不连接真实业务服务。系统配置文件放在下载与存储首项，仅展示当前位置；不提供修改、文件选择或切换引用，不列为待接入功能。正式设计工作稿位于 Workspace `docs/design/resource-filter-and-settings.md`。
