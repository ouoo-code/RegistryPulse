<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { useI18n, statusLabel } from '../../i18n'
import type { Category, Source } from '../../api'
import { formatDateTime } from '../../time'
import SortIndicator from '../SortIndicator.vue'
const props = defineProps<{ sources: Source[]; total: number; categories: Category[]; loading: boolean; query: string; tagQuery: string; status: string; sort: string; ascending: boolean; selectedIds: string[]; canGenerateConfig: boolean }>()
const emit = defineEmits<{ refresh: []; 'update:query': [value: string]; 'update:tagQuery': [value: string]; 'update:status': [value: string]; 'update:sort': [value: string]; 'update:ascending': [value: boolean]; toggle: [id: string]; 'open-generator': [] }>()
const { t } = useI18n()
const columnWidths = ref([8, 78, 190, 280, 125, 150, 150, 170])
let activeResize: { index: number; startX: number; startWidth: number } | null = null
const suppressSort = ref(false)
const columnTemplate = () => columnWidths.value.map(width => `${width}px`).join(' ')
function sortBy(column: string) {
  if (suppressSort.value) { suppressSort.value = false; return }
  if (props.sort === column) emit('update:ascending', !props.ascending)
  else { emit('update:sort', column); emit('update:ascending', false) }
}
function sortActive(column: string) { return props.sort === column }
function resizeMove(event: PointerEvent) {
  if (!activeResize) return
  columnWidths.value[activeResize.index] = Math.max(72, activeResize.startWidth + event.clientX - activeResize.startX)
}
function resizeEnd() {
  if (!activeResize) return
  activeResize = null
  suppressSort.value = true
  document.body.classList.remove('table-column-resizing')
  window.removeEventListener('pointermove', resizeMove)
  window.removeEventListener('pointerup', resizeEnd)
}
function resizeStart(event: PointerEvent, index: number) {
  event.preventDefault()
  event.stopPropagation()
  const visualIndex = [0, 4, 2, 3, 5, 6, 7, 1][index] ?? index
  activeResize = { index: visualIndex, startX: event.clientX, startWidth: columnWidths.value[visualIndex] }
  document.body.classList.add('table-column-resizing')
  window.addEventListener('pointermove', resizeMove)
  window.addEventListener('pointerup', resizeEnd)
}
onBeforeUnmount(resizeEnd)
function shortName(name: string) { const aliases: Record<string, string> = { 'GitHub Container Registry': 'GitHub', 'Microsoft Container Registry': 'Microsoft', 'Google Container Registry': 'Google', 'Elastic Container Registry': 'Elastic', 'NVIDIA Container Registry': 'NVIDIA', 'Kubernetes Registry': 'Kubernetes' }; return aliases[name] || (name.length > 24 ? `${name.slice(0, 23)}…` : name) }
function statusClass(value: string) { return value === 'online' ? 'online' : value === 'degraded' ? 'degraded' : 'offline' }
function categoryLabel(id: string) { const category = props.categories.find(item => item.id === id); return category?.slug || category?.name || '—' }
</script>

<template>
  <section class="registry-panel">
    <div class="registry-heading"><div><span class="panel-kicker">LIVE DIRECTORY</span><h2>{{ t.sources }} <small class="registry-summary">{{ sources.length }} {{ t.of }} {{ total }} {{ t.monitoredSites }} · 已选择 {{ selectedIds.length }}</small></h2></div><div class="registry-heading-actions"><button class="config-action" :disabled="!canGenerateConfig" :title="selectedIds.length && !canGenerateConfig ? t.sameCategoryHint : ''" @click="emit('open-generator')">生成配置 <b v-if="selectedIds.length">{{ selectedIds.length }}</b></button><button class="refresh-button" :disabled="loading" @click="emit('refresh')"><span :class="{ spinning: loading }">↻</span>{{ t.refresh }}</button></div></div>
    <div class="filter-bar"><label class="search-box"><span>⌕</span><input :value="query" :placeholder="t.search" :aria-label="t.search" @input="emit('update:query', ($event.target as HTMLInputElement).value)"></label><label class="filter-box"><span>{{ t.status }}</span><select :value="status" @change="emit('update:status', ($event.target as HTMLSelectElement).value)"><option value="">{{ t.all }}</option><option value="online">{{ t.online }}</option><option value="degraded">{{ t.degraded }}</option><option value="offline">{{ t.offline }}</option><option value="maintenance">{{ t.maintenance }}</option><option value="unknown">{{ t.unknown }}</option></select></label><label class="filter-box sort-box"><span>{{ t.sort }}</span><select :value="sort" @change="emit('update:sort', ($event.target as HTMLSelectElement).value)"><option value="status">{{ t.sortStatus }}</option><option value="latency">{{ t.sortLatency }}</option><option value="name">{{ t.sortName }}</option></select></label><button class="sort-direction" @click="emit('update:ascending', !ascending)" :aria-label="ascending ? t.desc : t.asc">{{ ascending ? '↑' : '↓' }}</button></div>
    <label class="tag-filter"><span>#</span><input :value="tagQuery" :placeholder="t.filterTags" @input="emit('update:tagQuery', ($event.target as HTMLInputElement).value)"></label>
    <div class="registry-table-head" :style="{ '--home-columns': columnTemplate() }"><span class="select-column">选</span><span class="sortable" :class="{ active: sortActive('category') }" @click="sortBy('category')">{{ t.category }} <SortIndicator :active="sortActive('category')" :ascending="ascending" /><i class="registry-column-resizer" @pointerdown="resizeStart($event, 1)"></i></span><span class="sortable" :class="{ active: sortActive('name') }" @click="sortBy('name')">{{ t.registry }} <SortIndicator :active="sortActive('name')" :ascending="ascending" /><i class="registry-column-resizer" @pointerdown="resizeStart($event, 2)"></i></span><span class="sortable" :class="{ active: sortActive('url') }" @click="sortBy('url')">{{ t.url }} <SortIndicator :active="sortActive('url')" :ascending="ascending" /><i class="registry-column-resizer" @pointerdown="resizeStart($event, 3)"></i></span><span class="sortable" :class="{ active: sortActive('tags') }" @click="sortBy('tags')">{{ t.tags }} <SortIndicator :active="sortActive('tags')" :ascending="ascending" /><i class="registry-column-resizer" @pointerdown="resizeStart($event, 4)"></i></span><span class="sortable" :class="{ active: sortActive('latency') }" @click="sortBy('latency')">{{ t.response }} <SortIndicator :active="sortActive('latency')" :ascending="ascending" /><i class="registry-column-resizer" @pointerdown="resizeStart($event, 5)"></i></span><span class="sortable" :class="{ active: sortActive('checked') }" @click="sortBy('checked')">{{ t.monitorTime }} <SortIndicator :active="sortActive('checked')" :ascending="ascending" /><i class="registry-column-resizer" @pointerdown="resizeStart($event, 6)"></i></span><span class="sortable" :class="{ active: sortActive('status') }" @click="sortBy('status')">{{ t.status }} <SortIndicator :active="sortActive('status')" :ascending="ascending" /><i class="registry-column-resizer" @pointerdown="resizeStart($event, 7)"></i></span></div>
    <div v-if="loading" class="empty-state">{{ t.loading }}</div>
    <div v-else-if="!sources.length" class="empty-state">{{ t.noResults }}</div>
    <article v-for="source in sources" v-else :key="source.id" class="registry-row" :style="{ '--home-columns': columnTemplate() }"><label class="source-select"><input type="checkbox" :checked="props.selectedIds.includes(source.id)" :aria-label="`选择 ${shortName(source.name)}`" @change="emit('toggle', source.id)"><span></span></label><div class="registry-category">{{ categoryLabel(source.category_id) }}</div><div class="registry-identity"><RouterLink :to="`/source/${source.id}`" :title="source.name">{{ shortName(source.name) }}</RouterLink><span v-if="source.is_official" class="official-badge" :title="t.official" :aria-label="t.official">🏛️</span><span v-if="source.is_recommended" class="recommended-badge" :title="t.recommended" :aria-label="t.recommended">⭐</span></div><div class="registry-url" :title="source.base_url">{{ source.base_url }}</div><div class="registry-tags"><span v-if="source.region">{{ source.region }}</span><span v-for="tag in source.tags" :key="tag">{{ tag }}</span><span v-if="!source.region && !source.tags?.length" class="muted-tag">—</span></div><div class="latency"><strong>{{ source.response_ms ? `${source.response_ms} ms` : t.unknown }}</strong></div><div class="checked-at" :title="source.last_checked || ''">{{ formatDateTime(source.last_checked) || t.unknown }}</div><div class="row-status" :class="statusClass(source.status)"><span></span>{{ statusLabel(source.status, t) }}</div></article>
  </section>
</template>
