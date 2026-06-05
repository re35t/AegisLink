# 信任边界

agent 的私有记忆默认保持在本地，只有在 capability 和 policy 明确允许时才暴露。server 可以路由和审计，但除非部署模式明确允许，否则不应接收私有 payload。
