# WT Media Cloud Linux 部署手册

本手册面向宝塔/Linux 服务器预发布部署。Cloud 包是只读程序；数据库密码、对象存储密钥和平台凭据只写入服务器 `shared/config`。本次仅演练预发布，不把 RC 包公开为正式版。

## 1. 下载与校验

1. 在对应产品 Tag 的 GitHub Actions Run 下载 `cloud-linux-amd64` Artifact，核对 Run 的 Cloud 组件 Tag 和 Commit。
2. 解开 Artifact ZIP，得到 Cloud tar.gz、Desktop Web tar.gz 和 `SHA256SUMS`。
3. 校验并解压 Cloud 包：

```bash
sha256sum -c SHA256SUMS
tar -xzf wt-media-cloud_<tag>_linux-amd64.tar.gz
cd wt-media-cloud_<tag>_linux-amd64
./deploy/verify-package.sh
cat release-info.json
```

## 2. 创建数据库与专用账号

复制 `deploy/prepare-database.sql.example`，在服务器上替换库名、账号和密码，然后以 MySQL 管理员执行。默认值为：

- 数据库：`wt_media_cloud`
- 账号：`wt_media_cloud`@`127.0.0.1`，与 Cloud 配置的 TCP 地址一致
- Cloud 应用密码：由你设置，不写入 Git

如需远程部署，请同时调整 SQL 的账号 Host 和 `init-config.sh --db-host`。

已有部署升级前，先在宝塔显示维护提示，停止 Server、Scheduler、Worker，并分别备份 MySQL 与对象存储；记录备份位置和恢复步骤。迁移前在数据副本演练，因为部分 MySQL DDL 无法通过切换程序版本撤销。

## 3. 安装固定版本

默认安装到 `/www/wt-media-cloud`：

```bash
sudo ./deploy/install.sh --install-root /www/wt-media-cloud --release <tag> --service-user www
```

安装产物：

- `/www/wt-media-cloud/releases/<tag>/`：本次程序
- `/www/wt-media-cloud/shared/config/`：私有配置
- `/www/wt-media-cloud/shared/logs/`：日志
- `/www/wt-media-cloud/shared/data/tmp/`：媒体处理临时文件，三个进程共用稳定的 `TMPDIR`；按服务器容量定期清理
- `/www/wt-media-cloud/current`：验收后才指向新版本的软链

## 4. 生成服务器配置

首次部署执行；脚本会在终端提示输入第 2 步的应用账号密码：

```bash
sudo /www/wt-media-cloud/releases/<tag>/deploy/init-config.sh \
  --config /www/wt-media-cloud/shared/config \
  --db-host 127.0.0.1 --db-port 3306 \
  --db-name wt_media_cloud --db-user wt_media_cloud \
  --admin-username admin \
  --owner www
```

`init-config.sh` 只接受空目录，避免覆盖已有私有配置。第二次终端提示输入已裁定的 RC 初始管理员密码。生成后检查：

- `database/primary.toml`
- `credentials/agent.toml`
- `credentials/douyin.toml`
- 如使用对象存储，复制 `credentials/object_storage.toml.example` 为 `object_storage.toml` 并填入密钥
- 检查 `storage/object_storage.toml` 的 `bucket` 和 `prefix`；RC 模板默认 `staging/` 前缀，正式环境须单独指定，避免和开发、生产文件混用

## 5. 迁移数据库

必须先完成数据库备份。然后在维护窗口执行：

```bash
cd /www/wt-media-cloud/releases/<tag>
# deploy/migrate.sh 内部固定执行 bin/migrate -dir migrations --create-database=false
sudo -u www ./deploy/migrate.sh
sudo -u www ./deploy/migrate.sh
```

重复执行 `migrate.sh` 应报告 0 个新 applied；任何迁移失败都不得启动任何新进程。

迁移成功并核对目标库后，切换当前版本。升级时先确认三个旧进程已停止：

```bash
sudo ./deploy/activate.sh --install-root /www/wt-media-cloud --to <tag>
```

> `schema_migrations` 只证明数据库结构已应用；`admin` 是 Server 首次启动时按配置初始化的，所以在第 6 步先启动并验证 Server，再执行数据库验收。

## 6. 启动三个进程

systemd 模板在 `deploy/systemd/`。按服务器实际安装路径检查后复制：

```bash
sudo cp deploy/systemd/*.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now wt-media-cloud-server.service
```

先完成第 7 节的健康、登录和数据库验收，再启动后台进程：

```bash
sudo systemctl enable --now wt-media-cloud-scheduler.service
sudo systemctl enable --now wt-media-cloud-worker.service
```

Server 默认监听 `127.0.0.1:8080`。宝塔反向代理将 HTTPS 域名转发到该地址，不要把 3306 或 8080 直接暴露公网。

## 7. 验收

```bash
sudo /www/wt-media-cloud/current/deploy/verify-runtime.sh \
  --base-url http://127.0.0.1:8080 --login-admin
```

预期：`runtime verified: health=ok login=admin`。再执行：

```bash
sudo ./deploy/verify-database.sh --database wt_media_cloud --user wt_media_cloud
```

预期：`database verified: migrations=50 initial_admin=admin/enabled`。最后用浏览器通过 HTTPS 域名打开 Cloud Web，用已裁定的初始管理员密码登录，并立即修改密码。

## 8. 回退

1. 停止 Scheduler、Worker、Server。
2. 若迁移不兼容或数据异常，不要只切程序；保持维护模式并按备份恢复 MySQL/对象存储。
3. 兼容回退才可执行：

```bash
sudo /www/wt-media-cloud/releases/<old-tag>/deploy/rollback.sh \
  --install-root /www/wt-media-cloud --to <old-tag>
```

## 9. 边界

本预发布验收不证明 BitBrowser、真实业务发布、对象存储生产写入或正式稳定版上线通过。
