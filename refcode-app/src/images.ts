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
