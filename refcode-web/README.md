# refcode-web

推薦碼媒合平台的官網。Nuxt 4（SSR）+ Tailwind 4。

需要 `refcode-api` 跑在 `http://localhost:7802`。

```bash
cp .env.example .env
npm install
npm run dev          # http://localhost:3000
```

## 為什麼是 Nuxt SSR 而不是 SPA

這個站的流量幾乎全部來自「oo 銀行 推薦碼」這類長尾搜尋。每個服務商都要有一頁
可被索引的 SSR 頁面，純 SPA 在這件事上先天吃虧。**不要為了省事把 `ssr` 關掉。**

## 路由

| 路由 | 說明 |
|---|---|
| `/` | 首頁：分類 + 所有服務商 |
| `/category/[slug]` | 分類頁 |
| `/referral/[slug]` | 服務商頁 —— 這一頁是整個站的重點 |
| `/about` | 平台怎麼運作 |
| `/login`、`/register` | 登入 / 註冊，`noindex`（見下方） |
| `/forgot-password` | 忘記密碼，`noindex, nofollow`。驗證碼在信裡，不在網址上 |
| `/sitemap.xml` | 從 API 動態產生（`server/routes/`） |

日文與英文在同樣的路由加語言前綴：`/ja/referral/[slug]`、`/en/referral/[slug]`。

## 多語（中／日／英）

`@nuxtjs/i18n`，語系檔在 `i18n/locales/{zh-TW,ja,en}.json`。

- **strategy 是 `prefix_except_default`，中文不加前綴。** 中文頁面已經被索引了，
  改成 `/zh-TW/...` 等於整站換網址。
- **`htmlAttrs` 的 lang、canonical、hreflang 全由 `app.vue` 的 `useLocaleHead()` 產生。**
  不要自己再寫一個 canonical，那會讓三種語言指向同一個網址、日英版直接不被收錄。
- **站內連結一律 `localePath('/xxx')`**，寫死 `to="/xxx"` 會把日英使用者踢回中文版。
- `detectBrowserLanguage.redirectOn` 是 `'root'`：只有首頁會依瀏覽器語言轉址。
  深層頁跟著轉的話，爬蟲拿到的內容會跟它請求的網址對不上。
- sitemap 每個頁面會出現三次（每種語言各一），而且每一筆都列出全部語言的
  `xhtml:link` alternate。

**服務商名稱、分類名、獎勵說明不翻譯**，那些是資料庫欄位，三種語言都顯示原文。

**錯誤訊息一律走 code。** `useApiError()` 拿後端回的 `error.code` 去查 `errors.*`，
查不到才退回後端那句中文（代表後端加了新 code 但語系檔還沒跟上）。
後端每個 code 都對應到單一句話，定義在 `refcode-api/internal/httpapi/response.go`
—— **後端加新 code 時這裡三份語系檔要一起補**。

`admin_required`、`owner_required`、`slug_*`、`review_*` 這幾個是後台專用，
官網碰不到，故意沒翻。

## 幾個要知道的

**SSR 時一定要轉發使用者的 header。** 曝光是後端在決定顯示哪些碼時記錄的，
SSR 階段打 API 的是 Nuxt server —— 不轉發 `user-agent` / `x-forwarded-for` 的話，
後端看到的全是 node 的，曝光會歸錯而且爬蟲也擋不掉。
見 `app/pages/referral/[slug].vue` 的 `useRequestHeaders`。

**後端會過濾爬蟲的曝光。** 搜尋引擎每爬一次服務商頁就是一整批曝光，
會直接稀釋排序用的曝光懲罰。這件事在後端做（`isBot`），前端不用管，
但改動 header 轉發時要記得它依賴那些 header。

**排序每次都不一樣是正常的。** 加權隨機輪播，不是 bug。

**複製之後才問「能不能用」。** 沒複製的人根本沒試過，問了只會收到雜訊。

## 登入

**瀏覽不需要登入，但要看到推薦碼的實際內容並複製需要註冊。** `/v1/merchants/{slug}`
對匿名訪客回的 `codes[].code` 是 `null`、`masked: true`——服務商頁本身仍完全公開
（家數、評價、備註都在），只有碼字面值被擋。`AuthPanel` 之類的公開頁面不要因此收緊，
擋碼的判斷在後端 `handleGetMerchant`，前端只是照著 `masked` 欄位切換 UI（見
`pages/referral/[slug].vue`），不要在前端自己重複判斷登入狀態。

**token 存 cookie，不是 localStorage。** `useAuth` 用 `useCookie` 讀寫，SSR 階段就知道有沒有
登入，header 才不會先 render 成未登入、hydrate 之後才跳成已登入。代價是 cookie 不能設
`httpOnly`（前端要自己讀來帶 `Authorization`），防護程度跟 localStorage 一樣，
要真正的 httpOnly 得在 `server/` 開一層 proxy 幫忙轉發。

**沒登入的訪客不會多打 API。** `plugins/auth.ts` 每次 SSR 都會 `restore()`，
但沒有 cookie 就直接跳過 —— 站上大多數流量是搜尋來的匿名訪客，不能為了他們多一次往返。

**access token 過期會自動換發一次再重試**（`useAuth` 的 `authedFetch`）。
refresh token 是 rotating 的，換發成功一定要把新的一組寫回 cookie，舊的再用一次
會被後端當成重用而撤銷整族。

**Google 登入用 Google Identity Services，官網沒有自己的 OAuth callback。**
Google 直接在瀏覽器裡回一個簽好的 ID token，丟給後端的 `/v1/auth/oauth` 驗（比對 `iss`／`aud`）。
`NUXT_PUBLIC_GOOGLE_CLIENT_ID` 沒設定就整顆按鈕不顯示，只留 email + 密碼——
半設定的按鈕按下去只會拿到看不懂的 Google 錯誤。設定方式見 `refcode-app/README.md`，
官網跟 app 共用同一個 web client id，記得把本站網址加進 Authorized JavaScript origins。

**所在地在註冊時選，之後官網改不了。** 選單是常用選項（`useCountries`），
名稱用 `Intl.DisplayNames` 跟著介面語言產生，不進語系檔。預設值依介面語言猜
（zh-TW→TW、ja→JP），猜錯使用者自己改 —— 語言不等於所在地，不要拿它直接當所在地存。
**官網沒有帳號設定頁，要改所在地目前只能在 app 的帳號分頁改。**

**目錄列表要帶 `authHeaders()`。** 後端靠 token 才知道這個人的所在地、要不要把在地服務商
排前面（`/v1/merchants` 是 optionalUser，token 過期只是拿到不分地區的排序，不會整頁失敗）。
沒登入的訪客拿到的排序跟以前一樣，SSR 內容不會因人而異。

**`/login` 與 `/register` 是 `noindex`。** 這兩頁對搜尋沒有價值，還會跟首頁搶字。

## 分類不存在時回 404

`/category/xxx` 找不到分類時 `throw createError({ statusCode: 404 })`，
不要改成顯示空列表 —— 那會產生一堆內容重複的可索引頁面。

## 部署前要改

`public/robots.txt` 和 `.env` 的 `NUXT_PUBLIC_SITE_URL` 都還指著 localhost。
