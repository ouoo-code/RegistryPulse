<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { adminApi } from '../../admin-api'
import type { AdminProxy, ProxyMetrics } from '../../api'
import { useI18n } from '../../i18n'
import { formatDateTime } from '../../time'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ error: [message: string] }>()
const { t } = useI18n()
const proxyData = ref<AdminProxy | null>(null)
const metrics = ref<ProxyMetrics | null>(null)
const loading = ref(false)
let refreshTimer: number | undefined

const statusKind = computed(() => {
  const data = proxyData.value
  if (!data?.status_available) return 'unavailable'
  if (!data.status.running) return 'stopped'
  if (!data.status.enabled) return 'disabled'
  return data.status.ready ? 'ready' : 'no-route'
})

const statusLabel = computed(() => {
  switch (statusKind.value) {
    case 'ready': return t.value.proxyStatusReady
    case 'no-route': return t.value.proxyStatusNoRoute
    case 'disabled': return t.value.proxyStatusDisabled
    case 'stopped': return t.value.proxyStatusStopped
    default: return t.value.proxyStatusUnavailable
  }
})

const responseTotal = computed(() => {
  const value = metrics.value
  if (!value) return 0
  return value.responses_1xx + value.responses_2xx + value.responses_3xx + value.responses_4xx + value.responses_5xx
})

const successRate = computed(() => {
  const requests = metrics.value?.requests || 0
  if (!requests) return '—'
  return `${((metrics.value?.successes || 0) / requests * 100).toFixed(1)}%`
})

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value < 1) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let amount = value
  let index = 0
  while (amount >= 1024 && index < units.length - 1) { amount /= 1024; index += 1 }
  return `${amount >= 10 || index === 0 ? amount.toFixed(0) : amount.toFixed(1)} ${units[index]}`
}

function formatDuration(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) return '—'
  if (seconds < 1) return `${Math.round(seconds * 1000)} ms`
  return `${seconds.toFixed(2)} s`
}

function fail(error: unknown) {
  emit('error', error instanceof Error ? error.message : t.value.proxyMonitorUnavailable)
}

async function refresh() {
  if (loading.value) return
  loading.value = true
  try {
    const [currentProxy, currentMetrics] = await Promise.all([adminApi.proxy(props.token), adminApi.proxyMetrics(props.token)])
    proxyData.value = currentProxy
    metrics.value = currentMetrics
  } catch (error) {
    fail(error)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void refresh()
  refreshTimer = window.setInterval(() => { void refresh() }, 10000)
})
onUnmounted(() => { if (refreshTimer !== undefined) window.clearInterval(refreshTimer) })
watch(() => props.token, () => { void refresh() })
defineExpose({ refresh })
</script>

<template>
  <div class="proxy-monitor-page">
    <section class="panel proxy-monitor-hero">
      <div>
        <p class="eyebrow">REGISTRY PROXY</p>
        <h2>{{ t.proxyMonitorTitle }}</h2>
        <p>{{ t.proxyMonitorDescription }}</p>
      </div>
      <div class="proxy-monitor-actions">
        <span class="proxy-monitor-auto-refresh">{{ t.proxyMonitorAutoRefresh }}</span>
        <button class="refresh" type="button" :disabled="loading" @click="refresh">{{ loading ? t.loading : t.proxyMonitorRefresh }}</button>
      </div>
    </section>

    <section class="panel proxy-monitor-status-panel" aria-live="polite">
      <div class="proxy-monitor-status-heading">
        <div>
          <p class="eyebrow">{{ t.proxyRuntimeStatus }}</p>
          <h3>{{ statusLabel }}</h3>
        </div>
        <span class="proxy-status-chip" :class="statusKind"><i></i>{{ statusLabel }}</span>
      </div>
      <div class="proxy-monitor-status-grid">
        <div><span>{{ t.proxyMonitorPort }}</span><strong>{{ proxyData?.status.actual_port || '—' }}</strong></div>
        <div><span>{{ t.proxyCategory }}</span><strong>{{ proxyData?.status.category_id || proxyData?.config.category_id || '—' }}</strong></div>
        <div><span>{{ t.proxyCandidates }}</span><strong>{{ proxyData?.status.candidate_count ?? '—' }}</strong></div>
        <div><span>{{ t.proxyLastSeen }}</span><strong>{{ formatDateTime(proxyData?.status.last_seen_at) || '—' }}</strong></div>
      </div>
    </section>

    <section v-if="metrics" class="proxy-monitor-metrics" aria-live="polite">
      <div class="metric-card"><span>{{ t.proxyMetricRequests }}</span><strong>{{ metrics.requests.toLocaleString() }}</strong><small>{{ t.proxyMetricSuccessRate }} {{ successRate }}</small></div>
      <div class="metric-card metric-card-success"><span>{{ t.proxyMetricSuccesses }}</span><strong>{{ metrics.successes.toLocaleString() }}</strong><small>{{ t.proxyMetricResponses }} {{ responseTotal.toLocaleString() }}</small></div>
      <div class="metric-card metric-card-warning"><span>{{ t.proxyMetricFailures }}</span><strong>{{ metrics.upstream_failures.toLocaleString() }}</strong><small>{{ t.proxyMetricRetries }} {{ metrics.retries.toLocaleString() }}</small></div>
      <div class="metric-card"><span>{{ t.proxyMetricActive }}</span><strong>{{ metrics.active_requests.toLocaleString() }}</strong><small>{{ t.proxyMetricRedirects }} {{ metrics.redirects.toLocaleString() }}</small></div>
      <div class="metric-card"><span>{{ t.proxyMetricBytes }}</span><strong>{{ formatBytes(metrics.bytes_forwarded) }}</strong><small>{{ t.proxyMetricForwarded }}</small></div>
      <div class="metric-card"><span>{{ t.proxyMetricAverageDuration }}</span><strong>{{ formatDuration(metrics.average_duration_seconds) }}</strong><small>{{ t.proxyMetricUpdatedAt }} {{ formatDateTime(metrics.collected_at) || '—' }}</small></div>
    </section>

    <section v-if="metrics" class="panel proxy-monitor-breakdown">
      <div class="section-heading"><div><p class="eyebrow">HTTP</p><h2>{{ t.proxyMetricStatusClasses }}</h2></div><span class="selection-count">{{ t.proxyMonitorRequestIdHint }}</span></div>
      <div class="proxy-status-class-grid">
        <div><span>1xx</span><strong>{{ metrics.responses_1xx.toLocaleString() }}</strong></div>
        <div class="is-success"><span>2xx</span><strong>{{ metrics.responses_2xx.toLocaleString() }}</strong></div>
        <div><span>3xx</span><strong>{{ metrics.responses_3xx.toLocaleString() }}</strong></div>
        <div class="is-warning"><span>4xx</span><strong>{{ metrics.responses_4xx.toLocaleString() }}</strong></div>
        <div class="is-error"><span>5xx</span><strong>{{ metrics.responses_5xx.toLocaleString() }}</strong></div>
      </div>
    </section>

    <section v-else class="panel proxy-monitor-empty">{{ t.proxyMonitorNoData }}</section>
  </div>
</template>
