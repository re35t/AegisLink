.PHONY: dev-db dev-server dev-web generate check test build

dev-db:
	docker compose up -d postgres

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

build:
	go build ./cmd/aegislink-server
	pnpm build:web
