# 推薦碼媒合平台 — 工作區

**跟同層的 `braoem/` 完全無關**，兩邊沒有共用任何東西。

四個模組在同一個 git repo 底下（不是 submodule，根目錄的 git 全部追蹤）：

| 目錄 | 技術 | port | 說明 |
|---|---|---|---|
| `refcode-api/` | Go + Postgres | 7802 | API、排序引擎、審核、計費 |
| `refcode-admin/` | Vue 3 + Vite + Naive UI | 5173 | 內部後台 |
| `refcode-web/` | Nuxt 4（SSR）+ Tailwind | 3000 | 官網，SEO 主力 |
| `refcode-app/` | Vue 3 + Ionic + Capacitor | 5174 | iOS / Android |

延伸文件——**需要細節時自己去讀，不要憑印象回答**：
- `README.md`：前置工具版本、各模組單獨啟動方式、常見問題
- `PLAN.md`：產品規劃
- `refcode-*/README.md`：各模組自己的說明
- `refcode-api/.env.example`：每個環境變數的用途與踩坑，註解比程式碼詳細

## 啟動一律用 ./dev.sh

不要 `cd` 進子目錄、不要自己記 port。`.env` 和 `node_modules` 缺了腳本會自動補。

```bash
./dev.sh status                 # 先看四個 port 有沒有在聽
./dev.sh api|admin|web|app      # 單獨前景啟動，Ctrl-C 停止
./dev.sh all                    # 四個背景啟動，log 寫進 .logs/
./dev.sh stop [模組...]         # 不給模組就四個全停
./dev.sh logs <模組>            # 跟蹤 log
```

## 硬規則

- **沒有本機資料庫。** 本機開發直接連 Supabase 上那一台，跟正式環境同一個實例——
  任何會改資料的操作都是真的改下去，沒有可以先試錯的沙箱。
- **那台 Postgres 裝了好幾個 side project，用 schema 隔開。** 所有東西都放
  `referral_code_bonus` schema，查詢明確指定，不要依賴連線的 `search_path`。
  goose 的版本表也一樣（`.env` 的 `GOOSE_TABLE`），不要弄髒 `public`。
- **migration 套用前一定要人工看過 SQL**，`make migrate-up` 會直接動到上面那台。
- **`db/queries/*.sql` 改完要 `make sqlc` 重新產生**，不要手改產出來的 Go code，
  下次 generate 會被蓋掉。
- **`make seed` / `cmd/seed` 只給本機跑**，正式環境不要碰。
- **`.env` 一律不進 repo**（`.gitignore` 已擋），正式值填在 Vercel / Northflank。

## 慣例

- 回答與 commit message 用繁體中文。
- **測試只有 `refcode-api` 有**（`make test`，實際跑 `go test ./...`），
  三個前端目前沒有測試設定——不要假裝有測試可以跑。
- 訂閱狀態的真相在 RevenueCat，後端只收 webhook 存一份副本供伺服器端判斷，
  不要把後端那份當來源改。
