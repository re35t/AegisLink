# Local Dev

Install dependencies and start PostgreSQL:

```bash
pnpm install
make dev-db
make dev-server
```

Run repository integration tests with an explicit database URL:

```bash
TEST_DATABASE_URL='postgres://aegislink:aegislink@127.0.0.1:55432/aegislink?sslmode=disable' make test
```

Goose applies migrations when the server opens the database. GORM owns runtime queries and transactions; `AutoMigrate` is intentionally disabled so local and production schema history stay identical.
