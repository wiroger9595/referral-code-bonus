# refcode-app

推薦碼媒合平台的手機 app。Vue 3 + Ionic Vue + Capacitor。

需要 `refcode-api` 跑在 `http://localhost:7802`。

```bash
cp .env.example .env
npm install
npm run dev          # http://localhost:5174（瀏覽器就能開發）
```

## 畫面

| 路由 | 說明 |
|---|---|
| `/tabs/explore` | 服務商列表、搜尋、分類篩選 |
| `/tabs/my-codes` | 我上架的碼與狀態（需登入） |
| `/tabs/account` | 登入 / 登出 |
| `/forgot-password` | 忘記密碼：寄驗證碼 → 輸入碼與新密碼，兩步同一頁 |
| `/merchant/:slug` | 推薦碼列表、複製、回報、前往註冊 |
| `/add-code` | 上架推薦碼（需登入） |

## 多語（中／日／英）

`vue-i18n`，語系檔在 `src/i18n/locales/{zh-TW,ja,en}.json`，設定在 `src/i18n/index.ts`。

- **第一次開啟跟著裝置語言**（`navigator.languages`），只認 `zh` / `ja` / `en` 前綴，
  其他一律給繁中。使用者手動選過之後就記在 Preferences 的 `refcode_lang`，不再跟著裝置跑。
- **`initLocale()` 要在 mount 之前 await**（`main.ts` 跟 `initTokens()` 一起），
  Preferences 是 async 的，晚一步讀回來畫面會先閃一次預設語言。
- 切換入口在「帳號」分頁，**登入與未登入都看得到** —— 看不懂介面的人多半還沒登入。
- `formatDate` 跟著當下語言走，不要寫死 `'zh-TW'`。

**服務商名稱、分類名、獎勵說明不翻譯**，那些是資料庫欄位。

**忘記密碼的信是後端寄的，前端沒有機會翻譯**，所以 `/v1/auth/password/forgot`
要把當下的 `locale` 一起送過去。信件文案在 `refcode-api/internal/httpapi/mailtext.go`。

**錯誤訊息一律走 code。** `apiErrorMessage(e, fallbackKey)` 拿 `ApiError.code` 去查 `errors.*`，
查不到才退回後端那句中文；`fallbackKey` 是連不上後端（根本沒拿到 `ApiError`）時顯示的話。
後端每個 code 都對應到單一句話，定義在 `refcode-api/internal/httpapi/response.go`
—— **後端加新 code 時這裡三份語系檔要一起補**。

## 幾個要知道的

**瀏覽不需要登入，但要拿到推薦碼需要註冊。** 服務商頁本身照常公開（家數、評價、
備註都看得到），但 `CodeItem.code` 對未登入的人是 `null`、`masked: true`，
`MerchantPage.vue` 這時只顯示一顆「登入查看完整碼」，複製和前往註冊都不會出現
——沒有碼的話去服務商那邊也不知道要填什麼。上架和看成效數據原本就要帳號，不受影響。

**refresh token 是 rotating 的，併發換發會炸。** 同時發兩個換發請求，
第二個會被後端當成 token 重用而撤銷整族、把使用者踢出去。
`client.ts` 用單一 in-flight promise 處理併發 401，**不要拆掉那段**。

**token 存在 Capacitor Preferences，是 async 的。** `main.ts` 會先 `await initTokens()`
才 mount，否則冷啟動的第一批請求會全部以未登入身分送出。

**`X-Device-ID` 是匿名去重的依據。** 安裝時產生一組 UUID 存進 Preferences，
回報「能不能用」時後端靠它擋重複提交。沒有這個 header 會退回 IP + UA，精準度差很多。

**所在地是選填，在註冊頁選、之後在帳號分頁改。** 目錄會把在地的服務商排前面
（排序在後端，見 `refcode-api`）。國家名稱用 `Intl.DisplayNames` 跟著介面語言產生，
不進語系檔；`src/countries.ts` 只維護代碼清單。改所在地打的是 `PATCH /v1/me`，
那支是整份覆寫，所以顯示名稱要一起送回去。

**複製之後才問「能不能用」。** 沒複製的人沒試過碼，問了只是雜訊。

**Google / Apple 登入走 `@capgo/capacitor-social-login`。** plugin 只負責拿 provider 簽的
ID token，驗證在後端（`refcode-api` 的 `internal/auth/oidc.go` 比對 `iss` 與 `aud`）。
`src/api/social.ts` 讀 `.env` 的 client id，**沒設定的 provider 不會出現在登入頁** ——
所以在瀏覽器裡開發時預設看不到那兩顆按鈕，那是正常的。
同一組 client id 也要列進後端的 `GOOGLE_CLIENT_IDS` / `APPLE_CLIENT_IDS`，兩邊沒對上會登入失敗。

## 設定 Google 登入

程式碼已經接好了，缺的只有 client id。在 Google Cloud Console 建一個專案，
設好 OAuth consent screen 之後，Credentials → Create OAuth client ID 依平台各建一個：

| 建哪種 client | 用在哪 | 填到哪 |
|---|---|---|
| Web application | 瀏覽器開發，以及 Android 拿 ID token 用的 server client | `VITE_GOOGLE_WEB_CLIENT_ID` |
| iOS（要填 bundle id `tw.refcode.app`） | iOS 原生流程 | `VITE_GOOGLE_IOS_CLIENT_ID` |
| Android（要填 package name 與簽章的 SHA-1） | Android 原生流程 | 不用填進 `.env`，但 console 上必須存在 |

Web client 的 Authorized JavaScript origins 要加 `http://localhost:5174`（app）
和 `http://localhost:3000`（官網，那邊也用同一個 web client）。

最後把 **web 與 iOS 兩個 id 都**列進 `refcode-api/.env` 的 `GOOGLE_CLIENT_IDS`（逗號分隔）——
各平台簽出來的 ID token `aud` 不一樣，後端逐一比對，少列哪個那個平台就登不進去。

改完要重啟 api 與 vite：`import.meta.env` 是 build 時代入的，熱更新不會帶進去。

## 上架

送審要用的所有文件在 `store/`（隱私權政策、服務條款、兩家商店的文案與問卷填答、
圖檔規格、送審 checklist）。**目前還有阻斷項沒解決，先讀 `store/README.md`。**

## 原生平台

`ios/` 與 `android/` 都加好了。**改完前端一定要 sync，不然裝到裝置上的是舊的：**

```bash
npm run build && npx cap sync        # 每次改完前端
npx cap open android                 # 開 Android Studio
npx cap open ios                     # 開 Xcode
cd android && ./gradlew :app:assembleDebug   # 不開 IDE 直接出 debug APK
```

**`capacitor.config.ts` 的 `plugins.SocialLogin.providers` 不要打開 facebook / twitter。**
那個 plugin 預設四家 provider 的原生 SDK 全包，Facebook SDK 會把 `AD_ID` 權限、
install referrer 與 `com.facebook.katana` 的 queries 塞進合併後的 manifest ——
Play 的資料安全性表單就必須申報使用廣告 ID，也跟隱私權政策衝突。
改了那段要重跑 `npx cap sync`，並依 `store/play-store/data-safety.md` 的指令重驗 manifest。

## 還沒做
加原生平台後，實機連本機 API 要把 `.env` 的 `VITE_API_BASE_URL` 指到電腦的區網 IP
（模擬器上的 localhost 是裝置自己）。

**推播通知**（追蹤的服務商有新碼時提醒）。
