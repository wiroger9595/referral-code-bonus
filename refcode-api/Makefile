.PHONY: run build test sqlc migrate-up migrate-down migrate-status seed fmt vet

# goose 不讀 .env，這裡從同一份設定取連線字串，避免兩邊分岔。
DATABASE_URL ?= $(shell grep -E '^DATABASE_URL=' .env 2>/dev/null | cut -d= -f2-)

# 版本表要放哪個 schema 跟著 DATABASE_URL 走 —— 共用的 DB 不能把它丟進 public。
# .env 沒設就留空，goose 用它自己的預設（public.goose_db_version）。
GOOSE_TABLE ?= $(shell grep -E '^GOOSE_TABLE=' .env 2>/dev/null | cut -d= -f2-)
GOOSE_ENV = GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(DATABASE_URL)" $(if $(GOOSE_TABLE),GOOSE_TABLE="$(GOOSE_TABLE)")
GOOSE = $(GOOSE_ENV) goose -dir db/migrations

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

sqlc:
	sqlc generate

migrate-up:
	$(GOOSE) up

# 套用前先自己看過產生的 SQL，這個指令會真的改資料庫。
migrate-down:
	$(GOOSE) down

migrate-status:
	$(GOOSE) status

# 只給本機：建第一個 admin 並塞 demo 服務商。
seed:
	go run ./cmd/seed -admin-email=$(EMAIL) -admin-password=$(PASSWORD) -demo
