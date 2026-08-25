-- 補上 69 筆跨國服務商的 countries
--
-- 這 69 筆原本是空陣列。空陣列在 ListMerchants 裡有明確語意（db/queries/merchants.sql）：
-- 過濾時放行（任何地區都看得到）、排序時排在「在地之後、外地之前」。所以它們
-- 本來就是能正常運作的，這次改成逐家列出實際營運國家，是為了讓在地的市場把它們
-- 當在地服務商排前面，不是在補資料缺漏。
--
-- 範圍限定在平台目前經營的 8 個市場（AU/CA/HK/MO/NZ/SG/TW/US）—— 現有 230 筆
-- 只用到這些，填平台沒有使用者的國家沒有意義。
--
-- 每筆後面的註解是判斷依據與信心水準：
--   [高] 有查證或事實明確（Revolut 的支援國、Uber 退出新加坡、Amazon 沒有本地站）
--   [中] 依營運範圍推斷，尤其加密貨幣交易所的地區支援變動快、公開來源互相矛盾
--        —— 這些採保守填法，只填有明確證據的。少填的後果是那國使用者看不到，
--        多填的後果是看到卻註冊不了，後者體驗更差。
--
-- 需要複核的是所有 [中]，特別是六家券商／加密貨幣。
--
-- 執行：psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f 這個檔案

BEGIN;

UPDATE referral_code_bonus.merchants AS m
SET countries  = v.countries::text[],
    updated_at = now()
FROM (VALUES
    ('1Password', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Adobe Creative Cloud', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Backblaze', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Canva', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('ClickUp', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Dropbox', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('ExpressVPN', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Grammarly', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Hostinger', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('monday.com', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Namecheap', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('NordVPN', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Notion', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('pCloud', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Proton', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Shopify', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Squarespace', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Surfshark', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Todoist', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Zoom', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Babbel', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Brilliant', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Coursera', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Duolingo', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('MasterClass', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Preply', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Rosetta Stone', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Skillshare', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Udemy', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Fortnite', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Genshin Impact', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('PlayStation Plus', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Xbox Game Pass', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Calm', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Headspace', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('MyFitnessPal', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Oura', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('WHOOP', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Airbnb', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Booking.com', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Expedia', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Hilton Honors', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Hotels.com', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('IHG One Rewards', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Marriott Bonvoy', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Etsy', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Freecash', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('StockX', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Crunchyroll', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('McDonald''s', '{AU,CA,HK,MO,NZ,SG,TW,US}'),  -- 全球數位服務 [高]
    ('Coinbase', '{AU,CA,SG,US}'),  -- 零售法幣通道；HK/TW 無 [中]
    ('Crypto.com', '{AU,CA,HK,NZ,SG,TW,US}'),  -- 亞太覆蓋最廣的一家；MO 未確認 [中]
    ('eToro', '{AU,HK,NZ,SG,TW,US}'),  -- CA 僅部分省份，保守不列 [中]
    ('Gemini', '{AU,HK,SG,TW,US}'),  -- 亞太仍收單但正在收縮回美國市場 [中]
    ('Kraken', '{AU,CA,NZ,US}'),  -- HK/SG 狀態不明，只填明確的 [中]
    ('moomoo', '{AU,CA,HK,SG,US}'),  -- 台灣沒有 moomoo [中]
    ('Revolut', '{AU,NZ,SG,US}'),  -- 已查證：無 HK/TW/MO/CA [高]
    ('Audible', '{AU,CA,NZ,US}'),  -- 無 TW/HK/SG/MO 本地商店 [高]
    ('Paramount+', '{AU,CA,US}'),  -- 亞太多透過第三方平台上架，保守 [中]
    ('Amazon Shopping', '{AU,CA,SG,US}'),  -- 其他市場無本地站 [高]
    ('eBay', '{AU,CA,HK,NZ,SG,US}'),  -- 台灣站已關 [中]
    ('Depop', '{AU,CA,NZ,US}'),  -- 英美澳紐為主 [中]
    ('Honey', '{AU,CA,NZ,US}'),  -- 支援的商店集中在英美澳紐 [中]
    ('Hopper', '{CA,US}'),  -- 北美為主 [中]
    ('Vrbo', '{AU,CA,NZ,US}'),  -- 亞太房源少、站點未本地化 [中]
    ('Uber', '{AU,CA,HK,NZ,TW,US}'),  -- SG 已退出（併給 Grab）、MO 無 [高]
    ('Bird', '{CA,US}'),  -- 北美為主 [中]
    ('Lime', '{AU,CA,NZ,US}'),  -- 亞洲已收攤 [中]
    ('ClassPass', '{AU,CA,HK,NZ,SG,US}')  -- 需當地合作場館，TW/MO 無 [中]
) AS v(name, countries)
-- 用 name 比對：這批是上一次匯入建的，name 就是品牌名且在庫裡唯一。
-- 條件帶上 countries = '{}' 是保險 —— 萬一有人在這之間手動填過，不要蓋掉他的值。
WHERE m.name = v.name AND m.countries = '{}';

COMMIT;
