package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"
	"github.com/re35t/AegisLink/migrations"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(ctx context.Context, databaseURL string) (*gorm.DB, error) {
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	sqlDatabase, err := database.DB()
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
	return database, nil
}

func Migrate(database *gorm.DB) error {
	sqlDatabase, err := database.DB()
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

// postgresStatement keeps existing PostgreSQL-numbered statements readable while
// routing parameter binding through GORM. It rewrites each $n occurrence to a
// GORM placeholder and repeats the matching argument when a statement reuses it.
func postgresStatement(query string, args ...any) (string, []any) {
	var statement strings.Builder
	variables := make([]any, 0, len(args))
	inSingleQuote := false
	inDoubleQuote := false
	for index := 0; index < len(query); {
		if query[index] == '\'' && !inDoubleQuote {
			statement.WriteByte(query[index])
			if inSingleQuote && index+1 < len(query) && query[index+1] == '\'' {
				statement.WriteByte(query[index+1])
				index += 2
				continue
			}
			inSingleQuote = !inSingleQuote
			index++
			continue
		}
		if query[index] == '"' && !inSingleQuote {
			statement.WriteByte(query[index])
			if inDoubleQuote && index+1 < len(query) && query[index+1] == '"' {
				statement.WriteByte(query[index+1])
				index += 2
				continue
			}
			inDoubleQuote = !inDoubleQuote
			index++
			continue
		}
		if inSingleQuote || inDoubleQuote {
			statement.WriteByte(query[index])
			index++
			continue
		}
		if query[index] != '$' || index+1 >= len(query) || query[index+1] < '0' || query[index+1] > '9' {
			statement.WriteByte(query[index])
			index++
			continue
		}
		end := index + 1
		for end < len(query) && query[end] >= '0' && query[end] <= '9' {
			end++
		}
		position, err := strconv.Atoi(query[index+1 : end])
		if err != nil || position < 1 || position > len(args) {
			statement.WriteString(query[index:end])
			index = end
			continue
		}
		statement.WriteByte('?')
		variables = append(variables, args[position-1])
		index = end
	}
	return statement.String(), variables
}

func raw(database *gorm.DB, query string, args ...any) *gorm.DB {
	statement, variables := postgresStatement(query, args...)
	return database.Raw(statement, variables...)
}

func exec(database *gorm.DB, query string, args ...any) *gorm.DB {
	statement, variables := postgresStatement(query, args...)
	return database.Exec(statement, variables...)
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
