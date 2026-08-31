# Docker Compose

当前 Compose 只运行 PostgreSQL；两个 Index 数据库使用带 pgvector Extension 的镜像。Agent Server、独立 Index 与 Vite Web 作为本地进程启动。

| Service               | Profile | Host Port | Database               | 用途                         |
| --------------------- | ------- | --------: | ---------------------- | ---------------------------- |
| `postgres`            | default |   `55432` | `aegislink`            | Agent Server 开发数据         |
| `postgres-test`       | `test`  |   `55433` | `aegislink_test`       | Agent Server Repository 测试  |
| `index-postgres`      | default |   `55434` | `aegislink_index`      | Index Registry/Vector 数据    |
| `index-postgres-test` | `test`  |   `55435` | `aegislink_index_test` | Index pgvector 集成测试       |

```bash
make dev             # 启动两个开发数据库与全部本地进程
make dev-db          # 启动开发 PostgreSQL
make dev-test-db     # 启动隔离测试 PostgreSQL 并等待健康状态
make test-integration
make dev-index-db
make test-index-integration
```

`make dev` 在 `Ctrl-C` 后只停止受监督的 Agent Server、Index 与 Vite 进程，两个开发数据库 Container 会继续运行。使用 `docker compose stop postgres index-postgres` 可停止 Container，但不会删除命名 Volume。

`docker compose restart postgres` 和普通容器重建会保留开发命名卷。`docker compose down -v` 等删除 Volume 的命令会有意清除数据，不属于正常开发流程。

当前 Compose 没有定义 Agent Server、Index 进程、Gateway、Redis、Queue、Observability Stack 或 Kubernetes Service；只管理四个相互隔离的 PostgreSQL 容器及其 Volume。
