# Docker Compose

The committed Compose file runs PostgreSQL only; the Agent Server, standalone Index shell, and Vite Web application run as local processes.

| Service               | Profile | Host port | Database               | Purpose                                  |
| --------------------- | ------- | --------: | ---------------------- | ---------------------------------------- |
| `postgres`            | default |   `55432` | `aegislink`            | Agent Server development data            |
| `postgres-test`       | `test`  |   `55433` | `aegislink_test`       | Agent Server repository integration      |
| `index-postgres`      | default |   `55434` | `aegislink_index`      | standalone Index Registry data           |
| `index-postgres-test` | `test`  |   `55435` | `aegislink_index_test` | Index Registry integration tests         |

```bash
make dev-db          # start development PostgreSQL
make dev-test-db     # start the isolated test PostgreSQL and wait for health
make test-integration
make dev-index-db
make test-index-integration
```

`docker compose restart postgres` and ordinary container recreation preserve the named development volume. Commands that remove volumes, such as `docker compose down -v`, intentionally delete data and are not part of the normal development workflow.

No Agent Server, Index process, gateway, Redis, vector extension, queue, observability stack, or Kubernetes service is defined by the current Compose file. Compose owns only the four PostgreSQL containers and their separate volumes.
