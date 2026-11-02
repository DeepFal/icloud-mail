# 安装与交付

本项目由 DeepFal 维护，源码仓库为 <https://github.com/DeepFal/icloud-mail>。当前源码元数据版本为 `0.2.9`；本仓库暂未发布正式 Release、离线包或可供用户依赖的 GHCR 镜像。来源与许可见 [NOTICE.md](NOTICE.md) 和 [LICENSE](LICENSE)。

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

仓库的 CI 在首次成功构建并推送后，才会生成对应的 `ghcr.io/deepfal/icloud-mail` 与 PostgreSQL 镜像。届时可在 `.env` 中用 `ICLOUD_API_IMAGE` 和 `ICLOUD_API_POSTGRES_IMAGE` 指定镜像或固定 digest，再按仓库 CI 产物提供的编排方式部署。本文件不承诺镜像已存在，也不承诺当前已有 Release 或离线包。

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

数据库、keys、installation state 和 mail archive 应形成同一个备份点。更新前先运行 `scripts/backup-local.sh`，保留 `.env` 与已有 Docker volumes，不执行带 `-v` 的清理：

```sh
COMPOSE_FILE=compose.yaml:compose.build.yaml sh scripts/backup-local.sh
git pull --ff-only
docker compose -f compose.yaml -f compose.build.yaml up -d --build --wait
```

完整兼容说明见 [docs/REFERENCE.md](docs/REFERENCE.md)。不要将正在使用的数据库、密钥、证书或备份提交到 GitHub。

## 维护者打包

发布前从干净源码构建 `runtime` 和 `postgres-runtime` 两个 Docker target，分别标记为 `icloud-api:TAG`、`icloud-api-postgres:TAG`，并设置 OCI `version` 与完整源码 `revision`。使用 `bash scripts/package-release.sh TAG` 生成源码归档、镜像归档和外部 `SHA256SUMS`；脚本只归档标签中的文件，不使用运行容器制作镜像。

正式 Release、镜像或离线包生成后，应以对应 CI 产物和仓库发布说明为准。
