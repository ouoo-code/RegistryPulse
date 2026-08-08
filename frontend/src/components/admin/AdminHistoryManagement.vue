<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { adminApi } from '../../admin-api'
import { type History, type Source } from '../../api'
import { useI18n, statusLabel } from '../../i18n'
import { formatDateTime } from '../../time'
import BaseTable from '../BaseTable.vue'
import Pagination from '../Pagination.vue'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ error: [message: string] }>()
const { t, locale } = useI18n()
const results = ref<History[]>([])
const sources = ref<Source[]>([])
const sourceFilter = ref('')
const statusFilter = ref('')
const query = ref('')
const dateFrom = ref('')
const dateTo = ref('')
const page = ref(1)
const pageSize = ref(25)
const loading = ref(false)
const sourceNames = computed(() => new Map(sources.value.map(source => [source.id, source.name])))
const filtered = computed(() => results.value.filter(result => {
  const sourceName = sourceNames.value.get(result.source_id) || result.source_id
  return (!sourceFilter.value || result.source_id === sourceFilter.value)
    && (!statusFilter.value || result.status === statusFilter.value)
    && (!dateFrom.value || String(result.checked_at || '').slice(0, 10) >= dateFrom.value)
    && (!dateTo.value || String(result.checked_at || '').slice(0, 10) <= dateTo.value)
    && (!query.value || `${sourceName} ${result.error || ''} ${result.error_stage || ''}`.toLowerCase().includes(query.value.toLowerCase()))
}))
const pageCount = computed(() => Math.max(1, Math.ceil(filtered.value.length / pageSize.value)))
const paged = computed(() => filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
function resetPage() { page.value = 1 }
function changePageSize(size: number) { pageSize.value = size; page.value = 1 }
async function refresh() {
  loading.value = true
  try { [sources.value, results.value] = await Promise.all([adminApi.sources(props.token), adminApi.results(props.token)]); page.value = Math.min(page.value, pageCount.value) }
  catch (error) { emit('error', error instanceof Error ? error.message : t.value.apiError) }
  finally { loading.value = false }
}
onMounted(refresh)
defineExpose({ refresh })
</script>

<template>
  <section class="panel history-filters">
    <div class="history-filter-row"><input v-model="query" :placeholder="t.search" @input="resetPage"><select v-model="sourceFilter" :aria-label="t.registry" @change="resetPage"><option value="">{{ t.all }}</option><option v-for="source in sources" :key="source.id" :value="source.id">{{ source.name }}</option></select><select v-model="statusFilter" :aria-label="t.status" @change="resetPage"><option value="">{{ t.status }}</option><option value="online">{{ t.online }}</option><option value="degraded">{{ t.degraded }}</option><option value="offline">{{ t.offline }}</option><option value="maintenance">{{ t.maintenance }}</option><option value="unknown">{{ t.unknown }}</option></select><input v-model="dateFrom" class="history-date-filter" type="date" :aria-label="t.startedAt" @change="resetPage"><input v-model="dateTo" class="history-date-filter" type="date" :aria-label="t.endedAt" @change="resetPage"><button class="icon-button history-refresh-button" type="button" @click="refresh">{{ t.refresh }}</button></div>
  </section>
  <BaseTable class="history-admin-table" :empty="loading ? t.loading : (!paged.length ? t.noResults : '')"><template #head><thead><tr><th class="history-time-column">{{ t.checked }}</th><th class="history-source-column">{{ t.registry }}</th><th class="history-status-column">{{ t.status }}</th><th class="history-response-column">{{ t.response }}</th><th class="history-stage-column">{{ locale === 'zh' ? '错误阶段' : 'Error stage' }}</th><th class="history-error-column">{{ t.error }}</th></tr></thead></template><template #body><tbody><tr v-for="result in paged" :key="`${result.source_id}-${result.checked_at}-${result.status}`"><td class="history-time-column">{{ formatDateTime(result.checked_at) || t.unknown }}</td><td class="history-source-column">{{ sourceNames.get(result.source_id) || result.source_id }}</td><td class="history-status-column"><span class="state" :class="result.status">{{ statusLabel(result.status, t) }}</span></td><td class="history-response-column">{{ result.response_ms ? `${result.response_ms} ms` : '—' }}</td><td class="history-stage-column">{{ result.error_stage || '—' }}</td><td class="history-error-cell history-error-column">{{ result.error || '—' }}</td></tr></tbody></template></BaseTable>
  <Pagination :page="page" :page-count="pageCount" :total="filtered.length" :page-size="pageSize" :page-size-options="[10, 25, 50, 100]" @change="page = $event" @update:page-size="changePageSize" />
</template>
