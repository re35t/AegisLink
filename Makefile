.PHONY: dev-db dev-test-db dev-index-db dev-index-test-db dev-server dev-index dev-web generate check test test-integration test-index-integration build

dev-db:
	docker compose up -d postgres

dev-test-db:
	docker compose --profile test up -d --wait postgres-test

dev-index-db:
	docker compose up -d --wait index-postgres

dev-index-test-db:
	docker compose --profile test up -d --wait index-postgres-test

dev-server:
	go run ./cmd/aegislink-server

dev-index:
	go run ./index/cmd/aegislink-index

dev-web:
	pnpm dev:web

generate:
	pnpm generate:api

check:
	test -z "$$(gofmt -l cmd index internal migrations)"
	go vet ./...
	pnpm check:web

test:
	go test ./...
	pnpm test:web

test-integration: dev-test-db
	TEST_DATABASE_URL='postgres://aegislink:aegislink@127.0.0.1:55433/aegislink_test?sslmode=disable' go test ./internal/postgres

test-index-integration: dev-index-test-db
	INDEX_TEST_DATABASE_URL='postgres://aegislink_index:aegislink_index@127.0.0.1:55435/aegislink_index_test?sslmode=disable' go test ./index/internal/postgres

build:
	go build ./cmd/aegislink-server
	go build ./index/cmd/aegislink-index
	pnpm build:web
