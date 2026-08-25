package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"
	"github.com/re35t/AegisLink/migrations"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Database struct {
	connection *gorm.DB
}

func Open(ctx context.Context, databaseURL string) (*Database, error) {
	connection, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	sqlDatabase, err := connection.DB()
	if err != nil {
		return nil, fmt.Errorf("access postgres connection pool: %w", err)
	}
	sqlDatabase.SetMaxOpenConns(12)
	sqlDatabase.SetMaxIdleConns(4)
	sqlDatabase.SetConnMaxLifetime(30 * time.Minute)
	if err := sqlDatabase.PingContext(ctx); err != nil {
		_ = sqlDatabase.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Database{connection: connection}, nil
}

func (database *Database) Migrate() error {
	sqlDatabase, err := database.connection.DB()
	if err != nil {
		return fmt.Errorf("access postgres connection pool for migrations: %w", err)
	}
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("configure migrations: %w", err)
	}
	if err := goose.Up(sqlDatabase, "."); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func (database *Database) Close() error {
	sqlDatabase, err := database.connection.DB()
	if err != nil {
		return fmt.Errorf("access postgres connection pool: %w", err)
	}
	if err := sqlDatabase.Close(); err != nil {
		return fmt.Errorf("close postgres: %w", err)
	}
	return nil
}

func gormArguments(args ...any) []any {
	arguments := make([]any, len(args))
	for index, argument := range args {
		arguments[index] = sql.Named(fmt.Sprintf("p%d", index+1), argument)
	}
	return arguments
}

func raw(database *gorm.DB, query string, args ...any) *gorm.DB {
	return database.Raw(query, gormArguments(args...)...)
}

func exec(database *gorm.DB, query string, args ...any) *gorm.DB {
	return database.Exec(query, gormArguments(args...)...)
}

func scanOne(database *gorm.DB, destination any, query string, args ...any) error {
	result := raw(database, query, args...).Scan(destination)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505"
}
