# 安装与交付

本项目由 DeepFal 维护，源码仓库为 <https://github.com/DeepFal/icloud-mail>。当前源码元数据版本为 `0.2.9`；GHCR 镜像由推送对应 `v*` tag 后的工作流生成，首次发布完成前请使用源码构建。本文件中的配置不表示镜像、正式 Release 或离线包已经发布。来源与许可见 [NOTICE.md](NOTICE.md) 和 [LICENSE](LICENSE)。

## 源码构建安装（推荐）

需要 Docker Engine、Docker Compose v2 和 Git，建议至少 2 核 / 4 GB 内存，并保持单个应用实例。构建阶段需要访问 Docker 基础镜像、Go 模块和 npm 依赖源。

```sh
git clone https://github.com/DeepFal/icloud-mail.git
cd icloud-mail
cp -n .env.example .env
docker compose -f compose.yaml -f compose.build.yaml up -d --build --wait
docker compose -f compose.yaml -f compose.build.yaml ps
curl -fsS http://127.0.0.1:8788/healthz
```

示例配置将 HTTP 发布到 `127.0.0.1:8788`、IMAPS 发布到 `127.0.0.1:1993`。运行时仍需访问 Apple/iCloud 或配置的上游 IMAP 服务。

## GHCR 镜像

工作流仅在推送 `v*` tag 时构建并发布两个镜像，支持 `linux/amd64` 和 `linux/arm64`：

| Docker target | 镜像 | 版本示例 |
| --- | --- | --- |
| `runtime` | `ghcr.io/deepfal/icloud-mail` | `v0.2.9` |
| `postgres-runtime` | `ghcr.io/deepfal/icloud-mail-postgres` | `v0.2.9` |

每个镜像同时提供版本 tag、`sha-<7位commit>` 和 `latest`；`latest` 只在 tag 发布时更新，安装使用固定版本。tag 必须与源码 `VERSION` 一致，例如 `VERSION=0.2.9` 对应 `v0.2.9`。

确认目标版本的两个包均已公开后，在克隆的仓库目录中安装：

```sh
cp -n .env.example .env
# .env 中设置 ICLOUD_MAIL_VERSION=v0.2.9（或其他已发布版本）
docker compose pull
docker compose up -d --wait
docker compose ps
```

基础、源码构建和离线编排均使用 `ICLOUD_MAIL_VERSION`，默认 `v0.2.9`；`compose.public.yaml` 继承基础镜像设置，只增加证书挂载。基础镜像安装保留 `ICLOUD_API_IMAGE` 和 `ICLOUD_API_POSTGRES_IMAGE` 作为完整镜像引用或 digest 的可选覆盖项；它们优先于版本变量。旧 `.env` 中如有 `:latest` 或其他镜像覆盖项，应移除或改为目标固定版本，否则仅修改 `ICLOUD_MAIL_VERSION` 不会生效。

镜像携带 `org.opencontainers.image.revision`（完整源码 commit）、`org.opencontainers.image.version`（如 `v0.2.9`）和 `org.opencontainers.image.source`（本仓库 URL）。拉取后可核对：

```sh
docker image inspect ghcr.io/deepfal/icloud-mail:v0.2.9 \
  --format '{{json .Config.Labels}}'
docker image inspect ghcr.io/deepfal/icloud-mail-postgres:v0.2.9 \
  --format '{{json .Config.Labels}}'
git rev-parse 'v0.2.9^{commit}'
```

两个镜像的 `revision` 应与 tag 指向的 commit 相同，`source` 应为 `https://github.com/DeepFal/icloud-mail`。

## 首次登录

首次空安装会生成管理员密码、OAuth Token、主密钥、管理路径和 IMAPS 自签证书，保存在 `icloud_api_keys` 卷中：

```sh
docker compose -f compose.yaml -f compose.build.yaml exec -T icloud-api cat /app/keys/admin-path
docker compose -f compose.yaml -f compose.build.yaml exec -T icloud-api cat /app/keys/admin-password
```

用户名为 `admin`；使用 `.env.example` 时入口为 `http://127.0.0.1:8788/admin/`。首次接入请在主号管理中添加 Apple 账号并完成连接。直连 iCloud IMAP 使用 Apple App 专用密码，第三方 IMAP 使用该服务自己的凭据。

远程服务器可通过 SSH 隧道访问：

```sh
ssh -N -L 127.0.0.1:8788:127.0.0.1:8788 USER@SERVER
```

公网 HTTPS、域名和证书见 [公网部署](docs/PUBLIC.md)。

## 数据备份与更新

数据库、keys、installation state 和 mail archive 应形成同一个备份点。更新前先运行 `scripts/backup-local.sh`，保留 `.env`、Compose 项目身份与已有 Docker volumes，不执行带 `-v` 的清理。备份和更新使用当前安装的同一组 Compose 文件（公网证书覆盖文件也需保留）。

**镜像安装升级 = 修改 `.env` 中的版本号 + `docker compose pull` + `docker compose up -d`。** 先查看目标版本的发布说明并备份，再将 `ICLOUD_MAIL_VERSION` 改为已发布的目标版本（如未来的 `v0.2.10`）：

```sh
sh scripts/backup-local.sh
# 编辑 .env 中的 ICLOUD_MAIL_VERSION；移除或同步修改完整镜像覆盖项
docker compose pull
docker compose up -d
docker compose ps
curl -fsS http://127.0.0.1:8788/healthz
```

`pull` 成功后再执行 `up`，使应用与 PostgreSQL 使用同一发布版本。回滚镜像时也需明确指定版本或各自的 digest；涉及数据库迁移的回滚必须同时恢复匹配的备份，不能只降级镜像。

源码构建安装需先更新到目标源码并核对 `VERSION`，同步 `.env` 中的 `ICLOUD_MAIL_VERSION` 后重新构建：

```sh
COMPOSE_FILE=compose.yaml:compose.build.yaml sh scripts/backup-local.sh
git pull --ff-only
# 核对目标源码的 VERSION，并同步 .env 中的 ICLOUD_MAIL_VERSION
docker compose -f compose.yaml -f compose.build.yaml up -d --build --wait
```

离线安装需先 `docker load -i <目标版本镜像归档>`，同步 `.env` 的版本后运行 `docker compose -f compose.yaml -f compose.offline.yaml up -d --no-build --pull never --wait`；离线模式不执行 `pull`。

完整兼容说明见 [docs/REFERENCE.md](docs/REFERENCE.md)。不要将正在使用的数据库、密钥、证书或备份提交到 GitHub。

## 维护者打包

推送 tag 后，先确认发布工作流的两个镜像都成功。制作 `linux/amd64` 离线包时，拉取对应平台的 `ghcr.io/deepfal/icloud-mail:TAG` 和 `ghcr.io/deepfal/icloud-mail-postgres:TAG`，再使用 `bash scripts/package-release.sh TAG` 生成源码归档、镜像归档和外部 `SHA256SUMS`。也可从该 tag 的干净源码构建 `runtime` 和 `postgres-runtime` 两个 Docker target，使用上述镜像名并设置 OCI `version`、完整源码 `revision` 与 `source=https://github.com/DeepFal/icloud-mail`。打包脚本会核对版本、commit、来源及离线编排的默认版本，只归档标签中的文件，不使用运行容器制作镜像。

首次发布时，维护者需允许仓库 Actions 使用 `GITHUB_TOKEN` 写入 Packages，确认两个 GHCR 包均为 public 并允许本仓库 Actions 访问；工作流使用 `contents: read` 和 `packages: write`，不需要个人访问令牌。发布前检查 tag、`VERSION`、Compose 默认版本及 `.env.example` 一致。`latest` 只表示最近一次 tag 发布的产物，部署与回滚均应指定版本或 digest。

正式 Release、镜像或离线包生成后，应以对应 CI 产物和仓库发布说明为准。
