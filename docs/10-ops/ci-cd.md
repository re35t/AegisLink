# Continuous integration

`.github/workflows/ci.yml` runs on pull requests and pushes to `main` with Go 1.26, Node 22, pnpm 10.4, and one PostgreSQL 17 service hosting separate `aegislink_test` and `aegislink_index_test` databases.

The workflow verifies:

1. locked dependency installation;
2. OpenAPI TypeScript regeneration has no Git drift;
3. Go formatting and `go vet` across the Agent Server and standalone Index;
4. both `aegislink-server` and `aegislink-index` build;
5. all Go tests, including Index Registry integration, Agent Server PostgreSQL integration, and mock model/MCP tests;
6. Web TypeScript, lint, formatting, Vitest, and production build.

CI does not currently build a container image, publish artifacts, deploy environments, validate Kubernetes manifests, or run maintained browser automation. Do not treat those as release guarantees.
