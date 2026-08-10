<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { adminApi } from '../../admin-api'
import { type Category, type CategoryInput, type Source, type TestImage } from '../../api'
import { useI18n } from '../../i18n'
import BaseDialog from '../BaseDialog.vue'
import BaseTable from '../BaseTable.vue'
import FormField from '../FormField.vue'
import SortIndicator from '../SortIndicator.vue'
import { formatDateTime } from '../../time'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ notice: [message: string]; error: [message: string] }>()
const { t } = useI18n()
const categories = ref<Category[]>([])
const testImages = ref<TestImage[]>([])
const categorySort = ref<'sort_order' | 'name' | 'status'>('sort_order')
const categoryAscending = ref(true)
const editorOpen = ref(false)
const editingId = ref<string | null>(null)
const form = ref<CategoryInput>(emptyForm())
function imageAppliesTo(image: TestImage, categoryID: string, probeMode: string) {
  const categoryIDs = [...(image.category_ids || []), ...(image.applicable_category_ids || [])]
  const probeModes = [...(image.probe_modes || []), ...(image.applicable_probe_modes || [])]
  const categoryMatches = !categoryID || !categoryIDs.length || categoryIDs.includes(categoryID)
  return categoryMatches && (!probeModes.length || probeModes.includes(probeMode))
}
const applicableTestImages = computed(() => testImages.value.filter(image => image.enabled && imageAppliesTo(image, form.value.id, form.value.default_probe_mode || 'registry')))
const displayedCategories = computed(() => [...categories.value].sort((left, right) => {
  let result = categorySort.value === 'name' ? left.name.localeCompare(right.name) : categorySort.value === 'status' ? Number(right.enabled !== false) - Number(left.enabled !== false) : (left.sort_order || 0) - (right.sort_order || 0)
  if (!result) result = left.name.localeCompare(right.name)
  return categoryAscending.value ? result : -result
}))
function sortCategories(key: 'sort_order' | 'name' | 'status') {
  if (categorySort.value === key) categoryAscending.value = !categoryAscending.value
  else { categorySort.value = key; categoryAscending.value = true }
}
function sortActive(key: 'sort_order' | 'name' | 'status') { return categorySort.value === key }

function emptyForm(): CategoryInput {
  return { id: '', slug: '', name: '', description: '', auth_type: 'registry', default_test_repository: 'library/alpine', default_test_tag: 'latest', default_test_image_id: '', default_probe_mode: 'registry', default_timeout_seconds: 15, enabled: true, sort_order: 100 }
}

async function refresh() {
  try {
    const [categoryList, sourceList, testImageList] = await Promise.all([adminApi.categories(props.token), adminApi.sources(props.token), adminApi.testImages(props.token)])
    testImages.value = testImageList
    const counts = new Map<string, number>()
    for (const source of sourceList) counts.set(source.category_id, (counts.get(source.category_id) || 0) + 1)
    categories.value = categoryList.map(category => ({ ...category, source_count: counts.get(category.id) || 0 })).sort((left, right) => (left.sort_order || 0) - (right.sort_order || 0) || left.name.localeCompare(right.name))
  } catch (error) { emit('error', error instanceof Error ? error.message : t.value.apiError) }
}

function openCreate() {
  editingId.value = null
  form.value = emptyForm()
  editorOpen.value = true
}

function openEdit(category: Category) {
  editingId.value = category.id
  form.value = { id: category.id, slug: category.slug, name: category.name, description: category.description, icon: category.icon, official_url: category.official_url, default_test_repository: category.default_test_repository, default_test_tag: category.default_test_tag, default_test_image_id: category.default_test_image_id || '', default_probe_mode: category.default_probe_mode || 'registry', default_timeout_seconds: category.default_timeout_seconds || 15, default_manifest_path: category.default_manifest_path, auth_type: category.auth_type, enabled: category.enabled !== false, sort_order: category.sort_order || 0 }
  editorOpen.value = true
}

function closeEditor() { editorOpen.value = false; editingId.value = null }

async function save() {
  try {
    if (editingId.value) await adminApi.updateCategory(props.token, editingId.value, form.value)
    else await adminApi.createCategory(props.token, form.value)
    emit('notice', t.value.saveSuccess)
    closeEditor()
    await refresh()
  } catch (error) { emit('error', error instanceof Error ? error.message : t.value.categorySaveError) }
}

async function toggleEnabled(category: Category) {
  try {
    await adminApi.updateCategory(props.token, category.id, { id: category.id, slug: category.slug, name: category.name, description: category.description, icon: category.icon, official_url: category.official_url, default_test_repository: category.default_test_repository, default_test_tag: category.default_test_tag, default_test_image_id: category.default_test_image_id, default_probe_mode: category.default_probe_mode, default_timeout_seconds: category.default_timeout_seconds, default_manifest_path: category.default_manifest_path, auth_type: category.auth_type, enabled: category.enabled === false, sort_order: category.sort_order || 0 })
    await refresh()
  } catch (error) { emit('error', error instanceof Error ? error.message : t.value.categorySaveError) }
}

async function remove(category: Category) {
  if ((category.source_count || 0) > 0) {
    emit('error', `${t.value.categoryInUse}（${category.source_count}）`)
    return
  }
  if (!confirm(t.value.confirmDelete)) return
  try { await adminApi.deleteCategory(props.token, category.id); emit('notice', t.value.deleteSuccess); await refresh() } catch (error) { emit('error', error instanceof Error ? error.message : t.value.categoryDeleteError) }
}

onMounted(refresh)
watch([() => form.value.id, () => form.value.default_probe_mode], () => {
  if (form.value.default_test_image_id && !applicableTestImages.value.some(image => image.id === form.value.default_test_image_id)) form.value.default_test_image_id = ''
})
defineExpose({ refresh })
</script>

<template>
  <section class="panel admin-table admin-resource-section admin-category-actions">
    <div class="section-heading admin-categories-heading"><span class="eyebrow">{{ t.sourceCategories }}</span><button class="refresh" type="button" @click="openCreate">{{ t.add }}</button></div>
  </section>

  <BaseDialog size="category" :open="editorOpen" :title="`${editingId ? t.edit : t.add} ${t.categoriesTitle}`" @close="closeEditor">
    <form class="admin-editor-form category-editor-form" @submit.prevent="save">
      <FormField :label="t.sortOrder"><input v-model.number="form.sort_order" type="number" min="0" max="999999" required></FormField>
      <FormField :label="t.id"><input v-model="form.id" :readonly="Boolean(editingId)" required></FormField>
      <FormField :label="t.name"><input v-model="form.name" required></FormField>
      <FormField :label="t.categoryKey"><input v-model="form.slug" required></FormField>
      <FormField :label="t.probeMode"><select v-model="form.default_probe_mode"><option value="registry">{{ t.registryProbe }}</option><option value="manifest">{{ t.manifestProbe }}</option><option value="http">{{ t.httpProbe }}</option><option value="docker_pull">{{ t.dockerPullProbe }}</option></select></FormField>
      <FormField :label="t.testImage"><select v-model="form.default_test_image_id"><option value="">{{ t.systemDefaultTestImage }}</option><option v-for="image in applicableTestImages" :key="image.id" :value="image.id">{{ image.reference }} · {{ image.max_bytes / 1048576 }} M</option></select></FormField>
      <FormField :label="t.defaultTimeout"><input v-model.number="form.default_timeout_seconds" type="number" min="1" max="300" required></FormField>
      <FormField :label="t.description"><input v-model="form.description"></FormField>
      <div class="editor-checks category-editor-checks"><label class="checkbox-field"><input v-model="form.enabled" type="checkbox"><span>{{ t.status }}：{{ form.enabled ? t.enabled : t.disabled }}</span></label></div>
      <div class="editor-form-actions"><button class="refresh" type="submit">{{ t.save }}</button><button class="icon-button" type="button" @click="closeEditor">{{ t.cancel }}</button></div>
    </form>
  </BaseDialog>

  <BaseTable class="category-table" :empty="!categories.length ? t.noResults : ''"><template #head><thead><tr><th class="sortable-header" :class="{ active: sortActive('sort_order') }" :aria-sort="sortActive('sort_order') ? (categoryAscending ? 'ascending' : 'descending') : 'none'" @click="sortCategories('sort_order')">{{ t.sortOrder }} <SortIndicator :active="sortActive('sort_order')" :ascending="categoryAscending" /></th><th class="sortable-header" :class="{ active: sortActive('status') }" :aria-sort="sortActive('status') ? (categoryAscending ? 'ascending' : 'descending') : 'none'" @click="sortCategories('status')">{{ t.status }} <SortIndicator :active="sortActive('status')" :ascending="categoryAscending" /></th><th class="sortable-header" :class="{ active: sortActive('name') }" :aria-sort="sortActive('name') ? (categoryAscending ? 'ascending' : 'descending') : 'none'" @click="sortCategories('name')">{{ t.name }} <SortIndicator :active="sortActive('name')" :ascending="categoryAscending" /></th><th>{{ t.id }}</th><th>{{ t.categoryKey }}</th><th>{{ t.associatedSources }}</th><th>{{ t.createdAt }}</th><th>{{ t.actions }}</th></tr></thead></template><template #body><tbody><tr v-for="category in displayedCategories" :key="category.id"><td>{{ category.sort_order || 0 }}</td><td><span class="enabled-state" :class="{ disabled: category.enabled === false }"><i></i>{{ category.enabled === false ? t.disabled : t.enabled }}</span></td><td>{{ category.name }}</td><td>{{ category.id }}</td><td>{{ category.slug }}</td><td>{{ category.source_count || 0 }}</td><td>{{ formatDateTime(category.created_at) || t.unknown }}</td><td class="table-actions"><button class="icon-button status-action" :class="category.enabled === false ? 'status-action-enable' : 'status-action-disable'" type="button" @click="toggleEnabled(category)">{{ category.enabled === false ? t.enable : t.disable }}</button><button class="icon-button" type="button" @click="openEdit(category)">{{ t.edit }}</button><button class="icon-button danger-button" type="button" :title="(category.source_count || 0) > 0 ? t.categoryInUse : ''" @click="remove(category)">{{ t.delete }}</button></td></tr></tbody></template></BaseTable>
</template>
