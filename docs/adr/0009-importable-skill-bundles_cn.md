# ADR 0009：本地 Skill Bundle 导入与只读资源

状态：已接受并实现。

日期：2026-08-24。

## 决策

- 自定义编辑继续通过 JSON REST 安装 inline `SKILL.md`。
- 本地导入使用 multipart REST，接受单个 Markdown 文件或 ZIP Bundle。
- ZIP 可有一个公共顶层目录，但规范化后必须包含根 `SKILL.md`；其余文件只能位于 `references/`、`assets/` 或 `scripts/`。
- 每个 Version 的规范化文件保存在 PostgreSQL `skill_version_files`，并记录 path、media type、size、hash 和 text-readable 状态。
- Bundle hash 由排序后的 path 与内容确定；只有内容完全一致时才复用 immutable Version。
- Runtime 的 `load_skill` 返回 `SKILL.md` 和资源清单；`read_skill_resource` 只读取当前 Agent 已启用 Version 的 UTF-8 文本资源。
- `scripts/` 文件只存储和读取，不赋予执行权限。binary assets 不进入模型文本上下文。

## 安全限制

- 上传文件最大 4 MiB，解包后最大 8 MiB、64 个文件，单文件最大 1 MiB。
- 拒绝绝对路径、`..` traversal、反斜杠路径、NUL、symlink、重复规范化路径和未声明根目录。
- `SKILL.md` 继续使用现有 frontmatter、UTF-8 和 128 KiB 限制。
- Runtime 只读取不超过 256 KiB 的 UTF-8 文件；导入不等于信任、签名或执行授权。

## 用户工作流

- Create or customize：编写新 Skill，或把当前 Version 复制到编辑器并创建新 Version。
- Import file：上传 `.md` 或 `.zip`，校验后自动绑定并启用到当前 Agent。
- Remove from agent：只删除 Agent binding；Package、Version 和 Bundle files 保留用于审计与复用。

## 后续边界

Git 来源、provenance、签名、恶意内容扫描、历史 Version 选择器、Bundle 导出和受控脚本执行不在本次范围内。
