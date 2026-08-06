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

**分類名與獎勵說明是資料庫欄位，但有多語版本**（`merchant_categories.name_en/ja`、
`merchants.reward_desc_en/ja`）。API 靠 `?lang=zh|en|ja` 決定回哪一份，譯文沒填就退回中文。
`client.ts` 的 `apiLang` 由 `setApiLang()` 注入 —— 不直接 import i18n，那會跟
`i18n/index.ts` 形成循環相依。**切語言時三個 API 都要重打**，不然分類名還停在上一個語言。

**服務商名稱（`merchants.name`）不翻**，那是品牌名。

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

## 測訂閱：先用 Test Store

商店那邊的商品、憑證都還沒好之前，用 RevenueCat 的 Test Store 就能把購買流程從頭到尾走一次
——購買不經過 App Store / Play，所以**不需要 Play Console 商品、不需要 service account
credentials、不需要先把 app 上傳到測試軌**。

1. RevenueCat 後台 → Product Catalog 建好 product 與 offering（設為 current），entitlement 掛 `pro`
2. Project settings → API keys 拿 Test Store 的 key（`test_` 開頭，也是 public key）
3. 填進 `.env` 的 `VITE_REVENUECAT_TEST_KEY`
4. `npm run build && npx cap sync android && npx cap open android`，用實機或模擬器跑

`purchases.ts` 裡 `VITE_REVENUECAT_TEST_KEY` 有值就蓋掉平台 key，兩把 `goog_` / `appl_`
不用動。**瀏覽器還是測不到** —— 這個 plugin 的 web 實作要另外接 RevenueCat Web Billing，
`purchasesAvailable` 在 web 一律 false。

Test Store 的行為跟正式的不一樣，別拿它當上架前的最後驗證：訂閱**最多自動續訂 5 次**就取消，
續訂週期被壓縮成 5–60 分鐘（依方案長度），資料一律記為 sandbox。

⚠️ **絕對不要拿 test key 包版送商店。** 送出去的話真實購買全部不會生效。
`.env` 的 `VITE_REVENUECAT_TEST_KEY` 只在開發時填，包版前清空。

## 產生 RevenueCat 的 Play service account credentials

`.env` 裡那兩把 `VITE_REVENUECAT_*_KEY` 只是 SDK 用的 public key，**沒有它 RevenueCat 也讀不到
Google Play 的訂閱狀態** —— RevenueCat 要用一組 Google Cloud 的 service account 去打 Play
Developer API 查購買、查續訂、收退款通知。那把金鑰是 JSON 檔，只貼進 RevenueCat 後台，
不進 `.env`、不進版控。

### 1. Google Cloud Console

Play Console → 設定 → API 存取，先確認已經連到一個 Google Cloud 專案（沒有就在那頁建），
接著在**那個專案底下**做：

1. 啟用兩個 API：**Google Play Android Developer API** 與 **Google Play Developer Reporting API**。
2. IAM 與管理 → 服務帳戶 → 建立服務帳戶，名字隨意（例如 `revenuecat`），角色給
   **Pub/Sub Editor**（RevenueCat 要用即時開發者通知才知道續訂／退款）與
   **Monitoring Viewer**（讓它自己看得到通知佇列有沒有塞住）。
3. 進那個服務帳戶 → 金鑰 → 新增金鑰 → 建立新的金鑰 → 選 **JSON** → 下載。
   **這個檔只會給你一次**，弄丟就重建一把新的。

### 2. Play Console 授權

回 Play Console → 使用者和權限 → 邀請新使用者，填剛才那個服務帳戶的 email
（`xxx@專案id.iam.gserviceaccount.com`），帳戶權限勾這三個，少一個 RevenueCat 就會報憑證無效：

- 查看應用程式資訊並下載大量報表（唯讀）
- 查看財務資料、訂單和取消問卷回覆
- 管理訂單和訂閱項目

### 3. 貼進 RevenueCat

RevenueCat Dashboard → 專案 → 該 Google Play app 的設定 → 上傳那份 JSON，存檔。

**存完當下多半會顯示憑證無效，那是正常的** —— Google 那邊權限傳播最久要 36 小時，
這段期間 RevenueCat 驗證會回 503 / 521。**不要因為報錯就重建一把金鑰**，只會從頭再等一次。

### 幾個會卡住的點

**金鑰一旦外流，Google 會直接停用整個服務帳戶**，訂閱狀態會全部查不到。所以 JSON 放
repo 外面（跟 keystore 放一起），不要為了方便丟進 `android/` 或 `.env`。

**2024/5/3 之後建立的 Google Cloud 組織預設禁止建立服務帳戶金鑰**
（`iam.disableServiceAccountKeyCreation` 這條組織政策）。個人帳號不受影響；
如果是公司的組織，第 3 步會直接被擋，要請組織管理員先關掉那條政策。

iOS 那邊不用這套，走的是 App Store Connect 的 In-App Purchase Key，另外產生。

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

## 包版

送商店的正式版。**每次包版一定從 `npm run build` 開始** —— `cap sync` 只是把 `dist/`
複製進兩個原生專案，它不會幫你重新 build 前端，忘了這步就是拿舊的 `dist/` 去送審。

**build 之前先確認 `.env` 的 `VITE_REVENUECAT_TEST_KEY` 是空的**，那把 key 會蓋掉平台 key，
帶著它送出去的版本收不到任何真實購買。

### 版本號

兩個平台各記各的，**沒有共用來源**（`package.json` 的 `version` 沒有人讀，別改那個）：

| 平台 | 檔案 | 欄位 |
|---|---|---|
| Android | `android/app/build.gradle` | `versionName`（給人看的 1.2.0）、`versionCode`（整數，只能往上加） |
| iOS | `ios/App/App.xcodeproj/project.pbxproj` | `MARKETING_VERSION`（對應 versionName）、`CURRENT_PROJECT_VERSION`（對應 versionCode） |

規則：`versionName` / `MARKETING_VERSION` 兩邊要一樣，使用者在兩家商店看到的是同一個版本。
`versionCode` / `CURRENT_PROJECT_VERSION` **每上傳一次就 +1**，就算版本名沒變也要加 ——
兩家商店都不接受重複的 build 號，被退回來才發現要整包重出。

### Android → `.aab`

```bash
npm run build && npx cap sync android
cd android && ./gradlew :app:bundleRelease      # 產物：app/build/outputs/bundle/release/app-release.aab
```

簽章由 `android/keystore.properties` 帶進來（gitignore 掉，裡面有密碼），keystore 本體
放在 repo 外面。**那把金鑰掉了就換不回來** —— Play 上的 app 認的是簽章，補不了，
只能用新的 package name 重上一個 app。備份它。

`keystore.properties` 不存在時 build 不會掛，只是出來的 aab 沒簽章（見
`android/app/build.gradle` 開頭）——CI 或別台機器上沒有金鑰也要能跑 debug build。
所以**上傳前先確認產物真的簽過**：

```bash
keytool -printcert -jarfile android/app/build/outputs/bundle/release/app-release.aab
```

印得出 `CN=RefCode` 才是簽好的；印不出來就是 `keystore.properties` 沒被讀到。

上傳走 Play Console → 正式版 → 建立新版本，把 `.aab` 拉進去。

### iOS → `.ipa`

```bash
npm run build && npx cap sync ios
xcodebuild -project ios/App/App.xcodeproj -scheme App -configuration Release \
  -sdk iphoneos -archivePath ios/App/output/App.xcarchive archive
xcodebuild -exportArchive -archivePath ios/App/output/App.xcarchive \
  -exportOptionsPlist ios/App/ExportOptions.plist -exportPath ios/App/output
```

`ios/App/output/` 已經在 `ios/.gitignore` 裡，產物不會進版控。

**目前第二步（export）跑不過**，卡在憑證：archive 這步用開發憑證就能簽，實測會過；
但 export 要的是 **iOS Distribution 憑證**，那個要有付費的 Apple Developer Program
帳號才申請得到，現在沒有。錯誤長這樣：

```
error: exportArchive No signing certificate "iOS Distribution" found
error: exportArchive No profiles for 'tw.refcode.app' were found
```

這是已知的送審阻斷項，見 `store/README.md` 的「5. 原生平台」。帳號下來之後，
**先用 Xcode 走一次** `npx cap open ios` → Product → Archive → Distribute App，
讓它把 Distribution 憑證與描述檔建起來，之後上面那條指令列的路才會通。

不要為了繞過這個錯誤而在指令後面加 `-allowProvisioningUpdates` —— 那會直接在
Apple 帳號上申請憑證，而 Distribution 憑證的數量有上限，該由人決定什麼時候建。

上傳用 `xcrun altool` 或 Xcode 的 Organizer；TestFlight 內部測試不必等審核，
正式送審走 App Store Connect。

**`ExportOptions.plist` 的 `manageAppVersionAndBuildNumber` 是 `false`，不要改成 true。**
開著的話 Xcode 會自己把 build number 往上加，專案裡的 `CURRENT_PROJECT_VERSION`
就跟送出去的版本對不起來，日後要查哪個 build 對應哪份原始碼會找不到。

### 送審前

`store/checklist.md` 走一遍。**目前還有阻斷項沒解決**，見 `store/README.md`。

## 還沒做
加原生平台後，實機連本機 API 要把 `.env` 的 `VITE_API_BASE_URL` 指到電腦的區網 IP
（模擬器上的 localhost 是裝置自己）。

**推播通知**（追蹤的服務商有新碼時提醒）。
