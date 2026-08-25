import {
  airplaneOutline,
  appsOutline,
  bagHandleOutline,
  barbellOutline,
  carOutline,
  cardOutline,
  cellularOutline,
  cloudOutline,
  constructOutline,
  fastFoodOutline,
  gameControllerOutline,
  playCircleOutline,
  schoolOutline,
  shieldCheckmarkOutline,
  trendingUpOutline,
} from 'ionicons/icons'

// 分類磁磚的圖示。分類在後端只有 id 與 name（沒有 slug），沒有可以對應圖示的
// 穩定欄位，所以只能拿名稱去比。比不到就給通用圖示，不會壞掉，但要新增分類時
// 記得回來補一條 —— 正解是後端在分類上加一個 icon 欄位。
//
// 探索頁與分類頁都要用同一份，放在這裡而不是各自複製，不然新增分類時
// 只補了一邊，兩頁的同一個分類會長出兩種圖示。
const CATEGORY_ICONS: { match: RegExp; icon: string }[] = [
  { match: /銀行|信用卡|カード|bank|card/i, icon: cardOutline },
  { match: /券商|投資|証券|invest|broker|stock/i, icon: trendingUpOutline },
  { match: /保險|保险|保険|insurance/i, icon: shieldCheckmarkOutline },
  { match: /外送|外食|デリバリー|delivery|food/i, icon: fastFoodOutline },
  { match: /影音|串流|動画|音楽|stream|video|music/i, icon: playCircleOutline },
  { match: /電信|通訊|通信|携帯|telecom|mobile/i, icon: cellularOutline },
  { match: /旅遊|訂房|旅行|ホテル|travel|hotel|flight/i, icon: airplaneOutline },
  { match: /購物|電商|通販|ショッピング|shop|retail|commerce/i, icon: bagHandleOutline },
  { match: /遊戲|ゲーム|game/i, icon: gameControllerOutline },
  { match: /軟體|訂閱|クラウド|saas|software|cloud/i, icon: cloudOutline },
  { match: /交通|出行|移動|ride|transport/i, icon: carOutline },
  { match: /健康|運動|フィットネス|health|fitness/i, icon: barbellOutline },
  { match: /學習|教育|学習|learning|education/i, icon: schoolOutline },
  { match: /工具|生產力|効率|productivity|business/i, icon: constructOutline },
]

export function categoryIcon(name: string): string {
  return CATEGORY_ICONS.find((c) => c.match.test(name))?.icon ?? appsOutline
}
