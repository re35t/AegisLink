# 持续集成

`.github/workflows/ci.yml` 在 Pull Request 与推送到 `main` 时运行，环境包括 Go 1.26、Node 22、pnpm 10.4，以及独立 PostgreSQL 17 `aegislink_test` Service。

Workflow 验证：

1. 按 Lockfile 安装依赖；
2. OpenAPI TypeScript 重新生成后没有 Git Drift；
3. Go 格式与 `go vet`；
4. 全部 Go 测试，包括 PostgreSQL Repository 集成测试及 Mock Model/MCP 测试；
5. Web TypeScript、Lint、格式、Vitest 与生产构建。

CI 当前不构建 Container Image、不发布 Artifact、不部署环境、不验证 Kubernetes Manifest，也没有维护中的浏览器自动化；不能把这些能力视为发布保证。
