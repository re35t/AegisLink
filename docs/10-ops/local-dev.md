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

Goose applies ordered migrations when the server opens the database. Runtime persistence uses GORM; `AutoMigrate` is intentionally disabled.

Curator configuration is optional: every empty provider/model field falls back to the corresponding `MODEL_*` value. Curator requests use JSON Output and `CURATOR_MODEL_THINKING=disabled` by default; the thinking switch is applied by the DeepSeek driver. AgentFacts publication remains unavailable until `AGENT_KEY_ENCRYPTION_KEY` contains the base64 encoding of exactly 32 random bytes. Keep this key stable and secret; changing it requires signing-key rotation.

## Validate

```bash
make generate
make check
make test
make build
```

`make test` skips PostgreSQL integration tests unless `TEST_DATABASE_URL` is set. Use the safe project target for them:

```bash
make test-integration
```

This starts `postgres-test` on port `55433`, waits for health, and targets `aegislink_test`. Repository integration tests truncate tables and roll migrations backward, so their guard rejects any database name that does not end in `_test`.

Development and test databases use separate named volumes. Restarting the development service preserves data; deleting its volume does not.
