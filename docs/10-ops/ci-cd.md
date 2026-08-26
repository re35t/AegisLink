# Continuous integration

`.github/workflows/ci.yml` runs on pull requests and pushes to `main` with Go 1.26, Node 22, pnpm 10.4, and a dedicated PostgreSQL 17 `aegislink_test` service.

The workflow verifies:

1. locked dependency installation;
2. OpenAPI TypeScript regeneration has no Git drift;
3. Go formatting and `go vet`;
4. all Go tests, including PostgreSQL repository integration and mock model/MCP tests;
5. Web TypeScript, lint, formatting, Vitest, and production build.

CI does not currently build a container image, publish artifacts, deploy environments, validate Kubernetes manifests, or run maintained browser automation. Do not treat those as release guarantees.
