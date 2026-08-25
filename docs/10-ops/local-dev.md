# Local Dev

Install dependencies and start PostgreSQL:

```bash
pnpm install
make dev-db
make dev-server
```

Run repository integration tests against the dedicated test database:

```bash
make test-integration
```

Repository integration tests truncate tables and roll migrations backward. They reject any `TEST_DATABASE_URL` whose database name does not end in `_test`; never point them at the development database.

Goose applies migrations when the server opens the database. GORM owns runtime queries and transactions; `AutoMigrate` is intentionally disabled so local and production schema history stay identical.
