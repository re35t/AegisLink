# Docker Compose

The committed Compose file runs PostgreSQL only; the Go server and Vite Web application run as local processes.

| Service         | Profile | Host port | Database         | Volume                    | Purpose                                  |
| --------------- | ------- | --------: | ---------------- | ------------------------- | ---------------------------------------- |
| `postgres`      | default |   `55432` | `aegislink`      | `aegislink-postgres`      | persistent development data              |
| `postgres-test` | `test`  |   `55433` | `aegislink_test` | `aegislink-postgres-test` | destructive repository integration tests |

```bash
make dev-db          # start development PostgreSQL
make dev-test-db     # start the isolated test PostgreSQL and wait for health
make test-integration
```

`docker compose restart postgres` and ordinary container recreation preserve the named development volume. Commands that remove volumes, such as `docker compose down -v`, intentionally delete data and are not part of the normal development workflow.

No server, gateway, Redis, vector database, queue, observability stack, or Kubernetes service is defined by the current Compose file.
