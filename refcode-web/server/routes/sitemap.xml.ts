interface SitemapEntry {
  slug: string
  updated_at: string
}

interface Category {
  id: string
}

// 跟 nuxt.config 的 i18n.locales 對齊。前綴策略是 prefix_except_default，
// 所以中文沒有前綴 —— 這裡多一個 prefix 欄位就是為了那個例外。
// 日文先停用，跟 nuxt.config 的 i18n.locales 一起拿掉的。
const LOCALES = [
  { code: 'zh-Hant-TW', prefix: '' },
  { code: 'en', prefix: '/en' },
]

interface Page {
  path: string
  priority: string
  lastmod?: string
}

// 服務商頁是這個站唯一有長尾搜尋價值的東西，sitemap 直接從 API 撈，
// 不另外維護一份清單（維護兩份一定會分岔）。
export default defineEventHandler(async (event) => {
  const { public: cfg } = useRuntimeConfig()

  const [merchants, categories] = await Promise.all([
    $fetch<{ entries: SitemapEntry[] }>('/v1/merchants/sitemap', { baseURL: cfg.apiBase }),
    $fetch<{ categories: Category[] }>('/v1/categories', { baseURL: cfg.apiBase }),
  ])

  const pages: Page[] = [
    { path: '', priority: '1.0' },
    { path: '/about', priority: '0.3' },
    { path: '/support', priority: '0.3' },
    ...categories.categories.map((c) => ({ path: `/category/${c.id}`, priority: '0.7' })),
    ...merchants.entries.map((m) => ({
      path: `/referral/${m.slug}`,
      priority: '0.9',
      lastmod: m.updated_at,
    })),
  ]

  // 每個頁面在每種語言各出現一次，而且每一筆都要列出全部語言的 alternate。
  // 只列自己那一種等於沒告訴 Google 它們是同一頁的不同語言版本。
  const body = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">
${pages
  .flatMap((p) =>
    LOCALES.map(
      (l) => `  <url>
    <loc>${cfg.siteUrl}${l.prefix}${p.path}</loc>${
      p.lastmod ? `\n    <lastmod>${p.lastmod}</lastmod>` : ''
    }
${LOCALES.map(
  (alt) =>
    `    <xhtml:link rel="alternate" hreflang="${alt.code}" href="${cfg.siteUrl}${alt.prefix}${p.path}" />`,
).join('\n')}
    <priority>${p.priority}</priority>
  </url>`,
    ),
  )
  .join('\n')}
</urlset>`

  setHeader(event, 'content-type', 'application/xml; charset=utf-8')
  return body
})
