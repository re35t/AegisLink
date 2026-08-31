# Local development

## Start

```bash
cp .env.example .env
# Set MODEL_API_KEY in .env.
pnpm install
make dev
```

`make dev` loads `.env` followed by `.env.local`, waits for both development PostgreSQL databases, then supervises the Agent Server, standalone Index, and Vite Web application in one terminal. The default Web URL is `http://127.0.0.1:5173`, the API is `http://127.0.0.1:4321`, Index is `http://127.0.0.1:4331`, and the two PostgreSQL databases are exposed on `127.0.0.1:55432` and `127.0.0.1:55434`.

If neither environment file sets `AGENT_KEY_ENCRYPTION_KEY`, `make dev` generates a stable base64-encoded 32-byte key for local development and stores it in the Git-ignored, permission-restricted `.aegislink-dev/agent-key`. Later starts reuse that key so AgentFacts and Collaboration Session ciphertext remains decryptable across restarts. An explicitly configured environment value always takes precedence.

Press `Ctrl-C` to stop all three local processes. Development database containers keep running and their named volumes are preserved. Stop them explicitly when desired:

```bash
docker compose stop postgres index-postgres
```

The existing `make dev-db`, `make dev-server`, `make dev-index-db`, `make dev-index`, and `make dev-web` targets remain available for granular startup.

It reads `INDEX_SERVER_ADDRESS`, `INDEX_SERVER_SHUTDOWN_TIMEOUT`, `INDEX_DATABASE_URL`, `INDEX_REGISTRATION_TOKEN`, and `INDEX_QUERY_TOKEN`. It does not require `DATABASE_URL`, `MODEL_API_KEY`, or the AgentFacts encryption key. The Index pgvector database is independent from the Agent Server database.

To connect Agent Server onboarding, Profile synchronization, and Discovery search to that process, configure `AGENT_INDEX_BASE_URL`, `AGENT_INDEX_REGISTRATION_TOKEN`, and `AGENT_INDEX_QUERY_TOKEN`. Configure the OpenAI-compatible embedding endpoint with `DISCOVERY_ENCODER_BASE_URL` and `DISCOVERY_ENCODER_MODEL`; set `DISCOVERY_ENCODER_API_KEY` when the provider requires authentication. Existing configured accounts can run with Agent Index disabled, but a newly registered account cannot complete its required setup until Index and the encoder are available. All required Agent Index and encoder fields are enabled as one group; partial configuration is rejected at startup. The encoder must produce the Index profile's fixed 1536-dimensional vectors.

Goose applies ordered migrations when the server opens the database. Runtime persistence uses GORM; `AutoMigrate` is intentionally disabled.

Curator configuration is optional: every empty provider/model field falls back to the corresponding `MODEL_*` value. Curator requests use JSON Output and `CURATOR_MODEL_THINKING=disabled` by default; the thinking switch is applied by the DeepSeek driver. AgentFacts publication and cross-Agent Collaboration require `AGENT_KEY_ENCRYPTION_KEY` to contain the base64 encoding of exactly 32 random bytes. Keep this key stable and secret; changing it requires signing-key rotation and invalidates unexpired Collaboration Sessions.

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
