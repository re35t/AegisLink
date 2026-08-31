# Docker Compose

The committed Compose file runs PostgreSQL only; both Index databases use an image with pgvector. The Agent Server, standalone Index, and Vite Web application run as local processes.

| Service               | Profile | Host port | Database               | Purpose                                  |
| --------------------- | ------- | --------: | ---------------------- | ---------------------------------------- |
| `postgres`            | default |   `55432` | `aegislink`            | Agent Server development data            |
| `postgres-test`       | `test`  |   `55433` | `aegislink_test`       | Agent Server repository integration      |
| `index-postgres`      | default |   `55434` | `aegislink_index`      | Index Registry/vector data               |
| `index-postgres-test` | `test`  |   `55435` | `aegislink_index_test` | Index pgvector integration tests         |

```bash
make dev             # start both development databases and all local processes
make dev-db          # start development PostgreSQL
make dev-test-db     # start the isolated test PostgreSQL and wait for health
make test-integration
make dev-index-db
make test-index-integration
```

`make dev` leaves the two development database containers running after `Ctrl-C`; it stops only the supervised Agent Server, Index, and Vite processes. Use `docker compose stop postgres index-postgres` to stop the containers without deleting their named volumes.

`docker compose restart postgres` and ordinary container recreation preserve the named development volume. Commands that remove volumes, such as `docker compose down -v`, intentionally delete data and are not part of the normal development workflow.

No Agent Server, Index process, gateway, Redis, queue, observability stack, or Kubernetes service is defined by the current Compose file. Compose owns only the four PostgreSQL containers and their separate volumes.
