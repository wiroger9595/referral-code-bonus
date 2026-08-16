# 推薦碼媒合平台 — 規劃書

## 一、產品定義

平台本身**不發獎勵、不碰獎勵金流**。獎勵由服務商（銀行、券商、電商、訂閱制 App…）那端各自發給雙方，平台只做「推薦碼的供需媒合 + 曝光排序」。

三種角色：

| 角色 | 想要什麼 | 平台提供 |
|---|---|---|
| 上架者（推薦人） | 自己的碼被更多人用 | 上架、看數據、出價買排序 |
| 找碼者 | 註冊時找一個能用的碼 | 目錄、搜尋、可信度標示、一鍵複製 |
| 平台（你） | 流量與廣告收入 | CPC 競價、服務商目錄維護、審核 |

**收入模型**：CPC 競價。上架者預存餘額、對自己的碼出價，點擊時扣款，出價高者排前面。

> ⚠️ 這裡有個要先講清楚的：選 CPC 等於平台必須做**廣告主餘額錢包 + 儲值金流 + 計費對帳**。雖然「獎勵」不涉及金流，但廣告費涉及。這是整個系統最重的一塊，也是唯一有真實金錢損失風險的部分（重複計費、作弊點擊、餘額對不上）。規劃已按此設計，但**建議 Phase 1 先不上競價**（見第七節里程碑），先把流量做起來——沒流量的點擊沒人願意出價。

---

## 二、系統架構

四個獨立 repo：

```
refcode-api          Go        REST API + 排序引擎 + 計費
refcode-app          Vue 3 + Ionic + Capacitor    iOS / Android
refcode-admin        Vue 3 + Vite (SPA)           內部後台
refcode-web          Nuxt 3 (SSR)                 官網 / SEO 目錄頁
```

四個 repo 之間唯一的耦合是 API 契約，因此**後端要輸出 OpenAPI spec，三個前端從 spec 產型別**（`openapi-typescript`）。獨立 repo 最容易爛掉的地方就是型別漂移，這一步不能省。

### 為什麼 web 用 Nuxt 而不是純 Vue SPA

這類平台的命脈是自然搜尋流量——「oo 銀行 推薦碼」這種長尾關鍵字。每個服務商都該有一頁可被索引的 SSR 頁面。純 SPA 在這件事上先天吃虧。Nuxt 3 本身就是 Vue 3，不違背你的技術選型。

app 與 admin 不需要 SEO，維持 SPA 即可。

---

## 三、技術選型

### 後端（Go）

| 用途 | 選型 | 理由 |
|---|---|---|
| HTTP router | `chi` | 標準 `net/http` 介面相容，中介層寫法乾淨，不綁框架生態 |
| DB 存取 | `pgx` + `sqlc` | 從 SQL 產生 type-safe Go code，不用 ORM。這類系統的排序查詢會很客製，ORM 只會擋路 |
| Migration | `goose` | 純 SQL 檔，好人工審視 |
| 快取 / 計數 | Redis | 加權隨機輪播的候選池、點擊去重、預算即時扣減 |
| 排程 | `river` 或 cron worker | 到期碼下架、日預算重置、對帳 |
| Auth | `golang-jwt` 自建 | access token 15 min + refresh token 30 天（rotating） |
| 設定 | env + `koanf` | — |
| 日誌 | `slog`（標準庫） | — |

### 前端共用

- Vue 3 `<script setup>` + TypeScript + Vite
- Pinia（狀態）、Vue Router
- TanStack Query for Vue（server state / 快取 / 重試）
- UI：app 用 Ionic Vue（原生感 + Capacitor 官方整合）、admin 用 Naive UI、web 用 Tailwind 自刻

### Capacitor plugins

`@capacitor/clipboard`（複製碼，核心動作）、`@capacitor/share`、`@capacitor/preferences`、`@capacitor/push-notifications`、`@capacitor-firebase/authentication`（Google / Apple 登入）、`@capacitor/browser`（開服務商註冊頁）。

---

## 四、資料模型

Postgres 獨立 schema：`refcode`。**所有查詢明確指定 schema，不依賴 `search_path`**（同一台機器上還有別的 side project）。

### 目錄與內容

```sql
merchants              -- 服務商目錄，只有你能新增（使用者不能自建，避免重複與亂填）
  id, slug, name, category_id, logo_url, signup_url,
  reward_desc,         -- 「雙方各得 500 元」這類說明，你維護
  code_format_regex,   -- 用來擋掉明顯亂填的碼
  is_active, created_at

merchant_categories
  id, slug, name, sort_order

referral_codes
  id, user_id, merchant_id,
  code,                -- 大小寫敏感，照原樣存
  note,                -- 上架者補充說明
  status,              -- pending / active / rejected / expired / disabled
  expires_at,          -- 必填。上架者自訂有效期
  quality_score,       -- 0~100，由回報數據算出，供排序用
  created_at, activated_at

  UNIQUE (user_id, merchant_id) WHERE status IN ('pending','active')
  -- 一人一家服務商只能有一個生效中的碼，防洗榜
```

### 品質控管

```sql
code_reviews           -- 人工審核軌跡
  id, code_id, admin_id, action, reason, created_at

code_reports           -- 使用者回報
  id, code_id, reporter_id, device_hash,
  result,              -- worked / failed / invalid_code / merchant_closed
  created_at
  UNIQUE (code_id, reporter_id)
```

三道關卡的關係：**人工審核**擋上架時的垃圾、**有效期**擋自然老化、**使用者回報**擋審核後才失效的碼。三者疊起來才夠——單靠人工審核擋不住「碼上架時有效、兩週後服務商停辦活動」。

自動下架規則（可調參數，寫在設定不寫死）：近 10 筆回報中 failed 佔比 ≥ 60% 且 failed 絕對數 ≥ 3 → 自動轉 `disabled` 並通知上架者申訴。

### 事件與計費

```sql
code_events            -- 曝光 / 點擊 / 複製，分區表按日切
  id, code_id, event_type, user_id, device_hash, ip_hash,
  is_billable, created_at

ad_campaigns
  id, code_id, bid_amount_cents, daily_budget_cents,
  status, starts_at, ends_at

ad_wallets
  user_id, balance_cents, reserved_cents, updated_at

wallet_transactions    -- 只寫不改，餘額是這張表的 sum
  id, user_id, type,   -- topup / click_charge / refund / adjustment
  amount_cents, ref_type, ref_id, created_at

payments               -- 儲值訂單（金流商回呼寫這裡）
  id, user_id, provider, provider_txn_id, amount_cents,
  status, raw_payload, created_at
```

`wallet_transactions` 用 append-only 帳本、餘額只當快取欄位——廣告費對帳出錯時，帳本是唯一真相。

---

## 五、排序引擎（核心）

同一個服務商頁面，碼的排列分兩區：

```
[ 競價區 ]  最多 3 個  ← CPC 出價排序，明確標示「推廣」
[ 自然區 ]  其餘全部   ← 加權隨機輪播
```

### 競價區

`effective_bid = bid_amount × quality_score_factor`

純比出價會讓爛碼靠錢買到頂位，砸掉整個平台的信任。乘上品質因子後，回報成功率高的碼用較低出價也能排前，這也是 Google Ads 的作法。

出價前要先檢查：餘額 > 出價、當日預算未用罄、碼是 `active`。

### 自然區加權隨機輪播

每次請求重新抽序，權重：

```
weight = quality_score × recency_decay × (1 / (1 + 已曝光次數/100))
```

第三項是關鍵——已經被大量曝光的碼權重下降，讓新上架的人也拿得到曝光。否則新用戶永遠埋底，就不會有人願意來上架。

實作：候選池放 Redis sorted set，每 5 分鐘由 worker 重算權重，請求時用 weighted sampling 取 N 個。不要每個 request 打 DB 算權重。

### 點擊計費防作弊

計費前必須通過：

1. **去重視窗**：同一 `device_hash + code_id` 在 24h 內只計費一次（Redis SETNX + TTL）
2. **IP 頻率**：同一 IP 對同一 campaign 每小時上限
3. **曝光前置**：沒有對應曝光事件的點擊不計費（擋直接打 API 刷點擊）
4. **停留驗證**：點擊後導向服務商註冊頁，未達最低停留時間的不計費
5. **預算即時扣減**：Redis 原子扣減，用罄立即退出競價區；DB 帳本非同步落地

所有點擊都寫 `code_events`，但 `is_billable` 分開標記——被判為無效的點擊要留紀錄，廣告主質疑帳單時要拿得出來。

---

## 六、認證設計

三種登入並存，統一收斂到同一個 `users`：

```sql
users
  id, email, email_verified_at, display_name, avatar_url,
  password_hash,       -- 只有 email 註冊者有值
  status, created_at

oauth_identities
  id, user_id, provider,   -- google / apple
  provider_user_id, created_at
  UNIQUE (provider, provider_user_id)
```

要點：

- **Apple 登入必做**。App Store 規則：有第三方登入就必須提供 Apple 登入。
- Apple 的 private relay email（`@privaterelay.appleid.com`）要能正常收信，別做網域白名單。
- 同 email 用不同方式登入時要合併帳號——先驗證 email 已驗證過才自動綁定，否則會有帳號接管漏洞。
- Refresh token rotating + 重用偵測（同一 token 用第二次 → 該裝置所有 token 全撤銷）。
- **匿名瀏覽**：找碼者不登入就能看碼、能複製。只有「上架碼」和「回報」需要登入。這對轉換率和 SEO 都關鍵。

---

## 七、里程碑

### Phase 1 — 目錄與媒合（4~6 週）

沒有流量的平台賣不掉點擊。這階段完全不做競價。

- Go API：服務商目錄、上架、審核、搜尋、排序（只做自然區加權隨機）
- admin：服務商 CRUD、推薦碼審核佇列
- web（Nuxt）：首頁、分類頁、服務商頁（SSR + structured data + sitemap）
- app：瀏覽、搜尋、複製碼、上架、我的推薦碼
- 認證三種方式、事件追蹤（曝光/點擊/複製先只記錄不計費）

### Phase 2 — 信任機制（2~3 週）

- 使用者回報流程（複製後延遲追問「這個碼能用嗎」）
- `quality_score` 計算 + 自動下架 + 申訴
- 到期自動下架排程
- app 上架 App Store / Play Store

### Phase 3 — CPC 變現（4~5 週）

- 錢包、儲值金流串接、帳本
- campaign 出價與預算
- 競價區排序 + 點擊計費 + 五道防作弊
- 上架者數據儀表板（曝光/點擊/花費/CTR）
- admin：帳務對帳、退款、異常點擊審查

### Phase 4 — 成長

推播（追蹤的服務商有新碼）、推薦人排行榜、瀏覽器擴充功能、服務商官方合作方案。

---

## 八、待你決定的事

1. **金流商**：台灣的話綠界 / 藍新 / TapPay。若 app 內販售會被 Apple 認定為數位商品而抽 30%——**廣告儲值只在 web 做，app 內不放購買入口**，這點在 Phase 3 前要確定。
2. **服務商目錄從哪來**：初期要人工建幾十家（銀行信用卡、券商、外送、串流、電商）。這是冷啟動的主要工作量，跟寫 code 無關但更關鍵。
3. **法遵**：金融業推薦碼的推廣在台灣有廣告揭露規範，Phase 3 上線前要確認。
4. **`quality_score` 初始值**：新碼沒有回報資料時給多少？建議 60（中間偏上），讓新人有曝光但不壓過已驗證的碼。

---

## 九、進度

Phase 1 與 Phase 2 的後端與三個前端都做完了，四個 repo 都能跑起來（`./dev.sh all`）。
細節見根目錄 README 與各 repo 的 README。

實作過程中偏離本規劃的地方：

- **config 沒用 koanf**，純 `os.Getenv` 加一個小 parser。這規模用不到。
- **沒引入 TanStack Query**。web 用 Nuxt 內建的 `useFetch`，admin 與 app 頁面數少，
  直接寫 composable 更直接。
- **事件表做成月分區而非日分區**。初期量用月切就夠，維護成本低很多。
- **多做了爬蟲曝光過濾**。官網是 SSR 的，搜尋引擎每爬一次就是一整批曝光，
  會直接稀釋排序用的曝光懲罰 —— 這在原規劃裡沒想到。

下一步建議：先補 **OpenAPI spec 輸出**（三個前端現在各有一份手寫型別，遲早分岔），
再看是要接 email 驗證信，還是直接進 Phase 3。
