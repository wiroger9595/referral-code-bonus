# Play 資料安全性表單填答

Play Console → 應用程式內容 →「資料安全性」的逐題答案。
依據是實際程式碼，出處都標了。**改到相關程式碼時要回來對齊這份。**

Play 會拿這份表單和實際的網路行為比對，對不上會被下架，比填不完整嚴重得多。

---

## 總覽題

| 題目 | 答案 |
|---|---|
| 這個應用程式會蒐集或分享任何必要使用者資料類型嗎？ | **是** |
| 傳輸過程中的所有使用者資料都經過加密嗎？ | **是**（全站 HTTPS） |
| 你是否提供使用者要求刪除資料的方式？ | **是**（app 內＋網頁，⛔ 尚未實作） |
| 你是否已通過全球公認的安全性標準獨立驗證？ | 否（個人開發者，沒做 MASA 稽核） |

> 「刪除資料」那題答「是」的同時，會要求填一個**不必安裝 app 就能送出請求的網址**：
> `https://{{官網網域}}/delete-account`。這頁還不存在，見 `../README.md` 阻斷項 1。

---

## 個人資訊

### 電子郵件地址

| 問題 | 答案 |
|---|---|
| 有蒐集嗎 | 是 |
| 有分享給第三方嗎 | 否 |
| 是否經過加密傳輸 | 是 |
| 使用者可以要求刪除嗎 | 是 |
| 這項資料是必要的還是選填 | **必要**（只在建立帳號時；瀏覽本身不需要帳號） |
| 用途 | 帳戶管理、應用程式功能 |

出處：`src/stores/auth.ts`、社群登入的 ID token `email` claim。

### 姓名

| 問題 | 答案 |
|---|---|
| 有蒐集嗎 | 是（顯示名稱） |
| 有分享給第三方嗎 | 否 |
| 是否經過加密傳輸 | 是 |
| 使用者可以要求刪除嗎 | 是 |
| 這項資料是必要的還是選填 | **選填**（留空會用 email 前半段） |
| 用途 | 應用程式功能（與上架的推薦碼一起公開顯示） |

### 使用者 ID

| 問題 | 答案 |
|---|---|
| 有蒐集嗎 | 是（帳號 ID、Google / Apple 的 provider 使用者 ID） |
| 有分享給第三方嗎 | 否 |
| 是否經過加密傳輸 | 是 |
| 使用者可以要求刪除嗎 | 是 |
| 必要 / 選填 | 必要（有帳號時） |
| 用途 | 帳戶管理、應用程式功能 |

---

## 應用程式活動

### 應用程式內搜尋記錄

**不勾。** 搜尋字串只作為 `GET /v1/merchants?q=` 的查詢參數，後端不儲存。

> ⚠️ 之後若為了做熱門關鍵字而開始存下來，這一項要改成有蒐集。

### 其他使用者產生的內容

| 問題 | 答案 |
|---|---|
| 有蒐集嗎 | 是（推薦碼、備註說明、有效期限、回報結果） |
| 有分享給第三方嗎 | 否 |
| 是否經過加密傳輸 | 是 |
| 使用者可以要求刪除嗎 | 是 |
| 必要 / 選填 | 選填（只有主動上架的人才有） |
| 用途 | 應用程式功能 |

> 這些內容**會公開顯示**給所有使用者。Play 的表單問的是「是否分享給第三方」，
> 指的是傳給其他公司，公開顯示在自家服務內不算「分享」，所以答「否」。
> 但公開這件事一定要寫在隱私權政策裡（已寫在第 2.3 節）。

### 應用程式互動

| 問題 | 答案 |
|---|---|
| 有蒐集嗎 | 是（瀏覽了哪些服務商與推薦碼、複製與點擊事件） |
| 有分享給第三方嗎 | 否 |
| 是否經過加密傳輸 | 是 |
| 使用者可以要求刪除嗎 | 是（刪除帳號後改為匿名保留） |
| 必要 / 選填 | 必要 |
| 用途 | 應用程式功能、數據分析 |

出處：`api.track()` → 後端的 `code_events`。

---

## 財務資訊 → 購買記錄

| 問題 | 答案 |
|---|---|
| 有蒐集嗎 | 是（Pro 訂閱的狀態、方案、到期日） |
| 有分享給第三方嗎 | **否** |
| 是否經過加密傳輸 | 是 |
| 使用者可以要求刪除嗎 | 是 |
| 必要 / 選填 | 選填（只有訂閱的人才有） |
| 用途 | 應用程式功能 |

**不要勾「付款資訊」。** 卡號與付款方式全程在 Google Play，我們拿不到也沒有儲存，
只收到「這個帳號有沒有生效中的訂閱」。

> 「有分享給第三方嗎」答否的理由：RevenueCat 是代我們處理計費的服務供應商
> （service provider），Play 的定義把這類轉移排除在「分享」之外。
> 但**這是我們的判斷**，送出前值得再讀一次 Play Console 當下的說明文字 ——
> Google 對 service provider 的界定改過。

---

## 裝置或其他 ID

| 問題 | 答案 |
|---|---|
| 有蒐集嗎 | 是 |
| 有分享給第三方嗎 | 否 |
| 是否經過加密傳輸 | 是 |
| 使用者可以要求刪除嗎 | 是（解除安裝即失效；伺服器端的紀錄可要求刪除） |
| 必要 / 選填 | 必要 |
| 用途 | **應用程式功能、詐欺防範與安全性、法規遵循** |

內容：app 首次啟動時 `crypto.randomUUID()` 產生的 UUID，存在 Capacitor Preferences，
以 `X-Device-ID` 標頭送出（`src/api/client.ts`）。

**不是 Android 廣告 ID。** Play Console 會另外問「你的 app 有使用廣告 ID 嗎」→ **否**。

這一題答「否」**是有條件的，不是天生如此**：`@capgo/capacitor-social-login` 預設會把
Facebook 與 X 的原生 SDK 一起打包，Facebook SDK 會帶進 `com.google.android.gms.permission.AD_ID`、
`ACCESS_ADSERVICES_*`、install referrer 與 `com.facebook.katana` 的 queries。
我們在 `capacitor.config.ts` 的 `plugins.SocialLogin.providers` 把那兩家關掉才乾淨。
**動到那段設定、或升級這個 plugin 之後，一定要重驗一次。**

實際驗過的合併後權限（`./gradlew :app:processDebugMainManifest` 之後看
`app/build/intermediates/merged_manifest/debug/.../AndroidManifest.xml`）：

| 權限 | 來源 | 要申報嗎 |
|---|---|---|
| `INTERNET` | 我們自己的 manifest | 否 |
| `ACCESS_NETWORK_STATE` | Play Billing / RevenueCat | 否 |
| `USE_BIOMETRIC` / `USE_CREDENTIALS` / `USE_FINGERPRINT` | `androidx.credentials`（Google 登入走 Credential Manager） | 否，都是 normal 權限，不會跳授權對話框 |

沒有任何 dangerous 權限，也沒有 `AD_ID`。

驗證指令：

```bash
cd android && ./gradlew :app:processDebugMainManifest --rerun-tasks
grep -c AD_ID app/build/intermediates/merged_manifest/debug/*/AndroidManifest.xml   # 要是 0
```

---

## 未蒐集的類別（都不要勾）

位置（精確與概略）、財務資訊、健康與健身、訊息（簡訊/通話）、
相片和影片、音訊檔案、檔案和文件、日曆、聯絡人、
應用程式效能（沒有崩潰回報 SDK）、網頁瀏覽記錄。

---

## 一致性檢查

送出前對照一次，這三份說的必須是同一件事：

- `data-safety.md`（本檔）
- `../app-store/app-privacy.md`
- `../legal/privacy-policy.zh-TW.md`

三者不一致時，最容易出事的是隱私權政策漏寫了某一類 —— Play 的自動掃描會抓。
