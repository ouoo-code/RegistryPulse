<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { api, type History, type Source, type SourceAggregates } from '../api'
import { buildFaultTimeline, historyTime, trendPoints } from '../history-utils'
import { statusLabel, useI18n } from '../i18n'
import { formatDateTime } from '../time'

const { t, locale } = useI18n()
const route = useRoute()
const source = ref<Source>()
const history = ref<History[]>([])
const aggregates = ref<SourceAggregates>({ hourly: [], daily: [] })
const loading = ref(true)
const failed = ref(false)
const orderedHistory = computed(() => [...history.value].sort((a, b) => historyTime(b).localeCompare(historyTime(a))))
const trend = computed(() => trendPoints(history.value))
const faults = computed(() => buildFaultTimeline(history.value))
const availability = computed(() => {
  const values = aggregates.value.daily.length ? aggregates.value.daily : aggregates.value.hourly
  const samples = values.reduce((total, item) => total + item.samples, 0)
  const online = values.reduce((total, item) => total + item.online_samples, 0)
  return samples ? Math.round((online / samples) * 10000) / 100 : null
})
const copy = computed(() => locale.value === 'zh' ? {
  detail: '镜像源详情', trend: '响应趋势', latest: '最新检测', samples: '样本', max: '峰值',
  faultTimeline: '故障时间线', noFaults: '暂无故障记录', noTrend: '暂无足够的历史数据生成趋势',
  faultStart: '开始', faultEnd: '结束', occurrences: '检测次数', recovered: '检测到恢复前的最后一次故障状态'
} : {
  detail: 'Registry detail', trend: 'Response trend', latest: 'Latest check', samples: 'samples', max: 'peak',
  faultTimeline: 'Incident timeline', noFaults: 'No incidents recorded', noTrend: 'Not enough history for a trend',
  faultStart: 'Started', faultEnd: 'Last seen', occurrences: 'checks', recovered: 'Last recorded fault state'
})

function formatTime(value: string) {
  if (!value || value === '—') return '—'
  return formatDateTime(value)
}

onMounted(async () => {
  const id = encodeURIComponent(String(route.params.id))
  try {
    ;[source.value, history.value, aggregates.value] = await Promise.all([
      api<Source>('/public/sources/' + id),
      api<History[]>('/public/sources/' + id + '/history'),
      api<SourceAggregates>('/public/sources/' + id + '/aggregates')
    ])
  } catch {
    failed.value = true
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <main class="page source-page">
    <div v-if="loading" class="empty detail-empty">{{ t.loading }}</div>
    <div v-else-if="failed" class="empty detail-empty">{{ t.apiError }}</div>
    <section v-else-if="source" class="detail">
      <div class="detail-heading"><div><p class="eyebrow">{{ copy.detail.toUpperCase() }}</p><h1>{{ source.name }}</h1><p class="lead">{{ source.base_url }}</p><span class="state badge" :class="source.status">{{ statusLabel(source.status, t) }}</span></div><RouterLink class="detail-back-button" to="/">{{ locale === 'zh' ? '返回' : 'Back' }}</RouterLink></div>
      <div class="detail-grid">
        <div><small>{{ t.region }}</small><strong>{{ source.region || t.unknown }}</strong></div>
        <div><small>{{ t.response }}</small><strong>{{ source.response_ms ? source.response_ms + ' ms' : t.unknown }}</strong></div>
        <div><small>{{ t.checked }}</small><strong>{{ formatDateTime(source.last_checked) || t.unknown }}</strong></div>
        <div><small>{{ t.error }}</small><strong>{{ source.error || '—' }}</strong></div>
        <div><small>24h / 30d availability</small><strong>{{ availability === null ? t.unknown : availability + '%' }}</strong></div>
        <div><small>Provider</small><strong>{{ source.provider || t.unknown }}</strong></div>
        <div><small>Test image</small><strong>{{ source.test_repository ? source.test_repository + ':' + (source.test_tag || 'latest') : t.unknown }}</strong></div>
        <div><small>Probe timeout</small><strong>{{ source.request_timeout_seconds ? source.request_timeout_seconds + ' s' : t.unknown }}</strong></div>
      </div>

      <section class="detail-panels">
        <section class="history panel trend-panel">
          <div class="section-title"><div><h2>{{ copy.trend }}</h2><p class="panel-subtitle">{{ copy.latest }} · {{ orderedHistory[0] ? formatTime(historyTime(orderedHistory[0])) : '—' }}</p></div><span>{{ orderedHistory.length }} {{ copy.samples }}</span></div>
          <div v-if="orderedHistory.length < 2" class="empty inner-empty">{{ copy.noTrend }}</div>
          <div v-else class="trend-wrap" role="img" :aria-label="copy.trend">
            <svg viewBox="0 0 640 180" preserveAspectRatio="none" aria-hidden="true">
              <line v-for="y in [18, 72, 126]" :key="y" x1="0" :y1="y" x2="640" :y2="y" class="trend-grid" />
              <polyline :points="trend.polyline" class="trend-line" />
              <circle v-for="point in trend.points" :key="point.x + '-' + historyTime(point.item)" :cx="point.x" :cy="point.y" r="4" :class="'trend-point ' + point.item.status"><title>{{ point.item.response_ms }} ms</title></circle>
            </svg>
            <div class="trend-scale"><span>0 ms</span><span>{{ trend.max }} ms {{ copy.max }}</span></div>
          </div>
        </section>

        <section class="history panel incident-panel">
          <div class="section-title"><div><h2>{{ copy.faultTimeline }}</h2><p class="panel-subtitle">{{ copy.recovered }}</p></div><span>{{ faults.length }}</span></div>
          <div v-if="!faults.length" class="empty inner-empty">{{ copy.noFaults }}</div>
          <ol v-else class="incident-timeline">
            <li v-for="event in faults" :key="event.startedAt + '-' + event.status" :class="event.status">
              <div class="timeline-dot" />
              <div class="incident-content"><div class="incident-head"><strong>{{ statusLabel(event.status, t) }}</strong><span>{{ event.count }} {{ copy.occurrences }}</span></div><p>{{ event.message || '—' }}</p><small>{{ copy.faultStart }} {{ formatTime(event.startedAt) }} · {{ copy.faultEnd }} {{ formatTime(event.endedAt) }}</small></div>
            </li>
          </ol>
        </section>
      </section>

      <section class="history panel">
        <div class="section-title"><h2>{{ t.history }}</h2><span>{{ orderedHistory.length }}</span></div>
        <div v-if="!orderedHistory.length" class="empty inner-empty">{{ t.emptyHistory }}</div>
        <div v-else class="history-table"><table><thead><tr><th>{{ t.time }}</th><th>{{ t.status }}</th><th>{{ t.latency }}</th><th>Blob speed</th><th>Error stage</th><th>{{ t.error }}</th></tr></thead><tbody><tr v-for="(item, i) in orderedHistory" :key="historyTime(item) + '-' + i"><td>{{ formatTime(historyTime(item)) }}</td><td><span class="state" :class="item.status">{{ statusLabel(item.status, t) }}</span></td><td>{{ item.response_ms ? item.response_ms + ' ms' : '—' }}</td><td>{{ item.blob_speed_bps ? Math.round(item.blob_speed_bps / 1024) + ' KiB/s' : '—' }}</td><td>{{ item.error_stage || '—' }}</td><td>{{ item.error || '—' }}</td></tr></tbody></table></div>
      </section>
    </section>
    <div v-else class="empty detail-empty">{{ t.noResults }}</div>
  </main>
</template>
