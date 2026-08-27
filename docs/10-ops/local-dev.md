# Local development

## Start

```bash
cp .env.example .env
# Set MODEL_API_KEY in .env.
pnpm install
make dev-db
make dev-server
```

Run `make dev-web` in another terminal. The default Web URL is `http://127.0.0.1:5173`, the API is `http://127.0.0.1:4321`, and development PostgreSQL is exposed on `127.0.0.1:55432`.

The standalone Index is optional and runs with its own PostgreSQL database:

```bash
make dev-index-db
make dev-index
curl http://127.0.0.1:4331/healthz
curl http://127.0.0.1:4331/readyz
```

It reads `INDEX_SERVER_ADDRESS`, `INDEX_SERVER_SHUTDOWN_TIMEOUT`, `INDEX_DATABASE_URL`, and `INDEX_REGISTRATION_TOKEN`. It does not require `DATABASE_URL`, `MODEL_API_KEY`, or the AgentFacts encryption key. The Registry database is independent from the Agent Server database.

Goose applies ordered migrations when the server opens the database. Runtime persistence uses GORM; `AutoMigrate` is intentionally disabled.

Curator configuration is optional: every empty provider/model field falls back to the corresponding `MODEL_*` value. Curator requests use JSON Output and `CURATOR_MODEL_THINKING=disabled` by default; the thinking switch is applied by the DeepSeek driver. AgentFacts publication remains unavailable until `AGENT_KEY_ENCRYPTION_KEY` contains the base64 encoding of exactly 32 random bytes. Keep this key stable and secret; changing it requires signing-key rotation.

## Validate

```bash
make generate
make check
make test
make build
```

`make check`, `make test`, and `make build` cover both Go processes; the build target emits both `aegislink-server` and `aegislink-index`. PostgreSQL integration tests are skipped unless their dedicated URL is set. Use the safe project targets:

```bash
make test-integration
make test-index-integration
```

These start `postgres-test` on `55433` and `index-postgres-test` on `55435`, then target `aegislink_test` and `aegislink_index_test`. Repository integration tests truncate tables, so their guards reject any database name that does not end in `_test`.

Development and test databases use separate named volumes. Restarting the development service preserves data; deleting its volume does not.
