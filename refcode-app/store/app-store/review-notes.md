# App Store 審核備註

App Store Connect → 版本資訊 → 「審核備註（Notes for Review）」要貼的內容。

**這欄不要空著。** 目錄型 / 聚合型的 app 最容易被以「功能過於單薄」（4.2）
或 UGC 要件不足（1.2）退件，主動把說明寫在前面，比被退了再申訴省一個禮拜。

備註欄只吃純文字，貼下面方框裡的內容（英文，審核員多半不讀中文）。

---

## 測試帳號

「登入資訊」那一區要勾 **Sign-in required**，並填：

| 欄位 | 值 |
|---|---|
| Username | `{{審核用帳號email}}` |
| Password | `{{審核用帳號密碼}}` |

這個帳號要**事先在正式環境建好**，而且要先上架一兩個推薦碼、狀態是 `active`，
讓審核員一進去「我的碼」就看得到東西。空帳號會讓人以為功能沒做完。

> 用 `make seed` 建的 demo 資料只給本機，正式環境不要跑（`APP_ENV=production` 時它會拒絕執行）。
> 審核帳號要另外手動建。

---

## 備註內文

```
ABOUT THIS APP

RefCode is a directory of referral codes. Browsing the merchant and code list requires no
account; an account is needed to reveal a code's actual content and copy it, to publish
your own code, or to see performance stats.

The app does NOT issue rewards and never handles money. Referral rewards are paid by
each merchant directly to both parties under that merchant's own program rules. We only
match supply and demand and rank the listings.

HOW TO TEST

1. Open the app — the Explore tab lists merchants by category. No login required.
2. Tap any merchant to see its referral codes, each with a quality score and recent
   "worked / failed" report counts. Without signing in, the code's actual content is
   masked with a "Sign in to reveal" button — this is intentional (see ABOUT THIS APP).
3. Sign in with the account provided above, then open the same merchant again. The code
   is now shown in full, with Copy and "go to sign-up" buttons.
4. Tap Copy on a code. A follow-up prompt asks whether the code worked — this is our
   user reporting mechanism for user-generated content.
5. Tap the sign-up button to open that merchant's registration page in an in-app browser.
6. Go to "My Codes" to see published codes and their impression / click / copy statistics.
7. Use "Add code" to publish a new one. It enters a PENDING state and is not publicly
   visible until a human reviewer approves it.

USER-GENERATED CONTENT (Guideline 1.2)

Referral codes and their notes are user-generated. Our moderation stack:

- Pre-publication review: every submitted code enters a pending queue and is manually
  approved by a moderator before it becomes visible to anyone. Nothing appears unmoderated.
- In-app reporting: after signing in and copying a code, users can report it as failed,
  invalid, or merchant-discontinued.
- Automatic delisting: a code whose recent reports exceed our failure threshold is taken
  down automatically without waiting for a moderator.
- Mandatory expiry: every code must carry an expiry date and is delisted when it passes.
- Contact: {{聯絡email}}, also reachable from the Account tab in the app. We act on
  reports of objectionable content within 24 hours.
- Terms of use with a zero-tolerance policy for objectionable content and abusive users:
  https://{{官網網域}}/terms

WHY THIS IS NOT JUST A LIST OF LINKS (Guideline 4.2)

The app's substance is the moderation and ranking layer, not the links:

- A curated merchant directory that only our staff can add to — users cannot create
  merchant entries, which is what keeps the directory from filling with duplicates.
- A quality score computed per code from real user reports, which drives both ranking
  and automatic delisting.
- Weighted randomised rotation so that newly published codes get exposure instead of
  being buried permanently behind the earliest ones.
- Per-code analytics for publishers (impressions, clicks, copies over time).

None of this exists on the merchants' own websites.

ACCOUNT DELETION (Guideline 5.1.1(v))

Account > Delete account. Deletion is immediate and permanent, and delists any codes the
account published. The same is available at https://{{官網網域}}/delete-account without
installing the app.

SIGN IN WITH APPLE (Guideline 4.8)

Sign in with Apple is offered alongside Google Sign-In and email. Apple's Hide My Email
relay addresses are fully supported — we do not filter by email domain.

SUBSCRIPTION (RefCode Pro)

The app offers one auto-renewable subscription, "RefCode Pro", in two durations (monthly and
yearly), managed through RevenueCat. It unlocks app features only:

- Unlimited active listings (free accounts are capped at 3)
- Full per-code performance history
- Priority position in the moderation queue

It does NOT buy rewards, ad placement, or anything outside the app. No physical goods.

HOW TO TEST THE SUBSCRIPTION

1. Sign in with the account above (a subscription must be tied to an account — the paywall is
   not reachable while signed out, because an anonymous purchase could not be attached to a user).
2. Account tab > "Upgrade to Pro" opens the paywall. Prices come from App Store Connect.
3. Purchase with a sandbox account. The entitlement unlocks immediately on device; our backend
   is updated by a RevenueCat webhook a moment later.
4. "Restore purchases" is on the same screen and also works on a fresh install.
5. To see the gate itself: on a free account, publish 3 codes, then try a 4th — the app routes
   to the paywall instead of showing an error.

Auto-renew terms, price, renewal timing and how to cancel are disclosed on the paywall itself,
directly above the purchase button.

PRIVACY

No advertising SDKs, no third-party analytics, no IDFA access, no cross-app tracking, so
no ATT prompt is presented. A random per-install UUID is used solely to prevent the same
device from submitting duplicate reports.

FINANCIAL MERCHANTS

Some merchants in the directory are banks and brokerages. We are not a financial services
provider, we do not offer accounts or investment products, and the app contains no
investment advice. The listings only point to the merchants' own public sign-up pages.

CONTACT

{{聯絡email}}
```

---

## 送審前要先確認

- [ ] 審核帳號在**正式環境**可以登入，而且有 active 的推薦碼
- [ ] 正式 API 是 HTTPS（iOS 的 ATS 會擋純 HTTP）
- [ ] 備註裡提到的每個網址都打得開：`/terms`、`/delete-account`、`/privacy`
- [ ] 備註裡寫的「帳號頁可以刪除帳號」**在 app 裡真的做得到** ——
      這句話寫了但功能不存在，會被當成不實陳述，比單純缺功能更嚴重
- [ ] 訂閱的兩個 product 都已在 App Store Connect 建好、狀態是「準備提交」，
      而且**跟這個版本綁在一起送審** —— 沒綁的話 app 過了但買不到東西
- [ ] 用 sandbox 帳號實際買過一次、也實際「恢復購買」過一次

## 出口合規

首次上傳建置版本時會問加密相關的問題。本 app 只用 HTTPS，屬於豁免範圍，
在 `Info.plist` 加這一行可以讓每次上傳都不用重答：

```xml
<key>ITSAppUsesNonExemptEncryption</key>
<false/>
```
