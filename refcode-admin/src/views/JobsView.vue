<script setup lang="ts">
import {
  NAlert,
  NButton,
  NCard,
  NEmpty,
  NModal,
  NSelect,
  NSpace,
  NSpin,
  NSwitch,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import { computed, onMounted, onUnmounted, ref } from 'vue'

import { ApiError, api } from '../api/client'
import type { JobRun, ScheduledJob } from '../api/types'

const message = useMessage()

const jobs = ref<ScheduledJob[]>([])
const loading = ref(false)
const loadError = ref('')
// 個別列在送出時要能各自轉圈，用 job 名稱當鍵而不是一個全域 flag。
const busy = ref(new Set<string>())

const runsOf = ref<ScheduledJob | null>(null)
const runs = ref<JobRun[]>([])
const runsLoading = ref(false)

// 間隔用下拉而不是讓人自己填秒數：這些排程的合理值就那幾種，
// 而填錯的代價是拿伺服器去連打外站。
const intervalOptions = [
  { label: '每 5 分鐘', value: 300 },
  { label: '每小時', value: 3600 },
  { label: '每 6 小時', value: 21600 },
  { label: '每天', value: 86400 },
  { label: '每 3 天', value: 259200 },
  { label: '每週', value: 604800 },
]

// 有排程在跑（或剛被按下立即執行）時自動重整，不然要手動 F5 才看得到結果。
const POLL_MS = 5000
let timer: number | undefined

const anyPending = computed(() =>
  jobs.value.some((j) => j.last_status === 'running' || j.run_requested_at !== null),
)

async function load(quiet = false) {
  if (!quiet) loading.value = true
  loadError.value = ''
  try {
    jobs.value = (await api.listJobs()).jobs
  } catch (e) {
    loadError.value = e instanceof ApiError ? e.message : '載入失敗'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void load()
  timer = window.setInterval(() => {
    // 沒有東西在跑就不用一直打 API。
    if (anyPending.value) void load(true)
  }, POLL_MS)
})
onUnmounted(() => window.clearInterval(timer))

function setBusy(name: string, on: boolean) {
  const next = new Set(busy.value)
  if (on) next.add(name)
  else next.delete(name)
  busy.value = next
}

async function patch(job: ScheduledJob, body: { enabled?: boolean; interval_seconds?: number }) {
  setBusy(job.name, true)
  try {
    await api.updateJob(job.name, body)
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '更新失敗')
  } finally {
    // 不管成功失敗都重抓：失敗時畫面要拉回後端的真實狀態，
    // 不要留著 switch 已經撥過去的假象。
    await load(true)
    setBusy(job.name, false)
  }
}

async function runNow(job: ScheduledJob) {
  setBusy(job.name, true)
  try {
    await api.runJob(job.name)
    message.success('已排入，最多 30 秒後開始')
    await load(true)
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '觸發失敗')
  } finally {
    setBusy(job.name, false)
  }
}

async function openRuns(job: ScheduledJob) {
  runsOf.value = job
  runsLoading.value = true
  runs.value = []
  try {
    runs.value = (await api.listJobRuns(job.name)).runs
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '載入紀錄失敗')
  } finally {
    runsLoading.value = false
  }
}

function fmtTime(v: string | null) {
  return v ? new Date(v).toLocaleString('zh-TW') : '—'
}

function fmtInterval(sec: number) {
  const known = intervalOptions.find((o) => o.value === sec)
  if (known) return known.label
  if (sec % 86400 === 0) return `每 ${sec / 86400} 天`
  if (sec % 3600 === 0) return `每 ${sec / 3600} 小時`
  return `每 ${Math.round(sec / 60)} 分鐘`
}

function statusTag(status: string) {
  if (status === 'ok') return { type: 'success' as const, text: '成功' }
  if (status === 'failed') return { type: 'error' as const, text: '失敗' }
  if (status === 'running') return { type: 'warning' as const, text: '執行中' }
  return { type: 'default' as const, text: '尚未執行' }
}
</script>

<template>
  <div class="page">
    <NAlert v-if="loadError" type="error" :title="loadError" style="margin-bottom: 16px" />

    <NAlert type="info" :bordered="false" style="margin-bottom: 16px">
      排程由 API 自己輪詢執行，關掉之後就不會再跑。按「立即執行」只是排進去，最多 30
      秒後才會真的開始。爬蟲類的排程（App Store 匯入、補 logo）會連到外站，間隔不要調得太密。
    </NAlert>

    <NSpin :show="loading">
      <NEmpty v-if="!loading && jobs.length === 0" description="沒有任何排程" />

      <NSpace v-else vertical :size="12">
        <NCard v-for="job in jobs" :key="job.name" size="small">
          <div class="row">
            <div class="info">
              <div class="title">
                <NText strong>{{ job.name }}</NText>
                <NTag size="small" :type="statusTag(job.last_status).type" :bordered="false">
                  {{ statusTag(job.last_status).text }}
                </NTag>
                <NTag v-if="job.run_requested_at" size="small" type="info" :bordered="false">
                  已排入，等待執行
                </NTag>
              </div>
              <NText depth="3" class="desc">{{ job.description }}</NText>

              <div class="meta">
                <span>上次開始：{{ fmtTime(job.last_run_at) }}</span>
                <span>結束：{{ fmtTime(job.last_finished_at) }}</span>
                <span>{{ fmtInterval(job.interval_seconds) }}</span>
                <span v-if="job.interval_overridden" class="overridden">間隔已人工調整</span>
              </div>

              <NText v-if="job.last_summary" depth="2" class="summary">
                {{ job.last_summary }}
              </NText>
              <NAlert v-if="job.last_error" type="error" :bordered="false" class="summary">
                {{ job.last_error }}
              </NAlert>
            </div>

            <div class="controls">
              <NSwitch
                :value="job.enabled"
                :loading="busy.has(job.name)"
                @update:value="(v: boolean) => patch(job, { enabled: v })"
              />
              <NSelect
                :value="job.interval_seconds"
                :options="intervalOptions"
                :disabled="busy.has(job.name)"
                style="width: 130px"
                @update:value="(v: number) => patch(job, { interval_seconds: v })"
              />
              <NButton
                size="small"
                :disabled="!job.enabled || job.run_requested_at !== null"
                :loading="busy.has(job.name)"
                @click="runNow(job)"
              >
                立即執行
              </NButton>
              <NButton size="small" quaternary @click="openRuns(job)">紀錄</NButton>
            </div>
          </div>
        </NCard>
      </NSpace>
    </NSpin>

    <NModal
      :show="runsOf !== null"
      preset="card"
      style="width: 720px"
      :title="`執行紀錄 — ${runsOf?.name ?? ''}`"
      @update:show="
        (v: boolean) => {
          if (!v) runsOf = null
        }
      "
    >
      <NSpin :show="runsLoading">
        <NEmpty v-if="!runsLoading && runs.length === 0" description="還沒有執行過" />
        <NSpace v-else vertical :size="8">
          <div v-for="run in runs" :key="run.id" class="run">
            <div class="run-head">
              <NTag size="small" :type="statusTag(run.status).type" :bordered="false">
                {{ statusTag(run.status).text }}
              </NTag>
              <NTag
                size="small"
                :bordered="false"
                :type="run.trigger === 'manual' ? 'info' : 'default'"
              >
                {{ run.trigger === 'manual' ? '手動' : '排程' }}
              </NTag>
              <NText depth="3">{{ fmtTime(run.started_at) }}</NText>
            </div>
            <NText v-if="run.summary" depth="2">{{ run.summary }}</NText>
            <NText v-if="run.error" type="error">{{ run.error }}</NText>
            <NText v-if="!run.summary && !run.error && run.status === 'ok'" depth="3">
              這輪沒有需要處理的資料
            </NText>
          </div>
        </NSpace>
      </NSpin>
    </NModal>
  </div>
</template>

<style scoped>
.page {
  max-width: 1000px;
}

.row {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  justify-content: space-between;
}

.info {
  min-width: 0;
  flex: 1;
}

.title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.desc {
  display: block;
  font-size: 13px;
}

.meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 6px;
  font-size: 12px;
  color: var(--n-text-color-3, #999);
}

.overridden {
  color: #d97706;
}

.summary {
  display: block;
  margin-top: 8px;
  font-size: 13px;
}

.controls {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 8px;
}

.run {
  padding: 8px 0;
  border-bottom: 1px solid var(--n-border-color, #eee);
}

.run-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}
</style>
