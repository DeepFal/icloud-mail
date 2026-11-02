# 更新日志

本文件记录 DeepFal/icloud-mail 的公开源码变化。当前源码元数据版本仍为 `0.2.9`；本仓库尚未发布正式 Release。API 路径、数据库 schema 和邮箱凭据版本分别管理。

## Unreleased

这是 DeepFal 初始公开源码，功能清单以当前源码和文档为准：

- 收件箱、MIME 归档、纯文本和安全 HTML 展示、附件与原文下载，以及网页已读、星标、归档和垃圾箱状态。
- 历史补取持久任务，支持暂停、恢复、取消和断点接续；实时收件与补取游标分开维护。
- 多主号、隐私邮箱同步、搜索筛选、批量创建与删除，以及按主号配置的自动创建计划。
- 邮箱池 Pool Key、领取确认、续租、释放、退役和批量确认；已使用地址不会重新作为可用库存发放。
- 每邮箱独立 API Key、IMAP/OAuth 凭据和取件链接；支持按需读取、连接复用、OTP API 与 recent-mail API。
- 邮箱本地备注、服务登记和状态查询；登记与第三方服务实际注册相互独立。
- 可选自动登录和会话恢复；Apple 限流、账号状态和上游网络仍可能影响运行结果。
- SQLite 与 PostgreSQL 启动迁移、备份脚本、Docker 源码构建编排和管理端 OpenAPI 文档。
- 深色与浅色主题，前端资源随服务本地提供。

详细行为、兼容约束、升级步骤和安全边界见 [README.md](README.md)、[DELIVERY.md](DELIVERY.md)、[docs/REFERENCE.md](docs/REFERENCE.md) 与 [NOTICE.md](NOTICE.md)。
