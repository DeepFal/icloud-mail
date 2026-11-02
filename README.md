<div align="center">

**简体中文 | [English](README.en.md)**

# iCloud Mail

**由 DeepFal 维护的自托管 iCloud 邮箱管理与收件控制面板。**

管理收件箱、实际邮箱与本地备注，区分服务登记状态，并通过邮箱池和 API 为日常工具提供按需收件与取码。

[功能概览](#功能) · [源码构建](#快速开始) · [文档](#文档)

</div>

## 功能

- 收件箱与按需收件：集中查看邮件，访问取件链接或 API 时增量读取，并在同一主号内复用收件连接。
- 实际邮箱与本地备注：按完整邮箱地址保存备注；备注独立于 Apple 目录标签，删除后重新导入同一地址仍可恢复。
- 服务登记与状态：为邮箱记录服务和账号标识，区分未记录、已注册、待确认、确认未注册和已停用；登记只记录本地信息，不代表代为注册第三方服务。
- 多账号管理：在一个面板中维护多个 Apple 主号、隐私邮箱、同步和连接状态。
- 邮箱池与 API：为项目签发 Pool Key，支持领取、确认使用、续租、释放和退役，并用邮箱 API 获取验证码。
- 批量任务：批量同步、创建、删除、登记和确认；自动创建计划可按主号配置并受实际上游限制影响。
- 深浅主题：提供深色和浅色界面，前端资源随服务本地提供。

更完整的服务登记语义和日常操作见[邮箱备注与服务登记](docs/MAIL-REGISTRY.md)。

## 快速开始

需要 Docker Engine、Docker Compose v2 和 Git。源码构建需要访问 Docker 基础镜像、Go 模块和 npm 依赖源；建议至少 2 核 / 4 GB 内存，并保持单个应用实例。

```sh
git clone https://github.com/DeepFal/icloud-mail.git
cd icloud-mail
cp -n .env.example .env
docker compose -f compose.yaml -f compose.build.yaml up -d --build --wait
docker compose -f compose.yaml -f compose.build.yaml ps
```

使用示例配置时，管理入口为 `http://127.0.0.1:8788/admin/`，用户名为 `admin`。首次启动生成的管理路径和密码保存在容器的 keys 卷中：

```sh
docker compose -f compose.yaml -f compose.build.yaml exec -T icloud-api cat /app/keys/admin-path
docker compose -f compose.yaml -f compose.build.yaml exec -T icloud-api cat /app/keys/admin-password
```

在面板中依次执行“添加主号 → 配置收件 → 连接 Apple → 同步地址”，再复制取件链接或把邮箱加入邮箱池。直连 iCloud IMAP 使用 Apple App 专用密码；它与管理隐私邮箱所用的账户密码分开配置。

远程服务器可通过 SSH 隧道访问管理入口：

```sh
ssh -N -L 127.0.0.1:8788:127.0.0.1:8788 USER@SERVER
```

升级前请备份数据库、密钥和邮件归档，保留 `.env` 与已有数据卷；安装、备份、迁移和公网 HTTPS 说明见 [DELIVERY.md](DELIVERY.md) 与 [公网部署指南](docs/PUBLIC.md)。

## API 示例

将面板中对应邮箱的 API Key 交给自己的程序即可读取验证码：

```sh
curl -H 'Authorization: Bearer MAILBOX_API_KEY' \
  https://HOST/api/v1/otp
```

邮箱池接口、请求定义和部署后的接口说明见 [docs/POOL.md](docs/POOL.md)、[docs/openapi.yaml](docs/openapi.yaml) 和服务的 `/docs/`。

## 文档

| 主题 | 文档 |
| --- | --- |
| 安装、更新、备份与迁移 | [DELIVERY.md](DELIVERY.md) |
| 公网访问与 HTTPS | [docs/PUBLIC.md](docs/PUBLIC.md) |
| 邮箱池与程序接入 | [docs/POOL.md](docs/POOL.md) · [docs/openapi.yaml](docs/openapi.yaml) |
| 自动创建与登录恢复 | [docs/AUTO_CREATION_CHANNELS.md](docs/AUTO_CREATION_CHANNELS.md) · [docs/AUTO_LOGIN.md](docs/AUTO_LOGIN.md) |
| 收件与配置 | [docs/MAIL-RECEPTION.md](docs/MAIL-RECEPTION.md) · [docs/REFERENCE.md](docs/REFERENCE.md) |
| 邮箱备注与服务登记 | [docs/MAIL-REGISTRY.md](docs/MAIL-REGISTRY.md) |

## 许可与来源

[LICENSE](LICENSE) · [NOTICE](NOTICE.md)。具体许可范围以 NOTICE 为准。
