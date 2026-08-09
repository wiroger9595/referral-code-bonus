// 目錄要不要限縮在使用者的所在地。
//
// 匿名訪客一律不篩：官網的 SSR 內容不能因人而異，否則 Googlebot 從不同機房
// 爬到的頁面會長得不一樣，hreflang 與收錄都會跟著亂。所以只有「登入而且填了
// 所在地」的人才會被篩，後端沒收到 ?region= 時也是照這個規則退回（見
// refcode-api 的 resolveRegion）。
export function useRegionFilter() {
  const { user } = useAuth()

  // 用 cookie 而不是 localStorage：SSR 階段就要知道要不要篩，否則伺服器先吐
  // 一份篩過的清單、hydrate 之後才跳成全部，畫面會閃一下。
  const showAll = useCookie<boolean>('refcode_all_regions', {
    default: () => false,
    sameSite: 'lax',
    maxAge: 60 * 60 * 24 * 365,
  })

  // 沒填所在地就沒有東西可篩，那個開關也就沒有意義，不要顯示。
  const canFilter = computed(() => Boolean(user.value?.country))

  // undefined 時 Nuxt 不會把這個 key 放進 query，等於交給後端自己決定。
  const regionQuery = computed(() => (showAll.value ? 'all' : undefined))

  return { showAll, canFilter, regionQuery }
}
