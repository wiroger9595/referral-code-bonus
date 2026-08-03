# App Privacy（隱私標籤）填答

App Store Connect →「App 隱私權」的逐題答案。這份的依據是實際的程式碼，
不是猜的 —— 每一項後面都標了出處。**改到相關程式碼時要回來對齊這份。**

填錯不只是被退件：標籤與實際行為不符是 Apple 會直接下架的項目。

---

## 追蹤

> 你是否將資料用於追蹤目的？

**否。**

app 內沒有任何廣告 SDK、沒有第三方分析 SDK、沒有取用 IDFA，
不會與資料仲介商共享資料，也不會跨其他公司的 app 或網站建立使用者輪廓。

RevenueCat 是訂閱計費的服務商（處理者），只拿到訂閱必要的資料，
不用於廣告或跨 app 追蹤，所以這一題仍然是「否」。

因此**不需要顯示 ATT（App Tracking Transparency）授權對話框**，
也不需要在 Info.plist 加 `NSUserTrackingUsageDescription`。

> 判斷依據：`package.json` 的相依只有 Capacitor 官方 plugin、`@capgo/capacitor-social-login`
> 與 `@revenuecat/purchases-capacitor`，沒有任何廣告或分析套件。
> **加新套件之前先確認它有沒有連自己的伺服器**，有的話這份要跟著改。

---

## 蒐集的資料類型

### 聯絡資訊 → 電子郵件地址

| | |
|---|---|
| 有蒐集嗎 | 是 |
| 用途 | **App 功能**（帳號建立與登入） |
| 是否連結到使用者 | **是** |
| 是否用於追蹤 | 否 |

出處：`src/stores/auth.ts` 的 `register()` / `login()`；社群登入時由 ID token 的 `email` claim 取得。

### 使用者內容 → 其他使用者內容

| | |
|---|---|
| 有蒐集嗎 | 是 |
| 內容 | 使用者上架的推薦碼、備註說明、有效期限、以及對他人推薦碼的回報結果 |
| 用途 | **App 功能** |
| 是否連結到使用者 | **是** |
| 是否用於追蹤 | 否 |

出處：`api.createCode()`、`api.report()`。

> 顯示名稱也會與推薦碼一起公開顯示。它由使用者自行輸入或由社群登入帶入，
> 在標籤上歸在「使用者內容」與「識別碼 → 使用者 ID」之間都說得通，
> 保守起見兩邊都已涵蓋。

### 識別碼 → 使用者 ID

| | |
|---|---|
| 有蒐集嗎 | 是 |
| 內容 | 帳號 ID、Google / Apple 的 provider 使用者識別碼 |
| 用途 | **App 功能** |
| 是否連結到使用者 | 是 |
| 是否用於追蹤 | 否 |

### 識別碼 → 裝置 ID

| | |
|---|---|
| 有蒐集嗎 | 是 |
| 內容 | app 首次啟動時產生的隨機 UUID，存在 Capacitor Preferences，以 `X-Device-ID` 標頭送出 |
| 用途 | **App 功能**（防止同一裝置重複回報、濫用偵測） |
| 是否連結到使用者 | **是**（登入後事件會同時帶 user_id） |
| 是否用於追蹤 | 否 |

出處：`src/api/client.ts` 的 `initTokens()` 與 `request()`。

**這不是 IDFA / IDFV。** 它由 `crypto.randomUUID()` 產生，只在本服務內有意義，
解除安裝即失效。填表時如果 Apple 追問「為何需要裝置識別碼」，答案是防止灌票 ——
描述已經寫進隱私權政策第 2.1 節。

### 購買項目 → 購買記錄

| | |
|---|---|
| 有蒐集嗎 | 是 |
| 內容 | Pro 訂閱的購買與續訂狀態、方案、到期日 |
| 用途 | **App 功能**（解鎖 Pro、恢復購買） |
| 是否連結到使用者 | **是** |
| 是否用於追蹤 | 否 |

出處：`src/api/purchases.ts`；狀態由 RevenueCat 的 webhook 回寫到後端的
`subscriptions` 表（`refcode-api/internal/httpapi/handlers_billing.go`）。

> 我們拿不到、也沒有儲存任何付款方式或卡號 —— 交易全部在 App Store 完成，
> 我們只收到「這個帳號有沒有生效中的訂閱」。填表時不要勾「財務資訊」。

### 使用資料 → 產品互動

| | |
|---|---|
| 有蒐集嗎 | 是 |
| 內容 | 瀏覽了哪些服務商與推薦碼、複製與點擊事件 |
| 用途 | **App 功能**（計算品質分數與排序）＋ **分析** |
| 是否連結到使用者 | 是 |
| 是否用於追蹤 | 否 |

出處：`api.track()`，後端寫進 `code_events`。

> 「分析」這一項要勾。雖然沒有第三方分析 SDK，但我們自己的伺服器確實在做統計，
> Apple 問的是行為不是工具。

---

## 未蒐集的類別（表單裡不要勾）

**財務資訊**（付款方式與卡號全程在 App Store，我們拿不到）、健康與健身、
位置（精確與概略皆無）、聯絡人、使用者的照片或影片、音訊資料、
瀏覽記錄（指跨網站的瀏覽記錄）、搜尋記錄（app 內搜尋字串目前不上傳，
只當查詢參數用完即丟）、診斷資料（沒有崩潰回報服務）、敏感資訊。

> ⚠️ `api.listMerchants()` 會把搜尋字串當 query 參數送到後端。
> 只要後端沒有**儲存**它，就不算蒐集「搜尋記錄」。
> 如果之後為了做熱門關鍵字而開始存下來，這一欄要改成有蒐集。

---

## 隱私權政策網址

`https://{{官網網域}}/privacy` —— 內容見 `../legal/privacy-policy.zh-TW.md`。

---

## iOS 隱私權資訊清單（PrivacyInfo.xcprivacy）

除了商店端的標籤，**app 的 Xcode 專案裡還要有一份 `PrivacyInfo.xcprivacy`**，
這是另一件事，不要混在一起。跑完 `npx cap add ios` 之後要自己加。

至少要宣告的：

| 項目 | 值 |
|---|---|
| `NSPrivacyTracking` | `false` |
| `NSPrivacyTrackingDomains` | 空陣列 |
| `NSPrivacyCollectedDataTypes` | 對應上面蒐集的六類（含購買記錄） |
| `NSPrivacyAccessedAPITypes` | `NSPrivacyAccessedAPICategoryUserDefaults`，理由代碼 `CA92.1` |

`UserDefaults` 那一條是因為 `@capacitor/preferences` 用它存 token 與裝置 ID。
Capacitor 官方 plugin 與 RevenueCat SDK 各自帶了自己的 manifest，
但**主 app target 的那一份要自己寫**。升級 SDK 之後要重跑一次隱私報告核對。

加完之後用 Xcode 的 Product → Archive，在 Organizer 產生一份隱私報告
（Generate Privacy Report）核對，比等上傳被退快得多。
