# ADR 0010：账户设置与显式界面偏好

## 状态

已接受

## 决策

AegisLink 通过 REST 提供登录后的账户设置。账户资料和界面偏好属于 Human Principal，而不属于某个 Personal Agent。

第一阶段只在 PostgreSQL 中保存显式的 `language` 与 `theme` 字段，API 不接受任意 JSON 设置袋。修改密码必须验证当前密码；修改成功后保留发起请求的当前会话，同时撤销该账户的其他活动会话。

React 客户端通过 TanStack Query 读取设置。一个很小的 Provider 负责解析“跟随系统”的实际语言和配色并应用到 document。表单草稿只保存在组件本地，服务端更新成功后才替换 Query 数据。

## 影响

- 未来同一用户创建第二个 Agent 时会共享用户界面偏好，但 Memory、Skills、MCP 连接和 Runtime 配置仍按 Agent 隔离。
- 新增持久化偏好必须同时提供显式迁移、校验和 API 合约。
- 在邮箱验证流程实现之前，登录邮箱保持只读。
- 当前语言切片覆盖应用导航和设置页面，其他产品文案可沿用同一翻译边界逐步接入。
