<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api, type Category, type Source } from '../api'
import { useI18n } from '../i18n'
import HomeCategoryPanel from '../components/home/HomeCategoryPanel.vue'
import HomeMetrics from '../components/home/HomeMetrics.vue'
import HomeRegistryList from '../components/home/HomeRegistryList.vue'
import HomeConfigDialog from '../components/home/HomeConfigDialog.vue'
import { formatDateTime } from '../time'
import './HomeView.css'
import './HomeViewWide.css'

const { t } = useI18n()
const sources = ref<Source[]>([])
const categories = ref<Category[]>([])
const counts = ref<Record<string, number>>({})
const query = ref('')
const tagQuery = ref('')
const status = ref('')
const selectedCategory = ref('')
const sort = ref('status')
const ascending = ref(false)
const loading = ref(true)
const error = ref('')
const lastUpdated = ref('')
const selectedIds = ref<string[]>([])
const configOpen = ref(false)
let refreshTimer: ReturnType<typeof setInterval> | undefined

const filtered = computed(() => [...sources.value
  .filter(source => {
    const haystack = `${source.name} ${source.base_url} ${source.provider || ''} ${source.region || ''} ${(source.tags || []).join(' ')}`.toLowerCase()
    return (!query.value || haystack.includes(query.value.toLowerCase()))
      && (!tagQuery.value || (source.tags || []).some(tag => tag.toLowerCase().includes(tagQuery.value.toLowerCase())))
      && (!status.value || source.status === status.value)
      && (!selectedCategory.value || source.category_id === selectedCategory.value)
  })
  .sort((a, b) => {
    const order: Record<string, number> = { online: 0, degraded: 1, offline: 2, maintenance: 3 }
    const av = sort.value === 'name' ? a.name : sort.value === 'category' ? a.category_id : sort.value === 'url' ? a.base_url : sort.value === 'tags' ? (a.tags || []).join(',') : sort.value === 'latency' ? a.response_ms || 999999 : sort.value === 'checked' ? a.last_checked || '' : order[a.status] ?? 4
    const bv = sort.value === 'name' ? b.name : sort.value === 'category' ? b.category_id : sort.value === 'url' ? b.base_url : sort.value === 'tags' ? (b.tags || []).join(',') : sort.value === 'latency' ? b.response_ms || 999999 : sort.value === 'checked' ? b.last_checked || '' : order[b.status] ?? 4
    return (av < bv ? -1 : av > bv ? 1 : 0) * (ascending.value ? 1 : -1)
  })])

const availability = computed(() => {
  const total = counts.value.total || sources.value.length
  return total ? Math.round(((counts.value.online || 0) / total) * 100) : 0
})
const selectedSources = computed(() => sources.value.filter(source => selectedIds.value.includes(source.id)))
const selectedSourceCategory = computed(() => {
  const categoryId = selectedSources.value[0]?.category_id
  return categories.value.find(category => category.id === categoryId)
})
const canGenerateConfig = computed(() => {
  if (!selectedSources.value.length || !selectedSources.value[0].category_id) return false
  return selectedSources.value.every(source => source.category_id === selectedSources.value[0].category_id)
})
function openConfig() { if (canGenerateConfig.value) configOpen.value = true }

function toggleSource(id: string) {
  selectedIds.value = selectedIds.value.includes(id) ? selectedIds.value.filter(value => value !== id) : [...selectedIds.value, id]
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [summary, cats, list] = await Promise.all([
      api<{ counts: Record<string, number>; last_updated?: string }>('/public/summary'),
      api<Category[]>('/public/categories'),
      api<Source[]>('/public/sources'),
    ])
    counts.value = summary.counts
    categories.value = cats
    sources.value = list
    lastUpdated.value = formatDateTime(summary.last_updated) || formatDateTime(new Date().toISOString())
  } catch {
    error.value = t.value.apiError
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
  refreshTimer = setInterval(load, 60_000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<template>
  <main class="home-page">
    <section class="home-hero dashboard-hero">
      <div class="hero-copy">
        <div class="status-pill"><span class="live-dot"></span>{{ t.liveMonitoring }}</div>
        <h1>{{ t.heroTitle }} <em>{{ t.heroAccent }}</em></h1>
        <p>{{ t.heroDescription }}</p>
        <div class="updated-at"><span class="pulse-line"></span>{{ t.lastUpdated }}：{{ lastUpdated || '—' }} · {{ t.autoRefresh }}</div>
      </div>
      <div class="health-card compact-health" aria-label="Availability summary">
        <span class="health-label">{{ t.availability }}</span><strong>{{ availability }}<small>%</small></strong>
        <div class="health-track"><i :style="{ width: `${availability}%` }"></i></div>
        <div class="health-caption"><span class="live-dot"></span>{{ t.liveData }}<small>{{ counts.total || 0 }} {{ t.monitoredSites }}</small></div>
      </div>
    </section>

    <div v-if="error" class="home-alert">{{ error }}</div>

    <HomeMetrics :counts="counts" />

    <section id="registry-list" class="monitor-layout">
      <HomeCategoryPanel :categories="categories" :sources="sources" :selected-category="selectedCategory" @select="selectedCategory = $event" />
      <HomeRegistryList :sources="filtered" :total="sources.length" :categories="categories" :loading="loading" :query="query" :tag-query="tagQuery" :status="status" :sort="sort" :ascending="ascending" :selected-ids="selectedIds" :can-generate-config="canGenerateConfig" @refresh="load" @toggle="toggleSource" @open-generator="openConfig" @update:query="query = $event" @update:tag-query="tagQuery = $event" @update:status="status = $event" @update:sort="sort = $event" @update:ascending="ascending = $event" />
    </section>
    <HomeConfigDialog :open="configOpen" :sources="selectedSources" :category="selectedSourceCategory" @close="configOpen = false" @clear="selectedIds = []" />
  </main>
</template>
