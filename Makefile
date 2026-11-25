-include .env
export

BIN           := bin/wallet-api
MIGRATE_IMAGE := migrate/migrate:v4.20.1

.PHONY: run build test lint fmt tidy up down logs db-up migrate-up migrate-down migrate-create

run:
	go run ./cmd/api

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/api

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...

tidy:
	go mod tidy

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f api

db-up:
	docker compose up -d postgres

migrate-up:
	docker compose run --rm migrate up

migrate-down:
	docker compose run --rm migrate down 1

migrate-create:
	@test -n "$(name)" || (echo "usage: make migrate-create name=<migration_name>" && exit 1)
	docker run --rm -v $(CURDIR)/migrations:/migrations $(MIGRATE_IMAGE) create -ext sql -dir /migrations -seq $(name)
