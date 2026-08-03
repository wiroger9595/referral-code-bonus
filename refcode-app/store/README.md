# 上架文件

App Store 與 Google Play 送審要用的所有文件。

| 檔案 | 用途 |
|---|---|
| `legal/privacy-policy.zh-TW.md` | 隱私權政策（中文，主要版本） |
| `legal/privacy-policy.en.md` | 隱私權政策（英文，App Store 預設語系要用） |
| `legal/terms-of-service.zh-TW.md` | 服務條款 / EULA，含 Apple 要求的 UGC 零容忍條款 |
| `app-store/listing.md` | App Store 商店文案、URL、分級 |
| `app-store/app-privacy.md` | App Privacy（隱私標籤）逐題填答 |
| `app-store/review-notes.md` | 審核備註欄要貼的內容、測試帳號 |
| `play-store/listing.md` | Play 商店文案、分類、聯絡資訊 |
| `play-store/data-safety.md` | Data safety 表單逐題填答 |
| `play-store/content-rating.md` | IARC 分級問卷答案、目標對象 |
| `assets.md` | icon、截圖、宣傳圖的尺寸與內容規劃 |
| `checklist.md` | 送審前逐項勾 |

文件裡的 `{{...}}` 是佔位符，送審前全部要換成真值。至少這幾個：

| 佔位符 | 說明 |
|---|---|
| `{{開發者姓名}}` | 個人開發者帳號的顯示名稱，兩家商店會公開顯示 |
| `{{聯絡email}}` | 支援信箱。Apple 與 Play 都會公開顯示，也是 UGC 檢舉的收件處 |
| `{{官網網域}}` | 例如 `refcode.tw`。隱私權政策、服務條款、支援頁都掛在這底下 |
| `{{生效日期}}` | 政策生效日 |

---

## 先解掉這些，不然一定被打回

送審前這幾項是硬阻斷，不是「最好有」。依嚴重程度排：

### 1. 帳號刪除 ✅ 已完成

- `DELETE /v1/me`（`refcode-api`）：一個交易內把 `code_events` 的 `user_id` 抹掉，
  再刪 users，其餘子表靠外鍵 cascade / set null
- app：帳號頁 → 刪除帳號（要打完整 email + 一次原生 alert 確認）
- 官網：`/delete-account`，三個語系都有，**不必安裝 app 就能完成整個流程**
  —— 這個網址要填進 Play Console 的「資料刪除」欄位

實測過刪除後 users / codes / tokens / oauth / subscriptions 全部歸零、
事件列留著但 `user_id` 已抹除、同一個 email 可以重新註冊。
順帶修掉一個 bug：帳號刪除後那張還沒過期的 access token 會讓 `/v1/me` 回 500，
現在回 401，app 收到會自動登出。

### 2. 隱私權政策與服務條款沒有公開網址 ⛔

兩家商店都要求一個**公開、免登入、可直接開啟**的隱私權政策網址。
現在 `refcode-web` 的 `NUXT_PUBLIC_SITE_URL` 還是 localhost，也還沒有 `/privacy`、`/terms` 這兩頁。

要做的：買網域 → 把 `legal/` 底下的內容做成 `refcode-web` 的頁面 →
更新 `NUXT_PUBLIC_SITE_URL` 與 `public/robots.txt`。

### 3. UGC 要件還差一項（Apple 1.2） ⚠️

推薦碼與備註是使用者產生的內容，Apple 對 UGC app 有一組固定要求。對照現況：

| 要求 | 現況 |
|---|---|
| 內容發布前過濾 | ✅ 全部先進 `pending`，由後台人工審核才會 `active` |
| 使用者檢舉不當內容的機制 | ✅ 複製後可回報「不能用 / 無效 / 已停辦」 |
| app 內公開聯絡方式 | ✅ 帳號頁有「聯絡我們 / 檢舉內容」，**但要設 `VITE_SUPPORT_EMAIL` 才會顯示** |
| 封鎖濫用者的機制 | ❌ 沒有 |

只剩封鎖濫用者。這個 app 沒有使用者之間的互動，可以用「檢舉這位上架者」＋
被檢舉後不再看到該上架者的碼來滿足，實作前建議先想清楚要做到多細。

### 4. Apple 登入要真的能用 ⚠️

程式碼已經接好（`src/api/social.ts`），但還沒有任何 client id，所以按鈕不會出現。
規則是：**只要提供了 Google 登入，就必須同時提供 Apple 登入**。
兩個都不提供也合規，但那等於放棄社群登入。

要做的：Apple Developer 開 Sign in with Apple capability、建 Services ID（web / Android 用）、
Google Cloud 建 OAuth client（iOS 與 Web 各一），填進 app 的 `.env` 與後端的
`GOOGLE_CLIENT_IDS` / `APPLE_CLIENT_IDS`。

Apple 的 Hide My Email 會給 `@privaterelay.appleid.com` 的轉寄信箱，**不要做網域白名單**，
之後要寄信也要走 Apple 的 sender 註冊。

### 5. 原生平台 ✅ 已建，但還缺圖與憑證

`ios/` 與 `android/` 都建好了，兩邊都實際編譯過（Android 出得了簽章 release AAB，
iOS 模擬器 build 通過）。`PrivacyInfo.xcprivacy` 與出口合規那行也加了。

還缺的：**app icon 與啟動畫面**（還是 Capacitor 預設，規格見 `assets.md`）、
iOS 的 Distribution 憑證（要等開發者帳號）。
Android keystore 已產在 `~/keystores/`，**但還沒異地備份 —— 那件事只有你能做**。

### 6. 忘記密碼 ✅ 已完成

重設密碼流程（email 驗證碼）已經做好，app 與官網都有入口。
**email 註冊時的驗證信仍然沒做** —— 那個不是硬性要求，但 `users.email_verified_at`
還一直是空的，社群登入的帳號合併會因此一律走 409（見 `refcode-api/README.md`）。

### 7. 正式 API 必須是 HTTPS ⚠️

iOS 的 ATS 預設擋純 HTTP，Android 9 以上預設也擋 cleartext。
`VITE_API_BASE_URL` 上架版本一定要指向 HTTPS 網域，不要為了方便去開 ATS 例外 ——
那會變成審核時要額外解釋的事。

---

## 兩個要留意的政策風險

不是阻斷項，但這類 app 被打回多半是因為這兩件事，送審文案與審核備註已經針對它們寫過：

**「這只是一堆推薦連結」**（Apple 4.2 最低功能性 / Play 的 Spam and Minimum Functionality）。
兩家都不喜歡單純聚合推廣連結的 app。我們的抗辯是：人工審核的服務商目錄、
使用者回報驅動的品質分數與自動下架、上架者的成效數據 —— 這些都是 app 自己的功能，
不是把網頁包起來。`app-store/review-notes.md` 把這段寫成給審核員看的說明了。

**金融類推薦碼**。目錄裡若有銀行、券商，Play 的 Financial Services 政策與台灣的金融廣告揭露
規範都可能適用（`PLAN.md` 第八節第 3 點已經標記過）。平台本身不提供金融服務、不碰獎勵金流，
但送審前值得確認一次目錄內容。這件事在 Phase 3 開始賣廣告之後只會更重要。

---

## 送審順序

```
1. 解掉還沒解的：2（政策網址）、3（封鎖濫用者）、4（Apple 登入 client id）、7（HTTPS）
2. 把 legal/ 兩份掛上正式網域
3. 出 icon、啟動畫面與截圖（assets.md）
4. Play：內部測試軌道先上，跑完 Data safety 與內容分級問卷
5. App Store：TestFlight 先上，填 App Privacy 與分級
6. 對 checklist.md 逐項勾，送審
```

Play 的稽核通常比 Apple 久（新開發者帳號還有封閉測試的門檻要求），**先送 Play**。
