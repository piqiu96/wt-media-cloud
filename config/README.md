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
| `prefix` | Key prefix inside the bucket, e.g. `dev/`. The dev tree sets one so a local run cannot overwrite a production object. |
| `presign_ttl` | How long a pre-signed GET stays valid. |
| `region` | May be empty where the provider does not need it. |
| `use_ssl` | `true` for HTTPS. |

## Credentials

`agent.toml` and `douyin.toml` are tracked locally. **`object_storage.toml` is
not**: it is listed in `.gitignore`, and the local tree ships only a value-less
`object_storage.toml.example`. The release template for every credential is
`config_online/credentials/*.toml.tpl`; real values arrive only through the
server-specific variables file.

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
