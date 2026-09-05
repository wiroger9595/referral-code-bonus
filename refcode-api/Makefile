.PHONY: run build test test-db test-db-reset sqlc migrate-up migrate-down migrate-status seed fmt vet

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

# 連資料庫的測試用本機一次性的 database，跟 .env 的 DATABASE_URL（Supabase 正式那台）
# 完全分開 —— 測試會清表，指錯地方就是清掉正式資料。這裡刻意不吃 .env，
# 連線字串寫死在下面，不給它連到別處的機會。
TEST_DB_NAME = refcode_test
TEST_DATABASE_URL = postgres://$(USER)@localhost:5432/$(TEST_DB_NAME)?sslmode=disable

# schema 是 00001 才建的，但 goose 的版本表要放進那個 schema，所以先手動開。
# extensions 是 Supabase 特有的擺放慣例（00011 的 pg_trgm 指定放那裡），本機要自己補。
test-db:
	@createdb $(TEST_DB_NAME) 2>/dev/null || true
	@psql -d $(TEST_DB_NAME) -qc "CREATE SCHEMA IF NOT EXISTS referral_code_bonus;"
	@psql -d $(TEST_DB_NAME) -qc "CREATE SCHEMA IF NOT EXISTS extensions;"
	@GOOSE_DRIVER=postgres GOOSE_DBSTRING="$(TEST_DATABASE_URL)" \
		GOOSE_TABLE=referral_code_bonus.goose_db_version \
		goose -dir db/migrations up
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./...

# migration 改壞了、或表結構跟測試對不起來時整個重建。
test-db-reset:
	dropdb --if-exists $(TEST_DB_NAME)
	$(MAKE) test-db

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
