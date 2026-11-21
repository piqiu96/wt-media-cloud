# Configuration

Runtime code always reads `./config`. `config_online/` is the release template
source, not a second runtime source. Its runtime paths must correspond one-to-one
with this directory: fixed files remain `.toml`, while environment values become
`{{VARIABLE}}` placeholders in `.toml.tpl`. Packaging copies `config_online/` to
the artifact as template-state `config/`; deployment renders it, validates it
with `bin/config-check`, and only then replaces that release's private `config/`.
`internal/config` loads each runtime file by an explicit path — nothing here is
discovered by scanning — so a new file has to be added to `LoadFromDir` as well
as to both trees.

## Object storage

`storage/object_storage.toml` names the bucket that prepared source objects live
in. It is required in both trees.

| Key | Meaning |
| --- | --- |
| `endpoint` | Host only, no scheme and no bucket label. `use_ssl` chooses the scheme. |
| `bucket` | Bucket name. |
| `prefix` | Prefix for new object locations; empty for the unified media directories. Stored s3 locations already contain the complete key and ignore later prefix changes. |
| `legacy_bucket` / `legacy_prefix` | Explicit home of old unqualified references during migration. Keep these until all available old references have been verified and switched. |
| `presign_ttl` | How long a pre-signed GET stays valid. |
| `region` | May be empty where the provider does not need it. |
| `use_ssl` | `true` for HTTPS. |

## Credentials

`agent.toml`, `agent_token.toml` and `douyin.toml` are tracked locally.
`agent_token.toml` holds the HMAC key for stateless Local Agent tokens: the
tracked value is a development key, deploy renders the real one from
`WT_MEDIA_AGENT_TOKEN_SECRET`, and the server refuses to start when the key is
missing or shorter than 32 characters. **`object_storage.toml` is not**: it is listed in `.gitignore`, and
the local tree ships only a value-less `object_storage.toml.example`. The release
template for every credential is `config_online/credentials/*.toml.tpl`; real
values arrive only through the server-specific variables file.

This is a deliberate difference from the other two, which were confirmed
tracked under the stage policy recorded as A-03 in
`docs/arch/2026-09-20-cloud-architecture-baseline.md`. The reason for the
exception is that the object-storage key pair is a long-lived credential that
authorises writes to every prepared source object, and that policy already
records the cost of tracking one — the Douyin values in this repository's
history have to be rotated before release. The key pair is supplied by whoever
runs the acceptance instead of being committed, so there is nothing to rotate
out of the tree later.

`object_storage.toml` is optional at load time. Absent, Cloud starts normally and
every object-storage call answers `ErrNotConfigured`; that is what keeps a
fresh checkout and `go test ./...` working without a secret. A file with only one
of the two values is a startup error — half a pair is an unfinished edit, and it
would otherwise appear as an authentication failure on the first request.

To use the remote endpoint locally, copy the example next to it as
`object_storage.toml` and fill in `access_key` and `secret_key`. Release builds
do not read that local file.

`bucket` currently reads `REPLACE_WITH_BUCKET` in both trees: the endpoint was
confirmed but the bucket name was not, and a sentinel keeps the tree loadable
while making the missing value fail loudly on the first call rather than as an
access-denied that reads like a permissions problem.

### 运营文件公共域名

`storage/object_storage.toml` 的 `files_public_base_url` 配置当前bucket对应的公共HTTPS origin（不含路径、查询参数或凭据）。仅运营文件API用它和持久化完整对象位置生成 `public_url`；不拼当前全局prefix，不改变素材/成片的授权规则。部署前需确认该域名能匿名读取运营文件；配置不会修改bucket ACL。旧 `resource_path` 保留兼容与附件下载。

## AI 文案（DeepSeek）

`app.toml` 的 `[publication_generation]` 配置模型、HTTPS `base_url` 和单次调用 `timeout`；两套配置树保持同构。M6 使用 `deepseek-flash`，非思考模式生成完整 JSON，单次最多2048 tokens，不自动重试。调用取消沿生成连接传播，超时不超过180秒。此客户端在 `internal/infra/client/deepseek/`，使用支持进行中取消的标准 HTTP 客户端，由HTTP进程统一装配；Scheduler/Worker不加载它。

本机从 `config/credentials/deepseek.toml` 读取 `api_key`，文件已忽略提交，建议权限0600；模板为同目录 `.example`。上线从 `WT_MEDIA_DEEPSEEK_API_KEY` 渲染凭据模板。缺少密钥仍可启动，生成接口明确拒绝。提示词、文案、密钥和模型原始错误正文不写日志；请求只带经授权素材标题与当前文字，不发送视频或平台登录信息。
