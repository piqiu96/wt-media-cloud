# WT Media Cloud Linux 宝塔部署手册

本手册用于当前 `online` 环境。Cloud 包是版本自包含的只读程序；数据库密码、对象存储密钥和平台凭据只存在于服务器当前版本的私有 `config/`。`pre` 环境未来使用另一套打包产物和 `wt-media/vars/cloud/pre.json`，不要与本次 online 部署混用。

## 1. 下载与校验

1. 在产品 Tag 的 GitHub Actions Run 下载 `cloud-linux-amd64` Artifact，核对 Cloud 组件 Tag 和 Commit。
2. 解开 Artifact ZIP，得到 Cloud tar.gz、Desktop Web tar.gz 和 `SHA256SUMS`。
3. 校验并解压 Cloud 包：

```bash
sha256sum -c SHA256SUMS
tar -xzf wt-media-cloud_<tag>_linux-amd64.tar.gz
cd wt-media-cloud_<tag>_linux-amd64
./deploy/verify-package.sh
cat release-info.json
```

包内 `config/` 是模板状态：固定文件保留 `.toml`，需注入的值使用 `.toml.tpl`。本地开发 `config/`、`config_test/` 和真实凭据不会进入制品。

## 2. 准备 MySQL

复制 `deploy/prepare-database.sql.example`，在 MySQL 管理员会话中替换库名、账号和密码后执行。当前 online 环境建议：

- 数据库：`wt_media_online`
- 账号：`wt_media_online` 或专用 `wt_media_cloud` 账号
- Host：与 Cloud 服务器访问路径一致，同机默认 `127.0.0.1`

Cloud 部署脚本固定 `--create-database=false`，不会替操作者猜测或创建生产库。

## 3. 安装新版本

在解压出的包目录执行：

```bash
sudo ./deploy/install.sh \
  --install-root /www/wt-media-cloud \
  --release <tag> \
  --service-user www
```

产物：

- `/www/wt-media-cloud/releases/<tag>/`：程序、Web、Migration 和部署脚本
- `/www/wt-media-cloud/releases/<tag>/config/`：模板态私有配置
- `/www/wt-media-cloud/releases/<tag>/logs/`
- `/www/wt-media-cloud/releases/<tag>/data/tmp/`
- `/www/wt-media-cloud/current`：仅在迁移验收后切换

安装脚本不会覆盖同名版本，也不会改变 `current`。

## 4. 拉取并渲染 online 变量表

变量对象由部署操作者在本地上传，当前 online 固定使用：

```text
wt-media/vars/cloud/online.json
```

未来预发布环境才使用：

```text
wt-media/vars/cloud/pre.json
```

在服务器执行：

```bash
sudo /www/wt-media-cloud/releases/<tag>/deploy/init-config.sh \
  --config /www/wt-media-cloud/releases/<tag>/config \
  --environment online \
  --variables-url '<有效期内的私有变量表URL>'
```

脚本会下载数据 JSON、渲染 `.toml.tpl`、检查缺失/未知变量和 TOML 语法，并调用包内 `bin/config-check` 使用 Cloud 配置规则校验。全部通过后才原子替换该版本的 `config/`，并写入只含环境与变量表摘要的 `.render-info.json`。任何失败都不能迁移或切换版本。

如使用本地恢复文件，可将 `--variables-url` 换成 `--variables-file <online.json>`。

## 5. 迁移数据库

维护状态下备份 MySQL 和需要保护的对象存储数据后执行：

```bash
cd /www/wt-media-cloud/releases/<tag>
sudo -u www ./deploy/migrate.sh
sudo -u www ./deploy/migrate.sh
```

重复执行应显示 0 个新 applied。迁移失败不得启动新版本。

## 6. 配置宝塔进程

三个进程都以 `www` 用户运行，工作目录都是 `/www/wt-media-cloud/current`。

### Server：宝塔 Go 项目

- 项目目录：`/www/wt-media-cloud/current`
- 启动文件：`/www/wt-media-cloud/current/bin/server`
- 运行用户：`www`
- HTTP 地址：来自 `config/app.toml`，当前建议 `127.0.0.1:8080`

Cloud Server 已提供 Cloud Web 静态文件、Vue history 回退、API 和健康路由。宝塔网站将域名反向代理到该 Go 服务，不需要单独配置 Nginx 静态根目录。

### Scheduler：宝塔进程管理器

- 名称：`wt-media-cloud-scheduler`
- 启动文件：`/www/wt-media-cloud/current/bin/discovery-scheduler`
- 工作目录：`/www/wt-media-cloud/current`
- 运行用户：`www`
- 不配置监听端口

### Worker：宝塔进程管理器

- 名称：`wt-media-cloud-worker`
- 启动文件：`/www/wt-media-cloud/current/bin/discovery-worker`
- 工作目录：`/www/wt-media-cloud/current`
- 运行用户：`www`
- 不配置监听端口

首次部署先启动 Server 并验收，再启动 Scheduler 和 Worker。升级切换前先停止 Scheduler、Worker、Server；切换后先启动 Server，再启动 Worker 和 Scheduler。

## 7. 切换与验收

确认配置和迁移通过后切换：

```bash
sudo ./deploy/activate.sh --install-root /www/wt-media-cloud --to <tag>
```

启动 Server 后执行：

```bash
sudo /www/wt-media-cloud/current/deploy/verify-runtime.sh \
  --base-url http://127.0.0.1:8080 \
  --login-admin
```

预期：

- `/healthz` 正常
- `/api/v1/health` 正常
- `/` 返回 Cloud Web
- `/login` 返回 Cloud Web
- 未知 `/api/...` 保持 API 404，不回退成 HTML
- `admin / admin123` 登录成功

再按变量表中的 MySQL 值执行数据库验收，确认 `schema_migrations` 数量与包内 SQL 数一致且 `admin` 为 enabled。随后从外部验证 `https://wt.longyanyue.cn/`、`/login` 和 `/api/v1/health`。

## 8. 回退

1. 停止 Scheduler、Worker、Server。
2. 数据库或外部数据不兼容时，不能只切程序；保持维护模式并按备份恢复。
3. 仅兼容回退才执行：

```bash
sudo /www/wt-media-cloud/releases/<old-tag>/deploy/rollback.sh \
  --install-root /www/wt-media-cloud --to <old-tag>
```

4. 从 `current` 重新启动旧版本三进程并重复验收。

每个旧版本保留自己的配置和日志。密钥轮换后，要同步更新仍具备回退资格的旧版本配置，或取消其回退资格。

## 9. 边界

本次 online 部署不证明 BitBrowser、真实业务发布、对象存储生产写入或正式稳定版全量验收通过。
