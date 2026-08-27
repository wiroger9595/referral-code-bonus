// 手機相簿挑出來的照片動輒 3–5MB，後端的大頭照上限是 2MB，直接傳幾乎一定失敗。
// 而且大頭照顯示出來只有 96px，傳原檔是白花使用者的流量。
const AVATAR_MAX_EDGE = 512
const AVATAR_QUALITY = 0.85

// toAvatarBlob 把選到的圖等比縮到長邊 512px 的 JPEG。
// 任何一步做不到就退回原檔——寧可讓後端用大小限制擋掉，也不要在這裡直接失敗。
export async function toAvatarBlob(file: File): Promise<Blob> {
  // from-image 是為了吃 EXIF 的方向資訊，不然直立拍的照片會躺著。
  const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' }).catch(() => null)
  if (!bitmap) return file

  try {
    const scale = Math.min(1, AVATAR_MAX_EDGE / Math.max(bitmap.width, bitmap.height))
    const width = Math.round(bitmap.width * scale)
    const height = Math.round(bitmap.height * scale)

    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const ctx = canvas.getContext('2d')
    if (!ctx) return file
    ctx.drawImage(bitmap, 0, 0, width, height)

    const blob = await new Promise<Blob | null>((resolve) => {
      canvas.toBlob(resolve, 'image/jpeg', AVATAR_QUALITY)
    })
    return blob ?? file
  } finally {
    bitmap.close()
  }
}

// Cloudinary 存回來的是上傳當下的原檔 URL（見 refcode-api 的 cloudinary.Upload，
// 那邊沒有加任何 transformation），長這樣：
//   https://res.cloudinary.com/<cloud>/image/upload/v1712345678/referral_code_bonus/merchants/xxx.png
// 一張只顯示 46px 的 logo 很可能是 1024px 的 PNG，WebView 照原尺寸解碼，
// 探索頁一次列幾十張就是幾十 MB 的 bitmap —— 這正是 Google Play 點陣圖用量
// 門檻在抓的形態。transformation 插在 /image/upload/ 後面，讓 Cloudinary 直接
// 出對的尺寸與格式。
const CLOUDINARY_UPLOAD = '/image/upload/'

// 最多算到 3 倍螢幕。更高的裝置有，但那點畫質差異換不到多一倍的解碼記憶體。
const MAX_DPR = 3

// thumb 把遠端圖片網址換成「剛好夠用的尺寸」。
// 非 Cloudinary 的網址（例如 Google 帳號帶回來的頭像）原樣回傳。
export function thumb(url: string | null | undefined, size: number): string | undefined {
  if (!url) return undefined

  const at = url.indexOf(CLOUDINARY_UPLOAD)
  if (at < 0) return url

  const head = url.slice(0, at + CLOUDINARY_UPLOAD.length)
  const tail = url.slice(at + CLOUDINARY_UPLOAD.length)

  // 只認「版本號開頭」這一種形狀。已經帶了 transformation 的再疊一層不會出錯，
  // 但同一張圖會產生兩種網址、各佔一份 CDN 與裝置快取。認不出來就原樣回傳 ——
  // 少省一點記憶體，比把網址弄壞好。
  if (!/^v\d+\//.test(tail)) return url

  const edge = Math.round(size * Math.min(window.devicePixelRatio || 1, MAX_DPR))
  return `${head}c_fill,w_${edge},h_${edge},f_auto,q_auto/${tail}`
}
