/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL?: string
  readonly VITE_GOOGLE_WEB_CLIENT_ID?: string
  readonly VITE_GOOGLE_IOS_CLIENT_ID?: string
  readonly VITE_APPLE_SERVICES_ID?: string
  readonly VITE_APPLE_REDIRECT_URL?: string
  readonly VITE_SUPPORT_EMAIL?: string
  readonly VITE_SITE_URL?: string
  readonly VITE_REVENUECAT_IOS_KEY?: string
  readonly VITE_REVENUECAT_ANDROID_KEY?: string
  readonly VITE_REVENUECAT_ENTITLEMENT?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
