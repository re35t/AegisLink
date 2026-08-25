.PHONY: dev-db dev-test-db dev-server dev-web generate check test test-integration build

dev-db:
	docker compose up -d postgres

dev-test-db:
	docker compose --profile test up -d --wait postgres-test

dev-server:
	go run ./cmd/aegislink-server

dev-web:
	pnpm dev:web

generate:
	pnpm generate:api

check:
	test -z "$$(gofmt -l cmd internal migrations)"
	go vet ./...
	pnpm check:web

test:
	go test ./...
	pnpm test:web

test-integration: dev-test-db
	TEST_DATABASE_URL='postgres://aegislink:aegislink@127.0.0.1:55433/aegislink_test?sslmode=disable' go test ./internal/postgres

build:
	go build ./cmd/aegislink-server
	pnpm build:web
