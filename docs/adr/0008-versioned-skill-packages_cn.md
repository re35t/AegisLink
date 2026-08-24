# ADR 0008：采用版本化 Skill Package 与 Agent Binding

状态：已接受并实现。

日期：2026-08-24。

## 背景

初版 `skills` 表以 `(owner_principal_id, name)` 唯一，并把 Package 元数据、`SKILL.md` 内容和版本混在同一行。`agent_skills` 虽然隔离了启停状态，但同一用户的两个 Agent 无法选择同名 Skill 的不同内容，重新安装也可能覆盖未来多 Agent 的版本语义。

## 决策

```text
Human Principal
└── skill_packages                 stable name and package id
    └── skill_versions             immutable content and hash
        └── agent_skills           selected version + enabled per Agent
```

- `skill_packages` 以 `(owner_principal_id, name)` 唯一，提供稳定 Package ID。
- `skill_versions` 保存 version label、description、完整 `SKILL.md`、content hash 和 source type；同一 Package 内 version label 和 content hash 分别唯一。
- `agent_skills` 以 `(agent_id, package_id)` 唯一，保存该 Agent 当前选择的 `version_id` 与 enabled 状态。
- 未提供 version label 的 inline 内容使用 `local-<hash prefix>`，相同内容可复用同一 Version，不同内容自然产生新 Version。
- 显式 version label 已存在但内容 hash 不同时返回冲突，禁止静默修改 immutable Version。
- 再次安装同名 Package 会创建或复用 Version，并只切换目标 Agent 的 binding；其他 Agent 保持原 Version。
- Uninstall 只删除当前 Agent binding，Package 与 Version 暂时保留用于复用和审计。后续如需要物理删除，增加显式 purge API 与引用检查。
- Runtime 仍只装载当前 Conversation Agent 已启用 binding 对应的 Version，不接触其他 Version。

## 迁移

`00004_skill_packages_and_versions.sql` 将每一条旧 `skills` 记录迁移为一个 Package 和一个 Version，并把旧 `agent_skills.skill_id` 同时映射为 `package_id` 与 `version_id`，保留内容、hash、启用状态和时间戳。

## 后续演进

- ADR 0009 已在该版本模型上增加本地 Markdown/ZIP Bundle 与 immutable files；本 ADR 的 Package/Version/Binding 语义保持不变。
- 当前管理页通过再次安装来创建/选择 Version，尚未提供 Package Library 和历史 Version 选择器。
- Version label 不是信任或发布签名；未来远端来源需要 provenance、签名和安全扫描。
