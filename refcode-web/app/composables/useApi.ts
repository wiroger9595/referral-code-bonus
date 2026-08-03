import type { ReportResult } from '~/types/api'

const DEVICE_KEY = 'refcode_device_id'

// 匿名去重的依據。後端沒收到這個 header 會退回 IP + UA，
// 精準度差很多（同一個網咖/公司網路會被當成同一台）。
export function useDeviceId(): string {
  if (import.meta.server) return ''

  let id = localStorage.getItem(DEVICE_KEY)
  if (!id) {
    id = crypto.randomUUID()
    localStorage.setItem(DEVICE_KEY, id)
  }
  return id
}

function clientHeaders(): Record<string, string> {
  const id = useDeviceId()
  return id ? { 'X-Device-ID': id } : {}
}

// 只在瀏覽器端呼叫：這些都是使用者動作觸發的，SSR 階段不該送。
export function useTracking() {
  const { public: cfg } = useRuntimeConfig()

  async function track(codeId: string, eventType: 'click' | 'copy') {
    if (import.meta.server) return
    try {
      await $fetch('/v1/events', {
        baseURL: cfg.apiBase,
        method: 'POST',
        headers: clientHeaders(),
        body: { code_id: codeId, event_type: eventType },
      })
    } catch {
      // 統計掉一筆不值得打斷使用者的操作。
    }
  }

  async function report(codeId: string, result: ReportResult) {
    await $fetch(`/v1/codes/${codeId}/reports`, {
      baseURL: cfg.apiBase,
      method: 'POST',
      headers: clientHeaders(),
      body: { result },
    })
  }

  return { track, report }
}
