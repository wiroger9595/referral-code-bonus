// robots.txt 要帶 sitemap 的絕對網址，而網域是環境變數決定的（本機 localhost、
// 正式站是部署的網域，之後換自訂網域還會再變一次）。原本這是 public/ 底下的
// 靜態檔，網址寫死在裡面 —— 換過網域就會指向不存在的地方，而且沒有人會發現，
// 因為它不影響任何頁面顯示。改成路由跟 sitemap.xml.ts 讀同一個 siteUrl，
// 兩邊就不可能再分岔。
export default defineEventHandler((event) => {
  const { public: cfg } = useRuntimeConfig()

  setHeader(event, 'content-type', 'text/plain; charset=utf-8')
  return `User-Agent: *
Disallow:

Sitemap: ${cfg.siteUrl}/sitemap.xml
`
})
