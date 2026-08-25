# ADR 0013: Use GORM for PostgreSQL persistence

## Status

Accepted.

## Context

The Go backend had repository code split across direct `database/sql` calls and pgx-specific error handling. As the Personal Agent OS adds Profile, Skill, MCP, and replayable Run state, consistent context propagation, transaction ownership, result handling, and persistence conventions are more valuable than maintaining a second low-level database style.

## Decision

- Every runtime database operation under `internal/postgres` uses GORM.
- Repository constructors accept `*gorm.DB`; GORM-specific models and clauses remain inside the PostgreSQL adapter.
- Complex joins, locking statements, PostgreSQL arrays, CTEs, and `RETURNING` operations may use GORM `Raw` or `Exec` when the query builder would obscure the invariant.
- Repository methods always use `WithContext(ctx)` and GORM-managed `Transaction` callbacks.
- Domain services and HTTP handlers remain independent of GORM.
- Goose remains the sole schema migration mechanism. `AutoMigrate` is prohibited.

## Consequences

The persistence adapter has one connection and transaction abstraction, while PostgreSQL-specific behavior remains explicit where it matters. Integration tests continue to run against real PostgreSQL because GORM does not replace database constraints, locking, or migration verification.
