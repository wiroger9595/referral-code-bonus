# refcode-api

推薦碼媒合平台的後端。Go + Postgres。產品規劃見上一層的 `PLAN.md`。

目前完成 Phase 1（目錄、上架、審核、排序）與 Phase 2 的信任機制（回報、自動下架、到期下架）。
CPC 競價與金流是 Phase 3，還沒開始。

## 起手式

**資料庫是 Supabase，本機開發也連同一台**，不用裝 local Postgres、也沒有 `createdb` 這一步。
連線字串在 Dashboard → Project Settings → Database → Connection string → Session pooler。

**下面前三行都只有第一次要跑**，已經設定過的機器直接跳到 `go run ./cmd/api`。

```bash
cp .env.example .env          # 只有第一次！.env 已經存在的話這行會蓋掉你填好的設定
                              # 填 DATABASE_URL（Supabase session pooler）與 JWT_SECRET
make migrate-up               # 已經是最新版就什麼都不做，可以安心重跑
make seed EMAIL=admin@local.test PASSWORD=admin12345   # 建 admin + demo 服務商，只給本機
go run ./cmd/api
```

已經跑過一輪還照抄的話，會踩到這三個：

- **`cp .env.example .env` 會直接覆蓋。** `.env.example` 裡的 `DATABASE_URL` 是
  `<project-ref>`、`<密碼>` 佔位字串，蓋掉之後 API 起來會報 `hostname resolving error`。
- **`make seed` 第二次跑一定失敗**：admin 已存在時 `cmd/seed` 是 `log.Fatal` 退出 1，
  而且 admin 那步在前面，連 `-demo` 的分類與服務商都不會塞。要重塞 demo 資料就換一個 email。
- **`go run ./cmd/api` 會撞 `bind: address already in use`**，上一隻沒關乾淨。
  先 `../dev.sh stop api`。這種情況的 log 會**同時**出現一行資料庫
  `operation was canceled`——那是 bind 失敗後 ctx 被取消的連帶結果，不是資料庫真的連不上，
  不用去查 DNS。

日常開發用 go 原生指令就好：`go run ./cmd/api`、`go build -o bin/api ./cmd/api`、`go test ./...`、
`go vet ./...`、`gofmt -w .`。`Makefile` 對這幾個只是包一層（`run` `build` `test` `vet` `fmt`）。

`sqlc` `migrate-up` `migrate-down` `migrate-status` `seed` 沒有等價的 go 指令，走 make——
goose 不讀 `.env`，Makefile 會自己從 `.env` 撈 `DATABASE_URL`，連線字串才只維護一份。

## 硬規則

- **這台 Postgres 上還有別的 side project，資料一律放 `referral_code_bonus` schema。** 所有 SQL 明確寫
  `referral_code_bonus.xxx`，不要依賴連線的 `search_path`。
- **改完 `db/queries/*.sql` 要跑 `make sqlc`**，`internal/store/dbgen/` 是產生出來的，不要手改。
- **migration 套用前自己看過**。`db/migrations/` 是純 SQL，goose 格式。
- **`cmd/seed` 只給本機**，`APP_ENV=production` 時會直接拒絕執行。
- 規則只寫一個地方：排序權重在 `internal/ranking/ranking.go`、品質分數與自動下架在
  `internal/ranking/quality.go`、審核動作對應狀態在 `handlers_admin.go` 的 `reviewActionStatus`。
  要改行為先找那一處，不要在呼叫端各自判斷。

## 架構

```
cmd/api        進入點
cmd/seed       本機 seed
internal/
  config       env 設定（.env 只當預設值，環境變數優先）
  store        pgxpool + sqlc 產出
  auth         JWT、bcrypt、Google/Apple OIDC 驗證
  ranking      排序權重與品質分數
  httpapi      路由、middleware、handler
  worker       到期下架、事件表分區補建
db/
  migrations   goose
  queries      sqlc 來源
```

## API

`Authorization: Bearer <access_token>`。錯誤一律是
`{"error":{"code":"...","message":"..."}}`。

**`code` 是前端唯一可靠的判斷依據，一個 code 對應一句話。** message 是中文，
app 與官網的日文／英文介面沒辦法直接顯示，它們拿 code 去查自己的語系檔。
所以不要讓幾十種情況共用一個 `conflict` —— 新增錯誤時在
`internal/httpapi/response.go` 一起加一個常數，並回頭補 `refcode-web` 與
`refcode-app` 各三份語系檔的 `errors.*`。後台專用的 code（`slug_taken`、
`review_*` 之類）不用翻，admin 是單一語言。

### 認證
| Method | Path | 說明 |
|---|---|---|
| POST | `/v1/auth/register` | email + 密碼註冊 |
| POST | `/v1/auth/login` | email + 密碼登入 |
| POST | `/v1/auth/oauth` | `{provider: google\|apple, id_token}` |
| POST | `/v1/auth/refresh` | 換發，會旋轉 refresh token |
| POST | `/v1/auth/logout` | 撤銷整個 token family |
| POST | `/v1/auth/password/forgot` | `{email, locale}`，寄驗證碼。永遠回 204 |
| POST | `/v1/auth/password/reset` | `{email, code, password}`，成功直接發新 token |
| POST | `/v1/admin/login` | 後台登入，不發 refresh token |

### 瀏覽（匿名可用）
| Method | Path | 說明 |
|---|---|---|
| GET | `/v1/categories` | 分類列表 |
| GET | `/v1/categories/{id}` | 單一分類，分類頁拿它顯示名稱 |
| GET | `/v1/merchants` | `?category=&q=&limit=&offset=`，category 是分類 id |
| GET | `/v1/merchants/{slug}` | 服務商 + 排序後的推薦碼，同時記錄曝光。未登入時 `codes[].code` 是 `null`（見下） |
| GET | `/v1/merchants/sitemap` | slug + updated_at，給 Nuxt 產 sitemap |
| POST | `/v1/events` | `{code_id, event_type: click\|copy}` |
| POST | `/v1/codes/{id}/reports` | `{result: worked\|failed\|invalid_code\|merchant_closed}` |

### 需登入
| Method | Path | 說明 |
|---|---|---|
| GET/PATCH | `/v1/me` | 個人資料 |
| GET | `/v1/me/codes` | 我上架的碼 |
| POST | `/v1/codes` | 上架，進 `pending` 等審核 |
| GET | `/v1/codes/{id}/stats` | `?days=30`，只有本人看得到 |

### 後台
| Method | Path | 權限 |
|---|---|---|
| GET | `/v1/admin/codes/pending` | reviewer |
| POST | `/v1/admin/codes/{id}/review` | reviewer，`{action: approve\|reject\|disable\|restore, reason}` |
| POST | `/v1/admin/categories` | owner |
| PATCH | `/v1/admin/categories/{id}` | owner |
| DELETE | `/v1/admin/categories/{id}` | owner，還有服務商掛著會回 409 `category_in_use` |
| POST | `/v1/admin/merchants` | owner，`category_id` 指到不存在的分類回 400 `category_not_found` |
| PATCH | `/v1/admin/merchants/{id}` | owner，slug 可改（舊網址不轉址），`category_id` 同上 |

## 幾個容易誤解的設計

**分類只有 id，服務商才有 slug。** 分類頁的網址是 `/category/{merchant_category_id}`，
`?category=` 也只收 id ——分類的 slug 欄位在 00007 已經刪掉了。服務商的 slug 留著，
`/referral/{slug}` 是官網吃自然搜尋的主力頁。

**改服務商的 slug 不會留轉址。** 舊網址直接 404，搜尋排名要重新累積。
後台的表單有提示，但沒有機制擋 —— 要改之前想清楚。

**要註冊才能拿到推薦碼。** `handleGetMerchant` 檢查 `auth.UserID(ctx)`，沒登入時
`codeItem.Code` 是 `nil`、`Masked` 是 `true`，其餘欄位（家數、評價、備註、分享者）
照常回傳——服務商頁本身仍完全公開、值得被搜尋引擎收錄，只有碼字面值被擋。
`optionalUser` 只驗簽章不查資料庫，跟 `viewerCountry`／`recordImpressions` 用的是
同一個判斷慣例，不用為此另外查一次使用者是否存在。前端（app、web）只照著
`masked` 欄位切 UI，判斷邏輯只在這一處，不要在前端重複判斷登入狀態。

**曝光不收 client 端上報。** `impression` 只在伺服器決定要顯示哪些碼時寫入
（`handleGetMerchant`）。開放給 client 送等於開放灌水，Phase 3 的點擊計費會直接被搞爛。
`click` 和 `copy` 是 client 才知道的，才由 `/v1/events` 收。

**排序每次請求重抽。** 同一個服務商頁連刷兩次順序會不一樣，這是刻意的
（加權隨機輪播），不是 bug。權重 = 品質分數 × 新鮮度加成 × 曝光懲罰，
曝光懲罰讓已經拿到大量曝光的碼把位置讓出來，否則新上架的人永遠排不進去、供給端會枯竭。

**回報允許匿名。** 真正試過碼的人大多沒註冊，限制在登入者身上就拿不到資料。
去重靠 `X-Device-ID`（client 安裝時產生的 UUID），沒帶的話退回 IP + UA。

**社群登入不會自動合併未驗證的帳號。** 同 email 只有在雙方 email 都驗證過時才自動綁定，
否則回 409 要求先用密碼登入 —— 不然「用別人的 email 註冊 → 對方用 Google 登入」就是帳號接管。
目前註冊還沒接驗證信，所以 email 註冊的帳號一律走 409 那條路。

**refresh token 是 rotating 的。** 舊 token 再次出現代表外洩，整個 family 立刻撤銷、
該裝置必須重新登入。

**目錄的地區優先是排序不是過濾。** `users.country`（註冊時自己選）對得上
`merchants.countries` 的排最前面，`countries` 是空的（串流、雲端這種跨國服務）排中間，
對不上的排最後 —— 但三種都看得到。規則只寫在 `db/queries/merchants.sql` 的
`ListMerchants` ORDER BY 裡，不要在 handler 再排一次。

**沒登入就沒有地區排序。** `viewer_country` 是 NULL 時 ORDER BY 的 CASE 全部落在同一層，
等於退回原本的排序。這是刻意的：官網的服務商頁靠自然搜尋吃流量，
匿名訪客（含爬蟲）拿到的內容不能因人而異。

**所在地不用 IP 也不用介面語言判定。** 旅居海外的人語言跟所在地常常不一樣，
IP 碰到 VPN／行動網路會猜錯，兩種都沒辦法讓使用者自己修正。前端的選單只是常用選項，
後端只驗格式（`internal/geo`），沒列在選單裡的國家一樣存得進去。

## 忘記密碼

6 位數驗證碼，不是重設連結 —— app 是 Capacitor 殼，信裡的連結點開會落到瀏覽器，
要讓它回到 app 得先設定 iOS associated domains 與 Android app links。驗證碼沒這個問題。

```
POST /v1/auth/password/forgot  {email, locale}          → 204
POST /v1/auth/password/reset   {email, code, password}  → user + tokens
```

**驗證碼與寄送次數在 redis，不在 Postgres。** 兩者都是短命、到期就該消失的東西，
進資料庫只會多一張要自己清的表。key 是 `pwreset:code:<email>` 與 `pwreset:sends:<email>`。

**三道限制缺一不可**（`PASSWORD_RESET_*`）：6 位數只有一百萬種組合，光靠雜湊擋不住暴力猜。
碼 10 分鐘過期、同一組碼猜錯 5 次就作廢、每小時最多索取 5 組。

**redis 存的是 HMAC 不是 sha256**，金鑰用 `JWT_SECRET`。純 sha256 的話 redis 內容一外流，
一百萬種組合幾秒就跑完。

**`forgot` 不管 email 有沒有註冊過都回 204**，跟 login 不區分帳號與密碼錯誤是同個理由。
但「redis 連不上」要先於這條判斷（`ResetService.Available()`），否則服務不可用會被蓋成 204，
使用者只會看到一個永遠等不到的驗證碼。

**重設成功會撤銷這個人所有的 refresh token**，並把 `email_verified_at` 標起來 ——
收得到驗證碼就是證明了信箱所有權，這也是 `handleOAuthLogin` 自動合併帳號的前提。

**redis 連不上不會擋啟動**，只有這兩支路由回 503 `reset_unavailable`，其他功能照常。

**信件文案在 `internal/httpapi/mailtext.go`，三種語言。** 錯誤訊息是前端拿 code 查自己的
語系檔，但信是後端直接寄的，前端沒有機會翻譯，所以 `forgot` 要帶 `locale`。

**`SMTP_HOST` 留空時信不會寄出去，改成印進 log**（驗證碼直接看得到），本機開發不用架 SMTP。
`APP_ENV=production` 而這欄留空會在 `config.Load` 就擋下來 —— 安靜地不寄信比直接壞掉更糟。

## 還沒做

- email 驗證信（註冊當下的那封；`users.email_verified_at` 目前只有重設密碼會標）
- 推播通知
- Phase 3 的競價、錢包、金流、點擊計費防作弊
- OpenAPI spec 輸出（四個 repo 分開，前端要從 spec 產型別，這個要補）
