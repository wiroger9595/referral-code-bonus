// 跟 refcode-app 的 src/images.ts 同一套規則：後端大頭照上限 2MB，
// 手機或相機拍出來的原檔幾乎都超過，而畫面上最大也只顯示到 80px。
const AVATAR_MAX_EDGE = 512
const AVATAR_QUALITY = 0.85

// toAvatarBlob 把選到的圖等比縮到長邊 512px 的 JPEG。
// 任何一步做不到就退回原檔——讓後端的大小限制去擋，不要在這裡直接失敗。
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
