# Privacy Policy

**Effective date: {{生效日期}}**
**Last updated: {{生效日期}}**

This policy explains how RefCode (the "Service") collects, uses and protects your personal data.
The Service is provided by wiwilab ("we", "us") as an individual developer.

Questions: robertsmart1989@gmail.com

> 中文版為主要版本，兩份內容不一致時以中文版為準。
> The Traditional Chinese version prevails in case of any discrepancy.

---

## 1. Data controller

| | |
|---|---|
| Controller | wiwilab (individual developer) |
| Contact | robertsmart1989@gmail.com |
| Website | https://{{官網網域}} |

We have not appointed a Data Protection Officer. All privacy requests are handled at the address above.

## 2. What we collect

### 2.1 Without an account

| Data | Detail | Purpose |
|---|---|---|
| Device identifier | A random UUID generated on first launch and stored on your device | Prevents the same device from submitting duplicate "did this code work?" reports |
| Usage data | Which merchants and referral codes you viewed, and when (the code's actual content is only shown once you sign in — see 2.2) | Computes impression counts and quality scores that drive ranking |
| IP address | Stored as a hash | Abuse detection and de-duplication |

This UUID is **not** Apple's IDFA and **not** the Android Advertising ID — we do not access either.
It is meaningful only inside this Service, does not track you across apps or websites, and is
destroyed when you uninstall the app.

### 2.2 With an account

Browsing the list of merchants and referral codes requires no account. An account is
required to reveal and copy a code's actual content, and to list your own codes or
view their performance.

| Data | Source |
|---|---|
| Email address | Provided at sign-up, or received from Google / Apple Sign-In |
| Display name | Provided at sign-up, or received from Google / Apple; shown publicly alongside codes you list |
| Password | Email sign-up only. Stored as a one-way hash and **cannot be reversed** |
| Profile photo | From Google / Apple Sign-In if you allow it, or an image you upload yourself. Images are hosted on Cloudinary and shown publicly alongside your display name |
| Google / Apple user identifier | Used to recognise you on return visits |
| Location (country) | **Optional.** Chosen by you from a country list at sign-up, and changeable or removable at any time from the Account tab. We use it to rank merchants available in your country higher in the directory; leaving it blank affects nothing else. **This is a country you select yourself, not derived from device location** — the app never requests or uses any location permission |

If you use Sign in with Apple with "Hide My Email", we receive only Apple's relay address
(`@privaterelay.appleid.com`) and cannot learn your real address.

### 2.3 Content you submit

Referral codes, notes and expiry dates you list, plus reports you submit about other people's codes.
**Codes, notes, your display name and your profile photo are shown publicly** to all users
including signed-out visitors, and may appear on our website and be indexed by search engines.
Do not put anything in the note field, or use any photo, that you would not want to be public.

A profile photo is optional and **is only uploaded when you actively pick an image**. It is
re-encoded on your device to a longest edge of 512 pixels, which normally also strips the
EXIF metadata (such as capture location and time) carried by the original photo.

### 2.4 Subscriptions (paying users only)

If you purchase RefCode Pro, we store the subscription status, plan, expiry date and the
subscription identifier provided by RevenueCat.

**We never receive or store your payment method or card number.** The transaction happens
entirely inside the Apple App Store or Google Play; we only learn whether the account has
an active subscription.

### 2.5 What we do not collect

We do not collect your legal name (other than a display name), phone number, address, date of
birth, gender, precise or coarse location, contacts, microphone, health data, or financial
account information.

**We do not read your photo library.** Setting a profile photo opens your device's own picker,
and the Service receives only the single image you select there — never anything else in your
library.

**We never handle reward money.** Rewards are paid by each merchant directly to you and the
other party; the Service only matches supply and demand.

## 3. How we use it

- Operating the Service: showing the code directory, letting you list and manage your codes
- Account management: registration, sign-in, keeping you signed in
- Quality and ranking: computing quality scores from reports and impressions, automatic delisting
- Abuse prevention: blocking duplicate reports, vote stuffing and ranking manipulation
- Analytics: understanding overall usage to improve the Service

We do **not** use your data for personalised advertising and do **not** track you across other
companies' apps or websites.

## 4. Legal bases (where GDPR applies)

| Purpose | Basis |
|---|---|
| Providing the Service, account management | Performance of a contract, Art. 6(1)(b) |
| Abuse prevention, quality control, analytics | Legitimate interests, Art. 6(1)(f) — the credibility of the listings is the core of the Service |
| Retention required by law | Legal obligation, Art. 6(1)(c) |

## 5. Sharing

**We do not sell your personal data and do not share it with third parties for marketing.**

Data leaves our systems only in these cases:

| Recipient | What | Why |
|---|---|---|
| Cloud hosting provider | All stored data, as the place where it resides | The Service needs servers |
| Google / Apple | Information required to verify a sign-in | Only if you choose to sign in with them |
| RevenueCat, Inc. | Your account identifier and subscription status | Handles subscription validation and renewal state on our behalf. Only applies to subscribers |
| Cloudinary Ltd. | The profile photo you upload | Stores and delivers images on our behalf. Only applies to users who upload one |
| Authorities | As required by law | Legal compliance |

The app contains **no third-party advertising or analytics SDKs** — no Google Analytics,
no Firebase Analytics, no Meta SDK, no crash-reporting service — and we never track you
across apps or websites for advertising.

The only third parties that receive data are **RevenueCat** (subscription billing),
**Cloudinary** (profile photo hosting) and whichever sign-in provider you choose. All three
process it solely to deliver that function.

When you tap "sign up" for a merchant, we open that merchant's website in an in-app browser.
Once you leave the Service, that company's privacy policy applies and we have no control over
what they collect.

## 6. Where data is stored

Data is stored on servers in {{資料存放地區，例如：Taiwan}}. If you are in the European Economic
Area, your data may be transferred outside the EEA for processing; we apply appropriate safeguards.

## 7. Retention

| Data | Retained |
|---|---|
| Account data | Until you delete your account |
| Referral codes you listed | Delisted and deleted when you delete your account |
| Usage events (impression / click / copy) | {{保存月數}} months. On account deletion the user identifier is stripped immediately; only anonymised records remain |
| Reports | For the lifetime of the corresponding code |
| Subscription records | Retained for a period after the subscription ends, for accounting and refund disputes |
| Profile photo | Until you replace it or delete your account. **Replacing it or deleting your account also deletes the image file from our image provider (Cloudinary)**; copies already cached by browsers or CDNs disappear as those caches expire |
| Device identifier | On your device only; gone when you uninstall |

## 8. Your rights

You may request access, rectification, erasure, restriction, objection, and withdrawal of consent.

**Deleting your account**: use the Account screen in the app, or visit
https://{{官網網域}}/delete-account to do it yourself without installing the app.
Both routes delete **immediately and permanently** — no grace period, no recovery:
your account, email, display name, every code you published, and all sessions on all devices.

**Deleting your account does not cancel a subscription.** If you have Pro, cancel it first
in your App Store or Google Play settings; we have no permission to cancel it for you.

For anything else, email robertsmart1989@gmail.com. We respond within 30 days.


You may lodge a complaint with your local data protection authority.

## 9. Children

The Service is not directed at children under 13 and we do not knowingly collect their data.
If you believe a child has provided us with personal data, email robertsmart1989@gmail.com and we will delete it.

## 10. Security

- All traffic to our servers uses HTTPS
- Passwords are stored as one-way hashes; we cannot see your original password
- IP addresses are hashed before storage
- Sign-in credentials are kept in the device's secure system storage

No system is perfectly secure, but we maintain these measures continuously.

## 11. Changes

We will update the "Last updated" date on this page. Material changes — such as collecting a new
category of data — will additionally be announced in the app or by email.

## 12. Contact

wiwilab
robertsmart1989@gmail.com
https://{{官網網域}}
