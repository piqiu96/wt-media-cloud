# WT Media Cloud Linux 宝塔部署手册

部署统一由 `bin/wtmctl` 完成。`wtmctl` 不启动、停止或重启任何 Cloud 进程；Server、Worker、Scheduler 仍由宝塔管理。

## 1. 目录约定

```text
/home/www/wt-media-cloud/
├── output/
│   ├── online.toml
│   ├── online.url
│   ├── online-deploy.toml
│   └── wt-media-cloud_<product-tag>_linux-amd64/
├── releases/
│   ├── v0.1.0-rc.12/
│   └── v0.1.0-rc.13/
└── current -> releases/v0.1.0-rc.13
```

`<product-tag>` 是本产品 RC 的 Tag（例如 `v0.1.0-rc.13`）。

## 2. TOML 变量

线上变量对象：

```text
wt-media/vars/cloud/online.toml
```

预发布变量对象：

```text
wt-media/vars/cloud/pre.toml
```

变量示例见：

```text
deploy/examples/online.toml.example
```

服务器只需保存预签名 URL 文件：

```text
/home/www/wt-media-cloud/output/online.url
```

`wtmctl deploy apply` 会自动远程拉取、校验和落地变量。

## 3. 部署 profile

复制并修改：

```text
deploy/examples/online-deploy.toml.example
```

到：

```text
/home/www/wt-media-cloud/output/online-deploy.toml
```

关键字段：

```toml
schema_version = 1

[deploy]
install_root = "/home/www/wt-media-cloud"
output_dir = "/home/www/wt-media-cloud/output"
environment = "online"
service_user = "www"
process_manager = "baota"
variables_url_file = "/home/www/wt-media-cloud/output/online.url"
variables_file = "/home/www/wt-media-cloud/output/online.toml"
health_url = "http://127.0.0.1:8188"
```

`package_root` 和 `release` **不用填**：在解压包内执行 `bin/wtmctl` 时，它会从 `release-info.json` 和自身所在的 `bin/` 目录推导。只有覆盖默认位置时才需要显式设置，且必须与包一致。

## 4. 部署预检

进入解压后的 Cloud 包：

```bash
cd /home/www/wt-media-cloud/output/wt-media-cloud_<product-tag>_linux-amd64
```

校验 Artifact：

```bash
./bin/wtmctl artifact verify \
  --profile /home/www/wt-media-cloud/output/online-deploy.toml
```

检查部署环境：

```bash
./bin/wtmctl doctor \
  --profile /home/www/wt-media-cloud/output/online-deploy.toml
```

部署预演：

```bash
./bin/wtmctl deploy plan \
  --profile /home/www/wt-media-cloud/output/online-deploy.toml
```

预演会远程拉取变量到临时文件并完成完整性、配置和包检查，不修改 `current`。

## 5. 一键部署

先由宝塔停止旧版本的 Server、Worker、Scheduler。

然后执行：

```bash
sudo ./bin/wtmctl deploy apply \
  --profile /home/www/wt-media-cloud/output/online-deploy.toml
```

`deploy apply` 依次执行：

1. 校验 Artifact 和变量 Schema。
2. 远程拉取 `online.toml`。
3. 检查 11 个变量是否完整。
4. 安装到 `releases/<tag>`。
5. 渲染并校验私有配置。
6. 执行 Migration。
7. 验证数据库结构。
8. 原子切换 `current`。
9. 输出需要由宝塔重启的进程。

它不会启动或停止任何进程。

## 6. 宝塔进程配置

三个进程的路径都指向 `current`，只需配置一次。

### 路径解析与环境变量

三个进程不再依赖启动时的工作目录。程序按以下顺序解析 Cloud 根目录（`ENV_PATH`）：

1. `WT_MEDIA_CLOUD_HOME`；
2. 可执行文件所在目录的上级（`<home>/bin/<二进制>`）；
3. 当前工作目录（本地开发）。

各子路径默认由根目录派生，并可单独覆盖：

| 用途 | 环境变量 | 默认值 |
| --- | --- | --- |
| Cloud 根目录（ENV_PATH） | `WT_MEDIA_CLOUD_HOME` | 可执行文件上级目录 |
| 配置目录 | `WT_MEDIA_CLOUD_CONFIG_PATH` | `<home>/config` |
| 日志目录 | `WT_MEDIA_CLOUD_LOG_PATH` | `<home>/logs` |
| Web 目录 | `WT_MEDIA_CLOUD_WEB_PATH` | `<home>/web` |

日志配置里的相对 `path`（例如 `app.log`）会被锚定到日志目录。因此宝塔即使从 `<home>/bin` 启动，也能找到 `config/`、`web/`、`logs/`；迁移安装目录时只需改 `WT_MEDIA_CLOUD_HOME`，或保持 `<home>/bin/<二进制>` 结构、什么都不用设。

### Server：宝塔 Go 项目


```text
项目名称：wt-media-cloud-server
项目目录：/home/www/wt-media-cloud/current
启动文件：/home/www/wt-media-cloud/current/bin/wt-media-cloud
运行用户：www
监听地址：127.0.0.1:8188
```

### Worker：宝塔进程管理器

```text
名称：wt-media-cloud-worker
工作目录：/home/www/wt-media-cloud/current
启动文件：/home/www/wt-media-cloud/current/bin/discovery-worker
运行用户：www
端口：不填写
```

### Scheduler：宝塔进程管理器

```text
名称：wt-media-cloud-scheduler
工作目录：/home/www/wt-media-cloud/current
启动文件：/home/www/wt-media-cloud/current/bin/discovery-scheduler
运行用户：www
端口：不填写
```

启动顺序：

```text
Server -> Worker -> Scheduler
```

## 7. 部署后验收

```bash
sudo /home/www/wt-media-cloud/current/bin/wtmctl deploy verify \
  --profile /home/www/wt-media-cloud/output/online-deploy.toml
```

检查：

- `current` 指向；
- Migration 数量；
- `admin/admin123` 登录；
- `/healthz`、`/api/v1/health`；
- `/`、`/login`；
- 未知 `/api/` 返回 404；
- 宝塔三个进程正在运行。

外部 HTTPS：

```bash
curl -I https://wt.longyanyue.cn/
curl -I https://wt.longyanyue.cn/login
curl -i https://wt.longyanyue.cn/api/v1/health
```

## 8. 回退

先由宝塔停止三个进程。

确认应用与数据库兼容后执行：

```bash
sudo /home/www/wt-media-cloud/current/bin/wtmctl release rollback \
  --profile /home/www/wt-media-cloud/output/online-deploy.toml \
  --to <old-tag> \
  --services-stopped
```

迁移不兼容时不能只切换软链，必须恢复 MySQL 和对象存储的一致快照。
