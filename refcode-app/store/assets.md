# 圖檔規格與內容規劃

兩家商店要的圖，尺寸與張數。**目前 `public/` 底下只有 Vite 的預設圖示，全部都要重做。**

---

## App icon

一張 **1024×1024 PNG、不透明、無圓角、無陰影**的母檔，其餘尺寸由工具產生。

| 平台 | 需要 |
|---|---|
| App Store | 1024×1024（商店用），app 內各尺寸由 Xcode 的 asset catalog 產生 |
| Play 商店頁 | 512×512 PNG（32-bit，含 alpha） |
| Android app 內 | adaptive icon：前景與背景兩層各 432×432，安全區是中心直徑 66% 的圓 |

產生方式（`cap add` 完之後）：

```bash
npm i -D @capacitor/assets
# resources/icon.png (1024x1024) 與 resources/splash.png (2732x2732) 放好後
npx capacitor-assets generate
```

設計上要注意的：

- **iOS 的圖示不能自己畫圓角**，系統會裁。畫了會變成雙重圓角，這是會被退件的。
- **Android adaptive icon 會被裁成圓形或圓角方形**，重要元素離邊至少留 17% 的邊距。
- 縮到 48px 還認得出來 —— 商店列表上就是那麼小。文字放圖示裡幾乎一定糊掉。
- 深色與淺色背景上都要看得清楚。

---

## 截圖

兩家的規則差很多，Play 寬鬆、Apple 嚴格。

### App Store

必須提供的尺寸（其餘尺寸 Apple 會自動縮放）：

| 裝置 | 尺寸（直式） | 張數 |
|---|---|---|
| iPhone 6.9"（最新的大尺寸） | 1290×2796 或 1320×2868 | 3~10 張，**至少 3 張** |
| iPad 13"（若支援 iPad） | 2064×2752 | 至少 3 張 |

- **不支援 iPad 的話，在 Xcode 把 target 設成 iPhone only**，就不用交 iPad 截圖。
  Ionic 的版面在 iPad 上不會壞，但要交一組截圖，第一版建議先只做 iPhone。
- 截圖裡**不能有裝置外框**，也不能出現其他平台的字樣（例如 "Available on Android"）。
- 截圖裡如果出現的畫面 app 裡沒有，會被退件 —— 加文案沒問題，但畫面本身要是真的。

### Google Play

| 項目 | 規格 | 必要 |
|---|---|---|
| 手機截圖 | 16:9 或 9:16，短邊 ≥ 320px、長邊 ≤ 3840px | **至少 2 張**（建議 4~8） |
| 主要圖片（Feature graphic） | **1024×500 PNG/JPG，不透明** | ✅ 必填 |
| 平板截圖 | 7 吋與 10 吋各一組 | 只有宣稱支援平板時才要 |
| 宣傳影片 | YouTube 連結 | 選填 |

**Feature graphic 很多人漏掉**，沒有它整個商店資訊存不了檔。
上面不要放太多字，Play 會在上面疊 app 名稱。

---

## 建議的截圖腳本

四張把價值講完，剩下的是加分。每張配一句大字標題（20 字內）。

| # | 畫面 | 標題 |
|---|---|---|
| 1 | Explore 分頁，分類與服務商列表 | 找推薦碼，不用再翻論壇 |
| 2 | 服務商頁，看得到品質分數與「N 人回報能用」 | 每個碼都經過人工審核 |
| 3 | 複製後跳出的「這個碼能用嗎」回報 | 失效的碼會被自動下架 |
| 4 | 我的碼，帶曝光 / 點擊 / 複製數字 | 上架自己的碼，看得到成效 |

拍截圖之前先讓資料好看一點：

```bash
./dev.sh all
cd refcode-api && make seed EMAIL=admin@local.test PASSWORD=admin12345
```

seed 只給本機，正式環境不能跑。截圖用的數字如果是假的，
**不要做成看起來像真實統計的樣子**（例如「已幫 12,384 人省下…」），
那是明確的不實陳述。

---

## 啟動畫面

Capacitor 的預設啟動畫面是白底加 Capacitor 的圖示，**一定要換掉**。

```bash
npm i @capacitor/splash-screen
```

母檔 `resources/splash.png` 做 2732×2732，重要內容放在中央 1200×1200 的範圍內 ——
各種螢幕比例都是從中心裁切的。深色模式要另外準備 `splash-dark.png`。

---

## 需要輸出的檔案清單

做完之後這些檔案要存在（`resources/` 建議加進 git，產生出來的原生資源不用）：

```
resources/
  icon.png            1024×1024
  splash.png          2732×2732
  splash-dark.png     2732×2732
store/assets/
  play-feature.png    1024×500
  screenshots/
    ios-6.9-1.png ~ ios-6.9-4.png
    android-1.png ~ android-4.png
```
