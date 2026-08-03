# App Store 商店資訊

App Store Connect 各欄位要填的內容。**方框裡的就是要貼上去的原文**，方框外是說明。

主要語系用「繁體中文」，另外補一份英文（美國）—— App Store 的預設語系沒填的話，
非中文地區的使用者看到的會是空白。

---

## 基本資訊

| 欄位 | 內容 |
|---|---|
| Bundle ID | `tw.refcode.app`（與 `capacitor.config.ts` 的 `appId` 一致，**建立後不能改**） |
| SKU | `refcode-app-001` |
| 主要語系 | 繁體中文 |
| 主要類別 | 購物（Shopping） |
| 次要類別 | 工具程式（Utilities） |
| 價格 | 免費 |
| App 內購買 | **有**：自動續訂訂閱「推薦碼 Pro」（月 / 年），透過 RevenueCat 管理 |
| 版權 | `{{西元年}} {{開發者姓名}}` |

> Pro 訂閱走 IAP 是正確的（賣的是 app 內的功能）。但 Phase 3 的**廣告儲值仍然不要放進 app**，
> 那是兩件事：Pro 是功能解鎖，儲值是買曝光。儲值只在官網做（`PLAN.md` 第八節第 1 點）。

### 訂閱的必填欄位

App Store Connect 建 auto-renewable subscription 時，除了價格還有幾個一定要填、
漏掉會在送審時被擋下來的：

| 欄位 | 內容 |
|---|---|
| Subscription Group | `refcode_pro`（月繳與年繳放同一組，使用者才能在方案間升降級） |
| Product ID | `tw.refcode.app.pro.monthly` / `tw.refcode.app.pro.yearly` |
| 訂閱顯示名稱 | 推薦碼 Pro |
| 訂閱說明 | 無限上架推薦碼、完整成效數據、優先審核 |
| 審核用截圖 | **要上傳 paywall 的截圖**，每個 product 各一張 |
| App 內購買的隱私權政策 URL | `https://{{官網網域}}/privacy` |
| 服務條款（EULA）URL | `https://{{官網網域}}/terms` |

RevenueCat 那邊要對應建好 entitlement `pro` 與 offering，
product id 兩邊必須完全一致（見 `../../src/api/purchases.ts` 的 `PRO_ENTITLEMENT`）。

## 名稱與副標

**App 名稱**（上限 30 字）

```
推薦碼交流站
```

**副標題 Subtitle**（上限 30 字）

```
找到能用的推薦碼，也上架自己的
```

## 關鍵字

**Keywords**（上限 100 字元，逗號分隔，**不要加空格**）

```
推薦碼,邀請碼,優惠碼,折扣碼,推薦人,好友推薦,開戶,信用卡,券商,回饋,現金回饋,分享碼,序號,揪團,推薦獎勵,開戶禮,首刷禮,註冊碼,省錢
```

（79 字元，還有空間。要再加的話從實際搜尋量高的詞補，不要湊字數。）

不要把 App 名稱和副標裡已經有的字重複放進來，Apple 會一起檢索。
也不要放其他 app 或品牌的名字，那是明確會被退件的。

## 宣傳文字

**Promotional Text**（上限 170 字，**不用送審就能改**，適合放時效性的話）

```
新增多家銀行與券商的推薦碼目錄。所有推薦碼都經人工審核，複製後可以回報「能不能用」——失效的碼會被自動下架，不會浪費你的時間。
```

## 描述

**Description**（上限 4000 字）

```
找推薦碼不用再翻論壇。

推薦碼交流站把散落在各處的推薦碼整理成一份目錄，每一個都經過人工審核，而且會隨著使用者回報自動淘汰失效的碼。

■ 找碼的人

・不用註冊就能瀏覽和複製，開啟就能用
・依銀行、券商、電商、外送、串流等分類瀏覽，也可以直接搜尋
・每個推薦碼都標示可信度分數，以及最近有多少人回報「能用」
・一鍵複製，接著直接跳到該服務商的註冊頁

■ 上架的人

・註冊帳號就能上架自己的推薦碼，免費
・可以看到自己的碼被曝光、點擊、複製了幾次
・同一家服務商一人只能上架一個生效中的碼，所以不會被洗版

■ 為什麼這裡的碼比較不會過期

過期的推薦碼是這類服務最大的問題。我們用三道機制處理：

1. 人工審核 —— 每個上架的碼都要先通過審核才會公開
2. 有效期限 —— 上架時必須設定期限，到期自動下架
3. 使用者回報 —— 複製之後可以回報這個碼能不能用，失效比例過高的碼會自動下架

■ 關於獎勵

本 App 不發放獎勵，也不經手任何金流。推薦獎勵由各服務商依其自身的活動規則，直接發給推薦人與被推薦人。活動內容與獎勵金額以該服務商公告為準。

金融類服務（開戶、信用卡、投資）有其風險，本 App 不構成任何投資建議。

■ 隱私

沒有廣告 SDK，沒有第三方分析工具，不會跨 App 追蹤你。瀏覽服務商與推薦碼列表不需要帳號；
註冊帳號後即可查看推薦碼並複製。

隱私權政策：https://{{官網網域}}/privacy
服務條款：https://{{官網網域}}/terms
聯絡我們：{{聯絡email}}
```

## 版本更新說明

**What's New**（首次送審填這個）

```
第一版。
```

之後每次更新都要重填，寫實際改了什麼，不要寫「修正若干問題並優化體驗」——
這種內容 Apple 近年會退回要求具體說明。

## 網址

| 欄位 | 值 | 必填 |
|---|---|---|
| Support URL | `https://{{官網網域}}/support` | ✅ |
| Marketing URL | `https://{{官網網域}}` | 選填 |
| Privacy Policy URL | `https://{{官網網域}}/privacy` | ✅ |

三個都要是**公開、免登入、能直接開**的頁面。Support 頁至少要有聯絡信箱與常見問題，
只放一個 email 的空白頁會被退。

## 年齡分級

在 App Store Connect 的分級問卷裡逐題回答，這幾題要留意：

| 題目 | 答案 | 理由 |
|---|---|---|
| 使用者產生的內容 | **有**，且已具備審核與檢舉機制 | 推薦碼與備註是使用者上架的 |
| 不限制的網路存取 | **無** | app 內建瀏覽器只會開後台維護的服務商註冊網址，不是任意網址 |
| 賭博 | 無 | |
| 模擬賭博 | 無 | |
| 競賽 / 抽獎 | 無 | 獎勵由服務商發放，本 app 不辦活動 |
| 酒精、菸草、藥物 | 無 | |
| 暴力、性相關內容 | 無 | |

問卷答完會自動得出分級。有 UGC 的情況下通常不會落在最低級距，
**預期是 13+ 或 16+**。若目錄中有金融類服務商，建議直接選較保守的級距 ——
分級太低反而是後續被檢舉的破口。

## 隱私標籤

見 `app-privacy.md`，那份是逐題的填答。

## 審核備註

見 `review-notes.md`。**這欄一定要填**，這類目錄型 app 空著送審被打回的機率很高。

---

## 英文（美國）版本

非中文地區看到的內容。至少要有名稱、副標、描述。

**Name**

```
RefCode — Referral Code Finder
```

**Subtitle**

```
Find codes that actually work
```

**Keywords**

```
referral,invite code,promo code,discount,refer a friend,cashback,signup bonus,broker,credit card
```

**Description**

```
Stop digging through forums for a referral code that still works.

RefCode collects referral codes into one reviewed directory. Every code is checked by a human before it appears, and codes that stop working get delisted automatically based on user reports.

FOR PEOPLE LOOKING FOR A CODE
• No account needed to browse the directory; sign in (free) to reveal and copy a code
• Browse by category — banks, brokers, shopping, delivery, streaming — or just search
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
This app does not issue rewards and never handles money. Referral rewards are paid by each merchant directly, under that merchant's own program rules. Financial products carry risk; nothing here is investment advice.

PRIVACY
No ad SDKs, no third-party analytics, no cross-app tracking. Browsing the directory requires no account; revealing and copying a code does.

Privacy policy: https://{{官網網域}}/privacy
Terms: https://{{官網網域}}/terms
Contact: {{聯絡email}}
```
