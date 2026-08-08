<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { adminApi, adminRequest, exportSources, importSources } from '../../admin-api'
import { type Category, type Source, type SourceInput, type TestImage } from '../../api'
import { statusLabel, useI18n } from '../../i18n'
import BaseDialog from '../BaseDialog.vue'
import BaseTable from '../BaseTable.vue'
import FormField from '../FormField.vue'
import SortIndicator from '../SortIndicator.vue'
import Pagination from '../Pagination.vue'
import { formatDateTime } from '../../time'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ notice: [message: string]; error: [message: string] }>()
const { t, locale } = useI18n()
const sources = ref<Source[]>([])
const categories = ref<Category[]>([])
const testImages = ref<TestImage[]>([])
const selected = ref<string[]>([])
const editing = ref<Source | null>(null)
const editorOpen = ref(false)
const tagText = ref('')
const page = ref(1)
const pageSize = ref(25)
const loading = ref(false)
const categoryFilter = ref('')
const query = ref('')
const statusFilter = ref('')
const officialFilter = ref('')
const recommendedFilter = ref('')
const bulkEditOpen = ref(false)
const bulkEnabled = ref('')
const bulkMaintenance = ref('')
const bulkCategory = ref('')
const bulkOfficial = ref('')
const bulkRecommended = ref('')
const deletingSourceId = ref<string | null>(null)
const pendingDeleteIds = ref(new Set<string>())
const sourceSort = ref<'enabled' | 'name' | 'url' | 'status'>('enabled')
const sourceAscending = ref(true)
const bytesPerMiB = 1024 * 1024
function bytesToMiB(bytes: number) {
  const value = bytes / bytesPerMiB
  return Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/0+$/, '').replace(/\.$/, '')
}

const emptyForm = (): SourceInput => ({ name: '', base_url: '', category_id: '', provider: '', region: '', tags: [], enabled: true, maintenance: false, probe_mode: 'registry', description: '', country: '', operator: '', test_image_id: '', request_timeout_seconds: 10 })
const form = ref<SourceInput>(emptyForm())
const pageCount = computed(() => Math.max(1, Math.ceil(filteredSources.value.length / pageSize.value)))
const filteredSources = computed(() => sources.value.filter(source => {
  const text = `${source.name} ${source.base_url} ${source.provider || ''}`.toLowerCase()
  return !pendingDeleteIds.value.has(source.id)
    && (!categoryFilter.value || source.category_id === categoryFilter.value)
    && (!query.value || text.includes(query.value.toLowerCase()))
    && (!officialFilter.value || String(source.is_official === true) === officialFilter.value)
    && (!recommendedFilter.value || String(source.is_recommended === true) === recommendedFilter.value)
    && (!statusFilter.value || (statusFilter.value === 'enabled' ? source.enabled !== false : statusFilter.value === 'disabled' ? source.enabled === false : source.status === statusFilter.value))
}))
const sortedSources = computed(() => [...filteredSources.value].sort((left, right) => {
  let result = 0
  if (sourceSort.value === 'enabled') result = Number(right.enabled !== false) - Number(left.enabled !== false)
  if (sourceSort.value === 'name') result = left.name.localeCompare(right.name)
  if (sourceSort.value === 'url') result = left.base_url.localeCompare(right.base_url)
  if (sourceSort.value === 'status') result = left.status.localeCompare(right.status)
  if (result === 0) result = (left.sort_order || 0) - (right.sort_order || 0) || left.name.localeCompare(right.name)
  return sourceAscending.value ? result : -result
}))
const pagedSources = computed(() => sortedSources.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
const allPageSelected = computed(() => pagedSources.value.length > 0 && pagedSources.value.every(source => selected.value.includes(source.id)))

function handleError(fallback: string) {
  return (error: unknown) => emit('error', error instanceof Error ? error.message : fallback)
}
function sortSources(key: 'enabled' | 'name' | 'url' | 'status') {
  if (sourceSort.value === key) sourceAscending.value = !sourceAscending.value
  else { sourceSort.value = key; sourceAscending.value = key !== 'enabled' }
}
function sortActive(key: 'enabled' | 'name' | 'url' | 'status') { return sourceSort.value === key }
function changePageSize(size: number) { pageSize.value = size; page.value = 1 }

async function refresh() {
  loading.value = true
  try {
    const [sourceList, categoryList, testImageList] = await Promise.all([adminApi.sources(props.token), adminApi.categories(props.token), adminApi.testImages(props.token)])
    sources.value = sourceList
    categories.value = categoryList
    testImages.value = testImageList
    page.value = Math.min(page.value, pageCount.value)
  } catch (error) { handleError(t.value.apiError)(error) } finally { loading.value = false }
}

function openCreate() {
  editing.value = null
  form.value = emptyForm()
  tagText.value = ''
  editorOpen.value = true
}

function openEdit(source: Source) {
  editing.value = source
  form.value = { name: source.name, base_url: source.base_url, category_id: source.category_id, provider: source.provider, region: source.region, tags: source.tags, enabled: source.enabled !== false, maintenance: source.maintenance, probe_mode: source.probe_mode || 'registry', description: source.description, country: source.country, operator: source.operator, is_official: source.is_official, is_cloudflare: source.is_cloudflare, is_recommended: source.is_recommended, priority: source.priority, sort_order: source.sort_order, test_image_id: source.test_image_id || '', request_timeout_seconds: source.request_timeout_seconds }
  tagText.value = source.tags.join(', ')
  editorOpen.value = true
}

function closeEditor() { editorOpen.value = false; editing.value = null }

async function save() {
  try {
    const path = editing.value ? `/admin/sources/${editing.value.id}` : '/admin/sources'
    await adminRequest<Source>(props.token, path, { method: editing.value ? 'PUT' : 'POST', body: JSON.stringify({ ...form.value, tags: tagText.value.split(',').map(tag => tag.trim()).filter(Boolean) }) })
    emit('notice', t.value.saveSuccess)
    closeEditor()
    await refresh()
  } catch (error) { handleError(t.value.saveError)(error) }
}

async function remove(source: Source) {
  if (!confirm(t.value.confirmDelete)) return
  if (deletingSourceId.value) return
  deletingSourceId.value = source.id
  try {
    await adminRequest<{ queued: boolean }>(props.token, `/admin/sources/${source.id}`, { method: 'DELETE' })
    pendingDeleteIds.value = new Set([...pendingDeleteIds.value, source.id])
    selected.value = selected.value.filter(id => id !== source.id)
    emit('notice', t.value.deleteQueued)
    void confirmDeletion(source.id)
  } catch (error) {
    handleError(t.value.deleteError)(error)
  } finally {
    deletingSourceId.value = null
  }
}

async function confirmDeletion(sourceID: string) {
  for (let attempt = 0; attempt < 60; attempt++) {
    await new Promise(resolve => window.setTimeout(resolve, 1000))
    try {
      const latest = await adminApi.sources(props.token)
      sources.value = latest
      if (!latest.some(source => source.id === sourceID)) {
        const next = new Set(pendingDeleteIds.value)
        next.delete(sourceID)
        pendingDeleteIds.value = next
        return
      }
    } catch {
      // Keep the row hidden while the asynchronous deletion is in progress.
    }
  }
  const next = new Set(pendingDeleteIds.value)
  next.delete(sourceID)
  pendingDeleteIds.value = next
  await refresh()
}

async function probe(source: Source) {
  try { await adminRequest<void>(props.token, `/admin/sources/${source.id}/probe`, { method: 'POST' }); emit('notice', t.value.probeQueued) } catch (error) { handleError(t.value.probeError)(error) }
}

function toggleSelected(id: string) { selected.value = selected.value.includes(id) ? selected.value.filter(value => value !== id) : [...selected.value, id] }
function togglePage() { const pageIds = new Set(pagedSources.value.map(source => source.id)); selected.value = allPageSelected.value ? selected.value.filter(id => !pageIds.has(id)) : [...new Set([...selected.value, ...pageIds])] }

async function batch(action: 'enable' | 'disable') {
  if (!selected.value.length) return
  try { await adminRequest<void>(props.token, '/admin/sources/batch', { method: 'POST', body: JSON.stringify({ ids: selected.value, action }) }); selected.value = []; await refresh() } catch (error) { handleError(t.value.registryUpdateError)(error) }
}
function openBulkEdit() {
  bulkEnabled.value = ''
  bulkMaintenance.value = ''
  bulkCategory.value = ''
  bulkOfficial.value = ''
  bulkRecommended.value = ''
  bulkEditOpen.value = true
}
async function saveBulkEdit() {
  const body: Record<string, unknown> = { ids: selected.value, action: 'edit' }
  if (bulkEnabled.value) body.enabled = bulkEnabled.value === 'true'
  if (bulkMaintenance.value) body.maintenance = bulkMaintenance.value === 'true'
  if (bulkCategory.value) body.category_id = bulkCategory.value
  if (bulkOfficial.value) body.is_official = bulkOfficial.value === 'true'
  if (bulkRecommended.value) body.is_recommended = bulkRecommended.value === 'true'
  if (Object.keys(body).length === 2) return
  try { await adminRequest<void>(props.token, '/admin/sources/batch', { method: 'POST', body: JSON.stringify(body) }); bulkEditOpen.value = false; selected.value = []; emit('notice', t.value.saveSuccess); await refresh() } catch (error) { handleError(t.value.registryUpdateError)(error) }
}

async function toggleEnabled(source: Source) {
  try {
    await adminRequest<void>(props.token, '/admin/sources/batch', { method: 'POST', body: JSON.stringify({ ids: [source.id], action: source.enabled === false ? 'enable' : 'disable' }) })
    await refresh()
  } catch (error) { handleError(t.value.registryUpdateError)(error) }
}

async function download(format: 'json' | 'csv') {
  try { const blob = await exportSources(props.token, format); const link = document.createElement('a'); link.href = URL.createObjectURL(blob); link.download = `registry-sources.${format}`; link.click(); URL.revokeObjectURL(link.href) } catch (error) { handleError(t.value.exportError)(error) }
}

async function importFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  try { await importSources(props.token, await file.text(), file.name.toLowerCase().endsWith('.csv')); emit('notice', t.value.importSuccess); await refresh() } catch (error) { handleError(t.value.importError)(error) } finally { input.value = '' }
}

onMounted(refresh)
defineExpose({ refresh, openCreate })
</script>

<template>
  <section class="panel admin-actions">
    <div class="section-heading source-actions-heading"><div class="source-actions-title"><span class="eyebrow">{{ t.sourceActions }}</span><span class="selection-count">{{ selected.length }} {{ t.selected }}</span></div>
    <div class="admin-action-buttons">
      <input v-model="query" class="source-search-filter" :placeholder="locale === 'zh' ? '搜索' : 'Search'" :aria-label="t.search" @input="page = 1">
      <select v-model="categoryFilter" class="source-category-filter" :aria-label="t.category" @change="page = 1"><option value="">{{ t.all }}</option><option v-for="category in categories" :key="category.id" :value="category.id">{{ category.slug }} · {{ category.name }}</option></select>
      <select v-model="statusFilter" class="source-status-filter" :aria-label="t.status" @change="page = 1"><option value="">{{ t.status }}</option><option value="enabled">{{ t.enabled }}</option><option value="disabled">{{ t.disabled }}</option><option value="online">{{ t.online }}</option><option value="offline">{{ t.offline }}</option><option value="maintenance">{{ t.maintenance }}</option><option value="unknown">{{ t.unknown }}</option></select>
      <select v-model="officialFilter" class="source-flag-filter" :aria-label="t.official" @change="page = 1"><option value="">{{ t.official }}</option><option value="true">{{ locale === 'zh' ? '是' : 'Yes' }}</option><option value="false">{{ locale === 'zh' ? '否' : 'No' }}</option></select>
      <select v-model="recommendedFilter" class="source-flag-filter" :aria-label="t.recommended" @change="page = 1"><option value="">{{ t.recommended }}</option><option value="true">{{ locale === 'zh' ? '是' : 'Yes' }}</option><option value="false">{{ locale === 'zh' ? '否' : 'No' }}</option></select>
      <button class="icon-button" type="button" :disabled="!selected.length" @click="batch('enable')">{{ t.enableSelected }}</button>
      <button class="icon-button" type="button" :disabled="!selected.length" @click="batch('disable')">{{ t.disableSelected }}</button>
      <button class="icon-button" type="button" :disabled="!selected.length" @click="openBulkEdit">{{ t.edit }}</button>
      <span class="action-divider" aria-hidden="true"></span>
      <button class="icon-button" type="button" @click="download('json')">{{ t.exportJson }}</button>
      <button class="icon-button" type="button" @click="download('csv')">{{ t.exportCsv }}</button>
      <label class="icon-button file-button">{{ t.importData }}<input type="file" accept=".json,.csv,application/json,text/csv" hidden @change="importFile"></label>
      <button class="icon-button" type="button" @click="openCreate">{{ t.add }}</button>
    </div></div>
  </section>

  <BaseDialog :open="bulkEditOpen" :title="`${t.edit} (${selected.length})`" @close="bulkEditOpen = false">
    <form class="bulk-edit-form" @submit.prevent="saveBulkEdit">
      <FormField :label="t.enabled"><select v-model="bulkEnabled"><option value="">{{ locale === 'zh' ? '不修改' : 'No change' }}</option><option value="true">{{ t.enabled }}</option><option value="false">{{ t.disabled }}</option></select></FormField>
      <FormField :label="t.maintenance"><select v-model="bulkMaintenance"><option value="">{{ locale === 'zh' ? '不修改' : 'No change' }}</option><option value="true">{{ t.maintenance }}</option><option value="false">{{ locale === 'zh' ? '取消维护' : 'Clear maintenance' }}</option></select></FormField>
      <FormField :label="t.category"><select v-model="bulkCategory"><option value="">{{ locale === 'zh' ? '不修改' : 'No change' }}</option><option v-for="category in categories" :key="category.id" :value="category.id">{{ category.slug }} · {{ category.name }}</option></select></FormField>
      <FormField :label="t.official"><select v-model="bulkOfficial"><option value="">{{ locale === 'zh' ? '不修改' : 'No change' }}</option><option value="true">{{ locale === 'zh' ? '标记为官方' : 'Mark official' }}</option><option value="false">{{ locale === 'zh' ? '取消官方' : 'Clear official' }}</option></select></FormField>
      <FormField :label="t.recommended"><select v-model="bulkRecommended"><option value="">{{ locale === 'zh' ? '不修改' : 'No change' }}</option><option value="true">{{ locale === 'zh' ? '标记为推荐' : 'Mark recommended' }}</option><option value="false">{{ locale === 'zh' ? '取消推荐' : 'Clear recommended' }}</option></select></FormField>
      <div class="editor-form-actions"><button class="refresh" type="submit">{{ t.save }}</button><button class="icon-button" type="button" @click="bulkEditOpen = false">{{ t.cancel }}</button></div>
    </form>
  </BaseDialog>

  <BaseDialog :open="editorOpen" :title="`${editing ? t.edit : t.add} ${t.adminSources}`" @close="closeEditor">
    <form class="admin-editor-form" @submit.prevent="save">
      <FormField :label="t.name"><input v-model="form.name" required></FormField>
      <FormField :label="t.url"><input v-model="form.base_url" type="url" required></FormField>
      <FormField :label="t.category"><select v-model="form.category_id" required><option value="" disabled>{{ t.category }}</option><option v-for="category in categories" :key="category.id" :value="category.id">{{ category.slug }} · {{ category.name }}</option></select></FormField>
      <FormField :label="t.provider"><input v-model="form.provider"></FormField>
      <FormField :label="t.region"><input v-model="form.region"></FormField>
      <FormField :label="t.tags"><input v-model="tagText" placeholder="official, cn"></FormField>
      <FormField class="form-field-description" :label="t.description"><textarea v-model="form.description" rows="1"></textarea></FormField>
      <FormField class="form-field-test-image" :label="t.testImage"><select v-model="form.test_image_id"><option value="">{{ t.systemDefaultTestImage }}</option><option v-for="image in testImages.filter(item => item.enabled)" :key="image.id" :value="image.id">{{ image.reference }} · {{ bytesToMiB(image.max_bytes) }} M</option></select></FormField>
      <FormField class="form-field-probe-mode" :label="t.probeMode"><select v-model="form.probe_mode"><option value="registry">{{ t.registryProbe }}</option><option value="docker_pull">{{ t.dockerPullProbe }}</option></select></FormField>
      <FormField class="form-field-timeout" :label="t.timeout"><input v-model.number="form.request_timeout_seconds" type="number" min="1" max="300"></FormField>
      <div class="editor-checks"><label class="checkbox-field"><input v-model="form.enabled" type="checkbox"><span>{{ t.enabled }}</span></label><label class="checkbox-field"><input v-model="form.is_official" type="checkbox"><span>{{ t.official }}</span></label><label class="checkbox-field"><input v-model="form.is_recommended" type="checkbox"><span>{{ t.recommended }}</span></label><label class="checkbox-field"><input v-model="form.maintenance" type="checkbox"><span>{{ t.maintenance }}</span></label></div>
      <div class="editor-form-actions"><button class="refresh" type="submit">{{ t.save }}</button><button class="icon-button" type="button" @click="closeEditor">{{ t.cancel }}</button></div>
    </form>
  </BaseDialog>

  <BaseTable class="source-table" :empty="loading ? t.loading : (!filteredSources.length ? t.noResults : '')">
    <template #head><thead><tr><th class="select-column"><input type="checkbox" :checked="allPageSelected" :aria-label="t.selectAllPage" @change="togglePage"></th><th class="sortable-header" :class="{ active: sortActive('enabled') }" :aria-sort="sortActive('enabled') ? (sourceAscending ? 'ascending' : 'descending') : 'none'" @click="sortSources('enabled')">{{ t.status }} <SortIndicator :active="sortActive('enabled')" :ascending="sourceAscending" /></th><th class="sortable-header" :class="{ active: sortActive('name') }" :aria-sort="sortActive('name') ? (sourceAscending ? 'ascending' : 'descending') : 'none'" @click="sortSources('name')">{{ t.name }} <SortIndicator :active="sortActive('name')" :ascending="sourceAscending" /></th><th class="sortable-header" :class="{ active: sortActive('url') }" :aria-sort="sortActive('url') ? (sourceAscending ? 'ascending' : 'descending') : 'none'" @click="sortSources('url')">{{ t.url }} <SortIndicator :active="sortActive('url')" :ascending="sourceAscending" /></th><th>{{ t.region }}</th><th>{{ t.probeMode }}</th><th>{{ t.createdAt }}</th><th class="sortable-header" :class="{ active: sortActive('status') }" :aria-sort="sortActive('status') ? (sourceAscending ? 'ascending' : 'descending') : 'none'" @click="sortSources('status')">{{ t.monitoringStatus }} <SortIndicator :active="sortActive('status')" :ascending="sourceAscending" /></th><th>{{ t.actions }}</th></tr></thead></template>
    <template #body><tbody><tr v-for="source in pagedSources" :key="source.id"><td class="select-column"><input type="checkbox" :checked="selected.includes(source.id)" @change="toggleSelected(source.id)"></td><td><span class="enabled-state" :class="{ disabled: source.enabled === false }"><i></i>{{ source.enabled === false ? t.disabled : t.enabled }}</span></td><td class="source-name-cell"><span class="admin-source-identity"><span>{{ source.name }}</span><span v-if="source.is_official" class="admin-source-badge" :title="t.official" :aria-label="t.official">🏛️</span><span v-if="source.is_recommended" class="admin-source-badge" :title="t.recommended" :aria-label="t.recommended">⭐</span></span></td><td class="source-url-cell">{{ source.base_url }}</td><td>{{ source.region || '—' }}</td><td>{{ source.probe_mode === 'docker_pull' ? t.dockerPullProbe : t.registryProbe }}</td><td>{{ formatDateTime(source.created_at) || t.unknown }}</td><td><span class="state" :class="source.status">{{ statusLabel(source.status, t) }}</span></td><td class="table-actions"><button class="icon-button status-action" :class="source.enabled === false ? 'status-action-enable' : 'status-action-disable'" type="button" @click="toggleEnabled(source)">{{ source.enabled === false ? t.enable : t.disable }}</button><button class="icon-button" type="button" @click="probe(source)">{{ t.probe }}</button><button class="icon-button" type="button" @click="openEdit(source)">{{ t.edit }}</button><button class="icon-button danger-button" type="button" :disabled="deletingSourceId !== null" @click="remove(source)">{{ deletingSourceId === source.id ? t.loading : t.remove }}</button></td></tr></tbody></template>
  </BaseTable>
  <Pagination :page="page" :page-count="pageCount" :total="filteredSources.length" :page-size="pageSize" :page-size-options="[10, 25, 50, 100]" @change="page = $event" @update:page-size="changePageSize" />
</template>
