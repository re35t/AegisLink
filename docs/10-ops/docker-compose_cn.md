# Docker Compose

当前 Compose 只运行 PostgreSQL；Go Server 与 Vite Web 作为本地进程启动。

| Service         | Profile | Host Port | Database         | Volume                    | 用途                       |
| --------------- | ------- | --------: | ---------------- | ------------------------- | -------------------------- |
| `postgres`      | default |   `55432` | `aegislink`      | `aegislink-postgres`      | 持久化开发数据             |
| `postgres-test` | `test`  |   `55433` | `aegislink_test` | `aegislink-postgres-test` | 破坏性 Repository 集成测试 |

```bash
make dev-db          # 启动开发 PostgreSQL
make dev-test-db     # 启动隔离测试 PostgreSQL 并等待健康状态
make test-integration
```

`docker compose restart postgres` 和普通容器重建会保留开发命名卷。`docker compose down -v` 等删除 Volume 的命令会有意清除数据，不属于正常开发流程。

当前 Compose 没有定义 Server、Gateway、Redis、Vector Database、Queue、Observability Stack 或 Kubernetes Service。
