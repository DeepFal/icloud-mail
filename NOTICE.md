# 版权、来源与许可范围

本项目由 DeepFal 在独立仓库 `DeepFal/icloud-mail` 维护。项目名称、维护账号和仓库历史的调整不改变既有代码与第三方组件的权利归属。

## 版权与许可

根目录 [LICENSE](LICENSE) 保留已有的 MIT 许可文本和 `t508708` 版权声明，并增加 DeepFal 对有权许可的修改所作的版权声明。该文本仅在相应权利人有权授权的范围内生效；本项目不据此主张拥有所有既有代码、其他贡献者或第三方依赖的版权。

既有代码包含源自 [mangobubu/icloud-api](https://github.com/mangobubu/icloud-api) 的实现。2026-11-02 核查其公开提交 [`866a0f059200dad544a2f759f6a3acde31a24c1f`](https://github.com/mangobubu/icloud-api/tree/866a0f059200dad544a2f759f6a3acde31a24c1f) 时，GitHub 未识别通用许可证，完整文件树中未找到常见 LICENSE、LICENCE、COPYING 或 NOTICE 文件。该提交用于固定许可核查依据，不表示本版本合入了其全部功能。

公开可读不等于已获得任意重新许可或再分发授权。本说明不替代相应权利人的授权，也不把根目录 MIT 文本扩展为对整个组合代码库的统一授权。如存在单独授权，应以该授权及相应权利人的适用条款为准。

## 第三方参考与依赖

- [q1953258942/iCloud-Privacy-Mail](https://github.com/q1953258942/iCloud-Privacy-Mail)：Apple Account 协议流程参考。
- [cquxl/iCloud-Alias-Hub](https://github.com/cquxl/iCloud-Alias-Hub)：连接续期与管理交互参考；其许可说明不自动覆盖本项目。
- [pyicloud](https://github.com/picklepete/pyicloud)、[ImapFlow](https://github.com/postalsys/imapflow)：会话与 IMAP 工程实践参考。

Go 与 npm 依赖保留各自的许可证、版权及 NOTICE。具体版本见 `go.mod`、`go.sum`、`web/package-lock.json`。Docker 构建会将本项目许可说明、Go 标准库及构建依赖中的许可文件收集到 `/usr/share/licenses/icloud-mail/`；基础镜像组件沿用其发行版的许可。

本项目是独立自托管工具，与 Apple Inc. 无隶属或背书关系。

## English

DeepFal maintains this project at `DeepFal/icloud-mail`. Changes to the project identity or repository history do not transfer ownership of pre-existing code or third-party components.

[LICENSE](LICENSE) retains the existing MIT text and the `t508708` copyright notice, with a notice for modifications DeepFal has the right to license. These statements apply only to material the respective grantors have the right to license. They do not assert ownership or a uniform MIT grant for every part of the combined codebase.

Pre-existing implementations include code originating from [mangobubu/icloud-api](https://github.com/mangobubu/icloud-api). On 2026-11-02, GitHub identified no general-purpose license at commit `866a0f059200dad544a2f759f6a3acde31a24c1f`, and the complete tree contained no common LICENSE, LICENCE, COPYING or NOTICE file. This reference records a licensing check, not incorporation of every upstream change. Public access alone does not establish relicensing or redistribution permission; any separate authorization and applicable rights-holder terms remain controlling.

Referenced projects and Go/npm dependencies retain their own rights and license terms. Build-time license collection preserves the applicable files and version indexes under `/usr/share/licenses/icloud-mail/`. Base-image components retain their distribution licenses. This is an independent self-hosted tool, not affiliated with or endorsed by Apple Inc.
