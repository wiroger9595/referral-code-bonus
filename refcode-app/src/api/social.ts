import { Capacitor } from '@capacitor/core'
import { SocialLogin } from '@capgo/capacitor-social-login'

import type { OAuthProvider } from './types'

// 各平台的 client id。沒填的 provider 不會出現在登入頁 —— 半設定的按鈕按下去
// 只會拿到看不懂的原生錯誤，不如不要顯示。
const GOOGLE_WEB_CLIENT_ID = import.meta.env.VITE_GOOGLE_WEB_CLIENT_ID ?? ''
const GOOGLE_IOS_CLIENT_ID = import.meta.env.VITE_GOOGLE_IOS_CLIENT_ID ?? ''
const APPLE_SERVICES_ID = import.meta.env.VITE_APPLE_SERVICES_ID ?? ''
const APPLE_REDIRECT_URL = import.meta.env.VITE_APPLE_REDIRECT_URL ?? ''

// 使用者按取消不是錯誤，呼叫端要能分辨。
export class SocialLoginCancelled extends Error {}

const isNative = Capacitor.isNativePlatform()
const platform = Capacitor.getPlatform()

// Apple 登入目前整個關掉（含 iOS）。這是明知故犯：iOS 上同時提供 Google 時，
// App Store 4.8 要求必須也提供 Apple 登入，單獨拿掉 Apple、留著 Google 會在
// 送審時被退件。日後真要上架，這裡要嘛把 Apple 加回來，要嘛把 Google 也一起拿掉。
//
// Android／Web 走的是網頁版 OAuth（Services ID／redirect url），跟 iOS 原生流程
// 是分開的兩條路，這裡一併關掉，沒有半殘留著的狀態。
// 型別故意寫成 boolean 而不是給 TS 推成 false 字面量 —— 字面量 false 會讓下面
// `...(appleReady && {...})` 的展開被當成一定是 false，TS 直接判它不是物件型別。
const appleReady: boolean = false
const googleReady = platform === 'ios' ? GOOGLE_IOS_CLIENT_ID !== '' : GOOGLE_WEB_CLIENT_ID !== ''

export function availableProviders(): OAuthProvider[] {
  const list: OAuthProvider[] = []
  if (googleReady) list.push('google')
  // Apple 登入在非 Apple 平台上不是必須的，Android 只有在真的設好 Services ID 時才顯示。
  if (appleReady) list.push('apple')
  return list
}

// initialize 要打原生層，冷啟動時做只是拖慢開啟速度 —— 第一次真的有人按登入才做，
// 而且只做一次。
let initializing: Promise<void> | null = null

function ensureInitialized() {
  initializing ??= SocialLogin.initialize({
    ...(googleReady && {
      google: {
        webClientId: GOOGLE_WEB_CLIENT_ID,
        iOSClientId: GOOGLE_IOS_CLIENT_ID,
        // 後端要的是 Google 簽的 ID token，不是 server auth code，所以維持 online。
        mode: 'online',
      },
    }),
    ...(appleReady && {
      apple: {
        clientId: APPLE_SERVICES_ID,
        // iOS 走原生流程，給了 redirectUrl 反而會被導去網頁版。
        redirectUrl: platform === 'ios' ? '' : APPLE_REDIRECT_URL,
      },
    }),
  }).catch((e) => {
    initializing = null // 失敗就讓下一次重試，不要卡死在壞掉的 promise 上。
    throw e
  })

  return initializing
}

// 拿 provider 簽的 ID token 交給後端驗（後端比對 iss 與 aud，見 refcode-api 的
// internal/auth/oidc.go）。這裡不解 token，client 端解出來的 claim 不可信。
export async function fetchIdToken(provider: OAuthProvider): Promise<string> {
  await ensureInitialized()

  try {
    if (provider === 'google') {
      // email/profile/openid 是 plugin 內建的預設 scope，這裡不用再帶——帶了會被當成
      // 自訂 scope，Android 端要求 MainActivity 額外實作介面才能用，沒改就直接被拒絕。
      const { result } = await SocialLogin.login({ provider: 'google', options: {} })
      if (result.responseType !== 'online' || !result.idToken) {
        throw new Error('Google 沒有回傳 ID token')
      }
      return result.idToken
    }

    const { result } = await SocialLogin.login({ provider: 'apple', options: { scopes: ['name', 'email'] } })
    if (!result.idToken) throw new Error('Apple 沒有回傳 ID token')
    return result.idToken
  } catch (e) {
    if ((e as { code?: string }).code === 'USER_CANCELLED') throw new SocialLoginCancelled()
    throw e
  }
}

// 登出時一併清掉原生層的 session，否則下次點登入會直接用上一個帳號進來，
// 使用者會以為登出沒生效。web 上 provider session 不歸我們管，跳過。
export async function signOutProviders() {
  if (!isNative || initializing === null) return

  await Promise.allSettled(
    availableProviders().map((provider) => SocialLogin.logout({ provider })),
  )
}
