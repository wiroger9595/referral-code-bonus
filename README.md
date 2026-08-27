# 推薦碼媒合平台 — 工作區

這層是工作區，不是單一 repo。底下四個模組**各自是獨立的 git repo**：

| 目錄 | 技術 | port | 說明 |
|---|---|---|---|
| `refcode-api/` | Go + Postgres | 7802 | API、排序引擎、審核、計費（Phase 3） |
| `refcode-admin/` | Vue 3 + Vite + Naive UI | 5173 | 內部後台：審核、服務商目錄 |
| `refcode-web/` | Nuxt 4（SSR）+ Tailwind | 3000 | 官網，SEO 主力 |
| `refcode-app/` | Vue 3 + Ionic + Capacitor | 5174 | iOS / Android |

產品規劃見 `PLAN.md`。各模組的細節見各自的 README。

## 前置工具

`dev.sh` 會自動補**專案的**相依（`.env`、`node_modules`、`go mod download`），
但**系統層級的工具要自己裝**——腳本不會幫你裝 Go 或 Node。

```bash
brew install go node          # 跑起來最低限度需要這兩個
brew install goose sqlc       # 只有改 migration / SQL 時才會用到
brew install libpq            # 想用 psql 直接連資料庫看東西才需要
```

| 工具 | 目前版本 | 誰要用 | 沒有會怎樣 |
|---|---|---|---|
| Go | 1.26.5 | api | api 起不來 |
| Node | 26.0.0 | admin / web / app | 三個前端都起不來 |
| goose | 3.27.3 | 改 migration | `make migrate-*` 失敗 |
| sqlc | 1.31.1 | 改 `db/queries/*.sql` | `make sqlc` 失敗 |
| redis | 8.x | api 的忘記密碼 | 只有忘記密碼停用（回 503），其他照常 |

redis 存忘記密碼的驗證碼與寄送次數。**連不上不會擋 api 啟動**，
所以不碰那個流程的話可以不裝；`brew install redis && brew services start redis`。

goose 和 sqlc **不是 `go mod` 的相依**（是獨立的 CLI），所以 `go mod download` 不會裝到它們。
平常只跑服務的話不需要這兩個。

**資料庫是 Supabase（Postgres 17.4），本機開發也連同一台，不需要裝 local Postgres。**
連線字串在 Dashboard → Project Settings → Database → Connection string → Session pooler，
填進 `refcode-api/.env` 的 `DATABASE_URL`。連得上沒：

```bash
cd refcode-api && psql "$(grep -E '^DATABASE_URL=' .env | cut -d= -f2-)" -c '\dt referral_code_bonus.*'
```

###### app 啟動一律 會自動重啟
## 啟動一律用 ./dev.sh

```bash
./dev.sh status                 # 先看四個 port 有沒有在聽
./dev.sh api|admin|web|app      # 單獨前景啟動，Ctrl-C 停止
./dev.sh all                    # 四個背景啟動，log 寫進 .logs/
./dev.sh stop [模組...]         # 不給模組就四個全停
./dev.sh logs <模組>            # 跟蹤 log
```

`.env` 缺了會自動從 `.env.example` 複製，`node_modules` 缺了會自動裝。

第一次跑要先建表（資料庫是 Supabase 上現成的，不用自己 create）：

```bash
cd refcode-api && make migrate-up
make seed EMAIL=admin@local.test PASSWORD=admin12345   # 建 admin + demo 服務商，只給本機
```

## 各模組單獨啟動

平常用 `./dev.sh` 就好，下面是腳本背後實際做的事——要在某個 repo 裡單獨開發、或腳本壞掉時用。
四個都預設連 `http://localhost:7802` 的 API，前端的 port 寫死在各自的設定檔裡（改了要同步
`refcode-api/.env` 的 `CORS_ORIGINS`）。

| 模組 | 目錄 | 啟動 | port | 需要 |
|---|---|---|---|---|
| api | `refcode-api/` | `go run ./cmd/api` | 7802 | Go 1.26.5、連得到 Supabase |
| admin | `refcode-admin/` | `npm run dev` | 5173 | Node |
| web | `refcode-web/` | `npm run dev` | 3000 | Node |
| app | `refcode-app/` | `npm run dev` | 5174 | Node |

每個模組第一次跑之前：`cp .env.example .env`，前端再 `npm install`、api 跑 `go mod download`。

### refcode-api

```bash
cd refcode-api
cp .env.example .env      # 只有第一次！會蓋掉已經填好的 .env
                          # DATABASE_URL 填 Supabase 的 session pooler 字串，JWT_SECRET 也要看過
go mod download           # 可略，go run 會自己補
make migrate-up           # 建表，套用前先自己看過 SQL；已是最新就什麼都不做
go run ./cmd/api          # 前景啟動，port 由 .env 的 HTTP_ADDR 決定
```

起不來的兩個常見原因（細節見 `refcode-api/README.md`）：`.env` 被 `cp` 蓋成佔位字串、
或 7802 還被上一隻佔著（`./dev.sh stop api`）。後者的 log 會連帶噴一行資料庫
`operation was canceled`，那是症狀不是原因。

其他常用指令：

```bash
go build -o bin/api ./cmd/api   # 產出單一執行檔
go test ./...
go vet ./...
gofmt -w .
```

這幾個 `Makefile` 有包一層（`make build` / `test` / `vet` / `fmt`），跑哪一邊都一樣。
下面兩個沒有 go 原生的等價指令，一定要走 make：

```bash
make migrate-up      # 套用 db/migrations/，套用前先自己看過 SQL
make migrate-status  # 看目前套到哪一版
make sqlc            # 改完 db/queries/*.sql 一定要跑，重產 internal/store/dbgen/
```

`migrate-*` 需要 `goose`、`make sqlc` 需要 `sqlc`，兩個都不是 `go mod` 的相依，要另外裝。
goose 不讀 `.env`，Makefile 會自己從 `.env` 撈 `DATABASE_URL`，所以連線字串只維護一份。

### refcode-admin

內部後台（Vue 3 + Vite + Naive UI）。

```bash
cd refcode-admin
cp .env.example .env  # VITE_API_BASE_URL，預設 http://localhost:7802
npm install
npm run dev           # http://localhost:5173
npm run build         # vue-tsc 型別檢查 + vite build，輸出 dist/
npm run preview       # 用 build 產物起一台本機 server
```

port 寫在 `vite.config.ts` 的 `server.port`，改了要同步 `refcode-api/.env` 的 `CORS_ORIGINS`。

**登入需要 admin 帳號**，一般使用者的帳號進不來（後端的 `users` 和 `admins` 是兩張表）。
第一個帳號由後端的 seed 建立：

```bash
cd ../refcode-api && make seed EMAIL=admin@local.test PASSWORD=admin12345
```

進去之後：`/review` 是審核佇列（reviewer 以上），`/merchants` 和 `/categories` 只有 owner
看得到。**後台不發 refresh token**，access token 過期（預設 15 分鐘）就會被踢回登入頁，
這是後端刻意的設計，不是 bug。

### refcode-app

手機 app（Vue 3 + Ionic + Capacitor）。

```bash
cd refcode-app
cp .env.example .env  # VITE_API_BASE_URL，預設 http://localhost:7802
npm install
npm run dev           # http://localhost:5174，瀏覽器就能開發
npm run build         # vue-tsc 型別檢查 + vite build，輸出 dist/
npm run preview       # 用 build 產物起一台本機 server
```

port 寫在 `vite.config.ts`，改了一樣要同步後端的 `CORS_ORIGINS`。

不用登入就能瀏覽和複製推薦碼，只有「我的推薦碼」和上架需要帳號——直接在 app 裡註冊即可。

**原生平台都加好了**（appId `com.referra.app`），Android 已經驗過能出 debug APK。

```bash
npm run build && npx cap sync     # 每次改完前端都要 sync 一次
npx cap open ios                  # 開 Xcode
npx cap open android              # 開 Android Studio

要做成 hot reload
# Terminal A：dev server（--host 一定要加，見下面第 2 點）
cd refcode-app && npm run dev -- --host

# Terminal B（`--host localhost` 不能省，見下面第 3 點）
cd refcode-app && npx cap run ios -l --host localhost --port 5174

```

加了原生平台之後，**實機和模擬器連不到 `localhost:7802`**——那個 localhost 是裝置自己。
把 `.env` 的 `VITE_API_BASE_URL` 改成電腦的區網 IP（`ipconfig getifaddr en0`），
並把同一個位址加進後端的 `CORS_ORIGINS`。

`cap run -l` 不給 `--host` 的話，Capacitor 會自己抓區網 IP 當 live reload 位址，
WebView 的 origin 就變成 `http://<區網IP>:5174`——那個 origin 不在後端的
`CORS_ORIGINS` 裡，API 全部會被擋掉，但畫面照樣載得出來，看起來像「後端掛了」。
釘成 `localhost` 就沿用已經放行的 origin。**上面兩個 Terminal 其實不用自己開，
`./dev.sh ios` 已經是這條指令**（會順便起 vite、跑 `cap sync`、結束時還原原生設定）。

Google / Apple 登入已經接上 `@capgo/capacitor-social-login`，但**還沒有任何 client id**，
所以那兩顆按鈕預設不會出現。要開啟就填 `.env`（app 端）與 `GOOGLE_CLIENT_IDS` /
`APPLE_CLIENT_IDS`（後端），兩邊必須是同一組。

上架 App Store / Play Store 要用的文件全部在 `refcode-app/store/`，
**目前還有阻斷項沒解決**（帳號刪除功能、政策網址、UGC 要件），先讀 `refcode-app/store/README.md`。

### refcode-web

官網（Nuxt 4，SSR）。這是唯一吃自然搜尋流量的模組。

```bash
cd refcode-web
cp .env.example .env  # NUXT_PUBLIC_API_BASE、NUXT_PUBLIC_SITE_URL
npm install
npm run dev           # http://localhost:3000
npm run build         # 輸出 .output/
npm run preview       # 用 build 產物起 server，驗 SSR 真的有 render
```

port 是 Nuxt 預設的 3000，要改的話 `npm run dev -- --port 3100`，一樣記得同步
後端的 `CORS_ORIGINS`。

跑起來之後值得看的幾頁：`/`（分類 + 服務商）、`/referral/<slug>`（服務商頁，站的重點）、
`/login`、`/register`（email + 密碼，另有 Google）、`/sitemap.xml`（從 API 動態產生，不是靜態檔）。

Google 登入的按鈕要填了 `NUXT_PUBLIC_GOOGLE_CLIENT_ID` 才會出現，沒填就只有 email + 密碼，
本機開發不影響。取得 client id 的步驟在 `refcode-app/README.md`（官網和 app 共用同一個）。

驗 SSR 有沒有真的生效——直接看 HTML 裡有沒有推薦碼，不要只看瀏覽器畫面：

```bash
curl -s http://localhost:3000/referral/demo-broker | grep -o "INVEST-000[0-9]"
```

**`ssr: true` 不要關掉**，也不要改用 `nuxt generate`。服務商頁的推薦碼是即時的，
預先產生的靜態頁會拿到過期的碼，而且排序是每次請求重抽的。

## 正式環境怎麼跑

`dev.sh` 只給本機開發用，正式環境跑的是 build 產物：

| 模組 | build | 跑起來 |
|---|---|---|
| api | `make build` | `./bin/api`（單一執行檔，不需要 Go） |
| admin | `npm run build` | `dist/` 是純靜態檔，丟給任何 static server |
| app | `npm run build` | `dist/` 同上；或用 Capacitor 包成原生 app |
| web | `npm run build` | `node .output/server/index.mjs`（Nitro，需要 Node） |

api 的 binary 一樣讀環境變數，`.env` 只是本機的預設值。上線前一定要換掉的：

```bash
APP_ENV=production
JWT_SECRET=$(openssl rand -base64 48)   # .env.example 那組是給本機用的
DATABASE_URL=...
CORS_ORIGINS=https://你的官網,https://你的後台網域
REDIS_HOST=...                          # 託管的 redis 記得一起打開 REDIS_TLS=true
SMTP_HOST=...                           # 留空的話 APP_ENV=production 會直接拒絕啟動
MAIL_FROM=no-reply@你的網域
```

`SMTP_HOST` 留空時信不會寄出去、只印進 log。那在本機是方便（驗證碼直接看得到），
在正式環境是災難，所以 `config.Load` 會在 `APP_ENV=production` 且沒設時擋下啟動。

web 還要改 `NUXT_PUBLIC_SITE_URL` 和 `public/robots.txt`——兩個都還寫著 localhost，
sitemap 會跟著產出錯的網址。

**`make seed` 不要在正式環境跑**，它會塞 demo 服務商。`APP_ENV=production` 時它會直接拒絕執行。

## 硬規則

- **這台 Postgres 上還有別的 side project，資料一律放 `referral_code_bonus` schema。**
  所有 SQL 明確寫 `referral_code_bonus.xxx`，不要依賴連線的 `search_path`。
- **migration 套用前自己看過。** `refcode-api/db/migrations/` 是純 SQL。
- **改完 `db/queries/*.sql` 要跑 `make sqlc`**，`internal/store/dbgen/` 是產生出來的，不要手改。
- **四個 repo 之間唯一的耦合是 API 契約。** 三個前端各有一份手寫的 `types.ts`，
  後端補上 OpenAPI spec 之後應該全部改成從 spec 產生 —— 這是分開 repo 最容易爛掉的地方。
- 規則只寫一個地方：排序權重在 `refcode-api/internal/ranking/ranking.go`、
  品質分數與自動下架在 `ranking/quality.go`、審核動作對應狀態在 `handlers_admin.go`。
  要改行為先找那一處，不要在呼叫端各自判斷。

## 目前進度

Phase 1（目錄、上架、審核、排序）與 Phase 2（回報、自動下架、到期下架）的後端與三個前端都完成了。

忘記密碼（6 位數驗證碼 + redis + SMTP）後端與 app、官網都完成了，
細節見 `refcode-api/README.md` 的「忘記密碼」一節。

還沒做：
- OpenAPI spec 輸出
- email 驗證信（註冊當下那封；`email_verified_at` 目前只有重設密碼會標）
- 使用者自己下架 / 刪除已上架的推薦碼
- 封鎖 / 檢舉上架者（Apple 1.2 的 UGC 要件，最後一項沒補的）
- 各平台的 OAuth client id 與 RevenueCat 的 API key（程式碼都接好了，缺設定）
- Phase 3 全部：CPC 競價、廣告主錢包、金流、點擊計費防作弊
# referral-code-bonus



測試
admin 帳密
- Email：owner@refcode.test
- 密碼：refcode1234
- 權限：owner（review、merchants、categories 都能進）

