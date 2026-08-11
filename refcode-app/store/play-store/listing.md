# Google Play 商店資訊

Play Console → 主要商店資訊 的各欄位。**方框裡的就是要貼上去的原文。**

預設語言用「中文（繁體）」，另外補「英文（美國）」。

---

## ⛔ 2026-08-11 退件紀錄：被判定為「只能由機構帳戶發布」

第一次送審被退，引用的是**「Play 管理中心規定」政策**，不是內容或 UGC 問題：

> 某些類型的應用程式只能由機構發布。由於您選取了相關應用程式類別，
> 或是聲明應用程式提供特定功能，因此必須使用機構帳戶提交應用程式。

2024-08-31 起，新開的**個人**開發者帳號不得發布金融、健康、VPN、政府這四類 app。
我們被歸到金融那一類。

**這個 app 並不提供金融服務** —— 它是推薦碼目錄，不辦帳戶、不放貸、不交易、
不碰加密貨幣，也不經手任何金流。所以正確的處理是把宣告改成如實反映功能，
不是去申請機構帳戶。退件通知自己也是這樣寫的（「請確認您的應用程式內容宣告
如實反映應用程式的功能」）。

送審前逐項確認，這四個地方任何一個沒對就會再被退：

1. **應用程式內容 → 財務功能**：要選「我的應用程式未提供任何財務功能」。
   這一頁最可能是元凶 —— 目錄裡有銀行的推薦碼，不等於這支 app 提供銀行服務。
2. **主要商店資訊 → 類別**：購物。**不要選財經**。
3. **標記**：不要有任何金融相關標記。
4. **商店文案**：不要用銀行、券商當賣點開頭。下面的文案已經改成以影音串流、
   購物、外送領頭，並加了一段明確聲明不提供金融服務 —— 照著貼，不要改回去。

目錄內容本身也降低了金融比重（見 `../README.md` 的政策風險那一節）。

## 基本資訊

| 欄位 | 內容 |
|---|---|
| 應用程式名稱 | 推薦碼交流站 |
| 套件名稱 | `tw.refcode.app`（與 `capacitor.config.ts` 一致，**發布後永遠不能改**） |
| 應用程式類型 | 應用程式 |
| 類別 | 購物 ⚠️ **絕對不要選「財經」** —— 見下方「2026-08-11 退件紀錄」 |
| 標記 | 折扣與優惠、社群（不要選任何金融相關標記） |
| 價格 | 免費 |
| 應用程式內購 | **有**：訂閱「推薦碼 Pro」（月 / 年） |
| 包含廣告 | **否**（app 內沒有任何廣告 SDK） |

> Pro 訂閱走 Play 計費是正確的。但 Phase 3 的**廣告儲值仍然只在官網做**，
> 不要放進 app —— Play 的計費政策與 Apple 的 IAP 規則是同一個問題
> （`PLAN.md` 第八節第 1 點）。

Play Console → 營利設定 → 訂閱，要建的：

| 欄位 | 內容 |
|---|---|
| 訂閱 ID | `pro` |
| 基本方案 | `monthly`、`yearly`（兩個都設自動續訂） |
| 名稱 / 說明 | 推薦碼 Pro／無限上架、完整成效數據、優先審核 |

RevenueCat 的 entitlement `pro` 要把這兩個基本方案都掛進去，
少掛一個那個方案買了不會解鎖。

## 應用程式名稱（上限 30 字）

```
推薦碼交流站
```

## 簡短說明（上限 80 字）

```
經人工審核的推薦碼目錄。免註冊就能找碼複製，失效的碼會被自動下架。
```

## 完整說明（上限 4000 字）

```
找推薦碼不用再翻論壇。

推薦碼交流站把散落各處的推薦碼整理成一份目錄，每一個都經過人工審核，並且會隨著使用者回報自動淘汰失效的碼。

【找碼的人】

・不用註冊就能瀏覽和複製
・依影音串流、購物、外送、交通、工具等分類瀏覽，或直接搜尋
・每個推薦碼都標示可信度分數，以及最近有多少人回報能用
・一鍵複製，接著跳到該服務商的註冊頁

【上架的人】

・註冊帳號就能免費上架自己的推薦碼
・看得到自己的碼被曝光、點擊、複製了幾次
・同一家服務商一人只能有一個生效中的碼，不會被洗版

【為什麼這裡的碼比較不會過期】

過期的推薦碼是這類服務最大的問題。我們用三道機制處理：

1. 人工審核：每個上架的碼都要先通過審核才會公開
2. 有效期限：上架時必須設定期限，到期自動下架
3. 使用者回報：複製之後可以回報這個碼能不能用，失效比例過高的碼會自動下架

【關於獎勵】

本應用程式不發放獎勵，也不經手任何金流。推薦獎勵由各服務商依其自身的活動規則，直接發給推薦人與被推薦人。活動內容與獎勵金額以該服務商公告為準。

本應用程式僅為推薦碼目錄，不提供銀行、投資、貸款、加密貨幣或任何其他金融服務，也不代辦開戶或申請。點選推薦碼後一律導向該服務商自己的網站，所有交易與帳戶行為都發生在該服務商，與本應用程式無關。

【隱私】

沒有廣告 SDK，沒有第三方分析工具，不會跨應用程式追蹤你。瀏覽服務商與推薦碼列表不需要帳號；
註冊帳號後即可查看推薦碼並複製。

隱私權政策：https://{{官網網域}}/privacy
服務條款：https://{{官網網域}}/terms
刪除帳號：https://{{官網網域}}/delete-account
聯絡我們：robertsmart1989@gmail.com
```

## 聯絡資訊

| 欄位 | 值 | 必填 |
|---|---|---|
| 電子郵件 | `robertsmart1989@gmail.com` | ✅ 會公開顯示在商店頁 |
| 網站 | `https://{{官網網域}}` | 建議填 |
| 電話 | 個人開發者可留空 | |

## 隱私權政策

`https://{{官網網域}}/privacy`

Play 會實際去抓這個網址。**擋爬蟲、需要登入、或只是重新導向到首頁都會被判定不合格。**

## 帳號刪除

Play Console → 應用程式內容 → 「資料刪除」要填一個**不必安裝 app 就能送出刪除請求的網址**：

```
https://{{官網網域}}/delete-account
```

同時 app 內也要有刪除入口。兩者缺一都不行。⛔ 目前兩邊都還沒做，見 `../README.md`。

## 資料安全性

見 `data-safety.md`。

## 內容分級與目標對象

見 `content-rating.md`。

## 應用程式存取權（測試帳號）

Play Console → 應用程式內容 → 「應用程式存取權」。
本 app 瀏覽目錄不需登入，但要看到推薦碼的實際內容並複製、以及上架與成效數據都需要，
所以要選「部分功能受限」並提供帳號：

| 欄位 | 值 |
|---|---|
| 名稱 | `Publisher account` |
| 帳號 | `{{審核用帳號email}}` |
| 密碼 | `{{審核用帳號密碼}}` |
| 操作說明 | `Browsing the merchant and code list needs no account, but each code's actual content is masked until you sign in. Sign in on the Account tab to reveal and copy codes, reach "My Codes" (published codes and their impression / click / copy stats), and the "Add code" flow. A submitted code stays in a pending state until a human moderator approves it.` |

---

## 英文（美國）版本

**名稱**

```
RefCode — Referral Code Finder
```

**簡短說明**

```
A moderated directory of referral codes. Browse free, sign in to reveal and copy.
```

**完整說明**

```
Stop digging through forums for a referral code that still works.

RefCode collects referral codes into one reviewed directory. Every code is checked by a human before it appears, and codes that stop working get delisted automatically based on user reports.

FOR PEOPLE LOOKING FOR A CODE
• No account needed to browse the directory; sign in (free) to reveal and copy a code
• Browse by category — streaming, shopping, delivery, rides, productivity — or just search
• Every code shows a quality score and how many people recently reported it working
• Copy with one tap, then jump straight to the merchant's signup page

FOR PEOPLE SHARING A CODE
• Create an account and list your own code, free
• See how many times your code was shown, tapped and copied
• One active code per merchant per person, so nobody can flood the list

WHY CODES HERE GO STALE LESS OFTEN
1. Human review — every submission is approved before it goes public
2. Expiry dates — required at submission, enforced automatically
3. User reports — after signing in and copying, you can report whether the code worked; codes with a high failure rate are delisted automatically

ABOUT REWARDS
This app does not issue rewards and never handles money. Referral rewards are paid by each merchant directly, under that merchant's own program rules.

This app is a referral code directory only. It does not provide banking, investment, lending, cryptocurrency or any other financial service, and it does not open or apply for accounts on your behalf. Tapping a code sends you to the merchant's own website; every transaction and account action happens there, not here.

PRIVACY
No ad SDKs, no third-party analytics, no cross-app tracking. Browsing the directory requires no account; revealing and copying a code does.

Privacy policy: https://{{官網網域}}/privacy
Terms: https://{{官網網域}}/terms
Delete your account: https://{{官網網域}}/delete-account
Contact: robertsmart1989@gmail.com
```

---

## 個人開發者帳號的額外門檻

2023 年 11 月之後新開的個人帳號，正式發布前 Play 要求：

- 完成身分驗證（Play Console 會要求證件與地址證明）
- **先跑封閉測試**：找到足夠數量的測試人員、持續一段時間，才能申請正式發布

實際的人數與天數 Google 調整過幾次，**以你的 Play Console 當下顯示的要求為準**，
不要照網路上的舊文章準備。這一關通常比 Apple 的審核花更多時間，所以排程上先送 Play。
