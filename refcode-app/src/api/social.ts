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

// iOS 的 Sign in with Apple 是系統原生的，不需要 Services ID；
// web 與 Android 走的是 Apple 的網頁流程，兩個都要填才算設定完成。
const appleReady = platform === 'ios' || (APPLE_SERVICES_ID !== '' && APPLE_REDIRECT_URL !== '')
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
      const { result } = await SocialLogin.login({ provider: 'google', options: { scopes: ['email', 'profile'] } })
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
