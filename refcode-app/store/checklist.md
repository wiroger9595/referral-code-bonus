# 送審前 checklist

從上到下逐項勾。前面兩區沒清完，後面的都不用開始。

---

## A. 產品面的阻斷項
細節見 `README.md`。

- [x] `DELETE /v1/me`（後端）＋ 帳號頁的刪除入口（app）＋ `/delete-account`（官網）三者都完成
- [x] 刪除的語意定案，並回頭對齊隱私權政策第七、八節
- [x] 使用者可以自己下架已上架的推薦碼（`POST /v1/codes/{id}/disable`，
      「我的推薦碼」每張卡上有入口。是 status → `disabled` 不是真刪，回報與統計保留）
- [ ] Apple／Play 的資料申報已對齊大頭照 —— `app-privacy.md`（照片或影片）、
      `data-safety.md`（相片）、隱私權政策三份都已改好，填表時照著填
- [ ] 隱私權政策與服務條款掛上正式網域，免登入可直接開
- [x] `.env` 的 `VITE_SUPPORT_EMAIL` 與 `VITE_SITE_URL` 已填
      —— 帳號頁的「聯絡我們 / 檢舉」與條款連結沒填就不會顯示，那三列是 UGC 的送審要件
- [x] 有封鎖 / 檢舉上架者的機制
- [x] 忘記密碼流程可用
- [ ] Apple 登入實際可用（不是只有按鈕）
- [x] 正式 API 是 HTTPS，`VITE_API_BASE_URL` 指向正式網域

## A2. 訂閱（RevenueCat）

- [ ] RevenueCat 專案已建立，entitlement `pro` 已建好
- [ ] App Store Connect 的訂閱群組 `refcode_pro` 與兩個 product 已建立並掛進 RevenueCat
- [ ] Play Console 的訂閱 `pro` 與兩個基本方案已建立並掛進 RevenueCat
- [ ] offering 已設為 current，`getOfferings()` 拿得到方案
- [ ] `refcode-app/.env` 的 `VITE_REVENUECAT_IOS_KEY` / `VITE_REVENUECAT_ANDROID_KEY` 已填
- [x] `refcode-app/.env` 的 `VITE_REVENUECAT_TEST_KEY` **已清空** ——
      Test Store 的 key 會蓋掉平台 key，帶著它送審等於真實購買全部收不到
- [ ] RevenueCat 的 Play service account credentials 已上傳且驗證通過
      （做法見 `refcode-app/README.md`，權限傳播最久 36 小時）
- [ ] `refcode-api/.env` 的 `REVENUECAT_WEBHOOK_AUTH` 已填，且與 RevenueCat 後台的
      Authorization 標頭值一致
- [ ] RevenueCat 後台的 webhook URL 指向正式環境的 `https://.../v1/webhooks/revenuecat`
- [x] 兩邊的 entitlement 名稱一致（app 的 `VITE_REVENUECAT_ENTITLEMENT` 與後端的 `PRO_ENTITLEMENT`）
- [ ] sandbox 買過一次，`subscriptions` 表真的有 upsert 進去
- [ ] paywall 上有自動續訂揭露（期間、價格、扣款時間、怎麼取消）與服務條款連結
- [ ] app 內找得到「恢復購買」
- [ ] 已經是 Pro 的人不會再被推銷同一個方案

## B. 帳號與設定

- [ ] Apple Developer Program 年費已繳（個人帳號 US$99/年）
- [ ] Play Console 開發者帳號已建立、身分驗證通過（US$25 一次性）
- [ ] Apple：App ID `com.referra.app` 已建立，開了 Sign in with Apple capability
- [ ] Apple：Services ID 已建立（web / Android 的 Apple 登入要用）
- [ ] Google Cloud：OAuth client 已建立（iOS 一組、Web 一組）
- [ ] client id 已填進 `refcode-app/.env` **與** `refcode-api` 的 `GOOGLE_CLIENT_IDS` / `APPLE_CLIENT_IDS`
- [ ] 正式環境已建好審核用帳號，且有 active 的推薦碼

## C. 原生專案

- [x] `npm i @capacitor/ios @capacitor/android && npx cap add ios && npx cap add android`
      —— 兩個平台都加好了，Android 已經能出 debug APK
- [ ] `npm run build && npx cap sync`（**每次改前端都要 sync**，忘記就是送出舊版）
- [ ] icon 與啟動畫面已產生（`assets.md`）
- [ ] 版本號策略定好：`version` 對使用者、`build` / `versionCode` 每次上傳都要遞增
- [x] iOS：`ITSAppUsesNonExemptEncryption` = `false`（已寫進 Info.plist，驗過會進 bundle）
- [x] iOS：`NSCameraUsageDescription` 已加進 Info.plist ——
      大頭照的 `<input type="file">` 選單有「拍照」，少了它使用者一點就閃退
- [x] iOS：`PrivacyInfo.xcprivacy` 已加（七類資料，含大頭照的 `PhotosorVideos`，
      加上 `UserDefaults` 的 `CA92.1`），已加進 target 且驗過會進 bundle
- [ ] iOS：Deployment Target 設定好，Xcode 的 Signing 用正式的 Distribution 憑證
- [x] Android：keystore 已產生在 `~/keystores/refcode-app-release.jks`，簽章接進 gradle，release AAB 出得來
      ⚠️ **還沒異地備份 —— 這件事只有你能做**
- [ ] Android：keystore 與密碼已備份到密碼管理器 / 另一台裝置
      （弄丟就永遠無法更新這個 package name，除非改用 Play App Signing 且已註冊）
- [x] Android：`targetSdkVersion` 符合 Play Console 當下公告的最低要求（目前 36）
- [ ] Android：合併後的 manifest 沒有多餘權限（尤其 `AD_ID`）——
      `./gradlew :app:processDebugMainManifest --rerun-tasks` 之後 grep 一次，
      這件事會被 social-login plugin 的 provider 設定影響，升級 plugin 後要重驗
- [ ] 實機測過冷啟動、登入、複製、回報、上架的完整流程

## D. App Store Connect

- [ ] 商店文案已填（`app-store/listing.md`），中英文兩份
- [ ] Support URL / Privacy Policy URL 都打得開
- [ ] App Privacy 已填（`app-store/app-privacy.md`）
- [ ] 年齡分級問卷已答
- [ ] 截圖已上傳（iPhone 6.9" 至少 3 張）
- [ ] 審核備註已貼（`app-store/review-notes.md`），測試帳號可登入
- [ ] TestFlight 內部測試跑過，至少一輪實機驗證
- [ ] 出口合規已回答

## E. Play Console

- [ ] 商店資訊已填（`play-store/listing.md`）
- [ ] Feature graphic 1024×500 已上傳（**最常漏的一項**）
- [ ] 截圖至少 2 張
- [ ] 資料安全性已填（`play-store/data-safety.md`）
- [ ] 內容分級問卷已完成（`play-store/content-rating.md`）
- [ ] 目標對象選 18 歲以上
- [ ] 應用程式存取權已填測試帳號
- [ ] 資料刪除網址已填
- [ ] 廣告聲明：否
- [ ] 個人開發者的封閉測試門檻已滿足（人數與天數以 Console 顯示的為準）

## F. 送出後

- [ ] 兩家的審核狀態通知信有進到會看的信箱
- [ ] 被退件的話，先讀清楚引用的是哪一條指南，再改 —— 不要憑猜測重送
- [ ] 上架後把版本、build number、送審日期記進 repo

---

## 每次更新版本都要重跑

- [ ] `npm run build && npx cap sync`
- [ ] build number / versionCode 遞增
- [ ] What's New 寫具體改了什麼
- [ ] 如果改動涉及蒐集的資料，回頭更新 `app-privacy.md`、`data-safety.md` 與隱私權政策
http://localhost:5175/login?redirect=/registerㄌ