# refcode-admin

推薦碼媒合平台的內部後台。Vue 3 + Vite + TypeScript + Naive UI。

需要 `refcode-api` 跑在 `http://localhost:7802`。

平常用工作區根目錄的 `./dev.sh admin`。要在這個目錄裡單獨開發：

```bash
cp .env.example .env
npm install
npm run dev          # http://localhost:5173
```

第一個 admin 帳號由後端的 seed 指令建立：

```bash
cd ../refcode-api && make seed EMAIL=admin@local.test PASSWORD=admin12345
```

## 頁面

| 路由 | 權限 | 說明 |
|---|---|---|
| `/login` | — | 後台登入 |
| `/review` | reviewer | 審核佇列：核准 / 拒絕待審的推薦碼 |
| `/merchants` | owner | 服務商目錄維護 |
| `/categories` | owner | 分類：新增／編輯／刪除 |

## 幾個要知道的

**後台沒有 refresh token。** 後端刻意不發，session 過期就得重登，減少長效憑證外流的面。
`client.ts` 收到 401 會清掉 token 並把人導回登入頁。

**選單的權限判斷只是 UX。** 真正的權限在後端 —— reviewer 打 owner 的 API 一樣會被擋。
不要把前端的 `meta.ownerOnly` 當成安全機制。

**服務商列表走 `/v1/admin/merchants` 而不是公開那支。** 公開的會過濾掉停用的服務商，
而且缺 `category_id`、`code_format_regex` 這些編輯表單要用的欄位。

**服務商的「適用國家」留空是有意義的**，代表不分地區（串流、雲端這種跨國服務），
排序時排在在地服務商後面、外地服務商前面。填了就只有那些國家的使用者會優先看到它。
選單上沒有的國家可以直接打 ISO 3166-1 alpha-2 代碼進去，後端只驗格式。

**拒絕和下架一定要填原因。** 原因會寫進 `code_reviews`，使用者申訴時要拿得出當初的判斷依據。

**分類沒有 slug**，一律用 id：分類頁的網址是 `/category/{id}`，`?category=` 篩選也是 id。

**服務商的 slug 改得動，但改了舊網址就死了**——沒有轉址。`/referral/{slug}` 是官網
吃自然搜尋的主力頁，改 slug 等於放棄那頁已經累積的排名，沒必要不要動。

**刪分類前先確認底下沒有服務商。** `merchant_categories` 沒有 `ON DELETE CASCADE`，
還有服務商掛著會被後端擋下來（`category_in_use`），不會意外把整批服務商連坐刪掉，
但錯誤訊息只會提醒你去改服務商的分類，不會列出是哪幾家——列表本身就在 `/merchants` 頁。

## 型別

`src/api/types.ts` 是手寫的，對應後端的回傳。後端補上 OpenAPI spec 之後應該改成
從 spec 產生（`openapi-typescript`），否則兩邊遲早分岔 —— 這是四個模組分開最容易爛掉的地方。
