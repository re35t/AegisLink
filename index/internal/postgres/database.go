package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/pressly/goose/v3"
	indexmigrations "github.com/re35t/AegisLink/index/migrations"
	postgresdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Database struct {
	connection *gorm.DB
}

func Open(ctx context.Context, databaseURL string) (*Database, error) {
	connection, err := gorm.Open(postgresdriver.Open(databaseURL), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open Index postgres: %w", err)
	}
	sqlDatabase, err := connection.DB()
	if err != nil {
		return nil, fmt.Errorf("access Index postgres connection pool: %w", err)
	}
	sqlDatabase.SetMaxOpenConns(24)
	sqlDatabase.SetMaxIdleConns(8)
	sqlDatabase.SetConnMaxLifetime(30 * time.Minute)
	if err := sqlDatabase.PingContext(ctx); err != nil {
		_ = sqlDatabase.Close()
		return nil, fmt.Errorf("ping Index postgres: %w", err)
	}
	return &Database{connection: connection}, nil
}

func (database *Database) Migrate() error {
	sqlDatabase, err := database.connection.DB()
	if err != nil {
		return fmt.Errorf("access Index postgres pool for migrations: %w", err)
	}
	goose.SetBaseFS(indexmigrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("configure Index migrations: %w", err)
	}
	if err := goose.Up(sqlDatabase, "."); err != nil {
		return fmt.Errorf("apply Index migrations: %w", err)
	}
	return nil
}

func (database *Database) Ready(ctx context.Context) error {
	sqlDatabase, err := database.connection.DB()
	if err != nil {
		return fmt.Errorf("access Index postgres pool: %w", err)
	}
	if err := sqlDatabase.PingContext(ctx); err != nil {
		return fmt.Errorf("ping Index postgres: %w", err)
	}
	return nil
}

func (database *Database) Close() error {
	sqlDatabase, err := database.connection.DB()
	if err != nil {
		return fmt.Errorf("access Index postgres pool: %w", err)
	}
	if err := sqlDatabase.Close(); err != nil {
		return fmt.Errorf("close Index postgres: %w", err)
	}
	return nil
}
