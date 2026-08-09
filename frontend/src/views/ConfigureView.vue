<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { api, type Category, type Source } from '../api'
import { renderContainerdMirrors, renderDockerMirrors, renderNerdctlMirrors, renderOnePanelMirrors, renderPodmanMirrors, renderRegistryPullCommands } from '../config-generator'
import { useI18n } from '../i18n'

type Runtime = 'docker' | '1panel' | 'podman' | 'containerd' | 'nerdctl' | 'commands'
const { t } = useI18n()
const format = ref<Runtime>('docker')
const mirrors = ref('')
const copied = ref(false)
const categories = ref<Category[]>([])
const sources = ref<Source[]>([])
const selectedCategory = ref('')
const selectedIds = ref<string[]>([])
const sourceQuery = ref('')
const loading = ref(true)
const error = ref('')
let errorTimer: number | undefined

function showError(message: string) {
  if (errorTimer !== undefined) window.clearTimeout(errorTimer)
  error.value = message
  errorTimer = window.setTimeout(() => {
    error.value = ''
    errorTimer = undefined
  }, 5000)
}

const category = computed(() => categories.value.find(item => item.id === selectedCategory.value))
const isDockerHub = computed(() => category.value?.id === 'dockerhub' || (!category.value && !selectedCategory.value))
const visibleSources = computed(() => sources.value.filter(source => {
  const haystack = (source.name + ' ' + source.base_url + ' ' + (source.provider || '')).toLowerCase()
  return source.category_id === selectedCategory.value && (!sourceQuery.value || haystack.includes(sourceQuery.value.toLowerCase()))
}))
const selectedSources = computed(() => sources.value.filter(source => selectedIds.value.includes(source.id)))
const urls = computed(() => mirrors.value.split(/\r?\n|,/).map(value => value.trim()).filter(Boolean))
const output = computed(() => {
  if (!isDockerHub.value && format.value === 'podman') return renderPodmanMirrors(urls.value, category.value?.id || 'custom')
  if (!isDockerHub.value) return renderRegistryPullCommands(urls.value, category.value?.id || 'custom')
  if (format.value === 'docker') return renderDockerMirrors(urls.value)
  if (format.value === '1panel') return renderOnePanelMirrors(urls.value)
  if (format.value === 'podman') return renderPodmanMirrors(urls.value)
  if (format.value === 'nerdctl') return renderNerdctlMirrors(urls.value)
  return renderContainerdMirrors(urls.value)
})

function syncMirrors() {
  mirrors.value = selectedSources.value.map(source => source.base_url).join('\n')
}
function chooseCategory(id: string) {
  selectedCategory.value = id
  selectedIds.value = []
  syncMirrors()
}
function toggleSource(id: string) {
  if (selectedIds.value.includes(id)) selectedIds.value = selectedIds.value.filter(value => value !== id)
  else if (selectedIds.value.length < 10) selectedIds.value = [...selectedIds.value, id]
  syncMirrors()
}
function selectOnline() {
  selectedIds.value = visibleSources.value.filter(source => source.status === 'online').slice(0, 10).map(source => source.id)
  syncMirrors()
}
function clearSelection() {
  selectedIds.value = []
  mirrors.value = ''
}
async function copy() {
  try { await navigator.clipboard?.writeText(output.value) } catch { /* clipboard permissions are optional */ }
  copied.value = true
  setTimeout(() => copied.value = false, 1500)
}

watch(() => category.value?.id, id => {
  format.value = id === 'dockerhub' ? 'docker' : 'commands'
})

onMounted(async () => {
  try {
    const [loadedCategories, loadedSources] = await Promise.all([
      api<Category[]>('/public/categories'),
      api<Source[]>('/public/sources'),
    ])
    categories.value = loadedCategories
    sources.value = loadedSources
    selectedCategory.value = loadedCategories.find(item => item.slug.toLowerCase() === 'dockerhub')?.id || loadedCategories[0]?.id || ''
    const initial = loadedSources.filter(source => source.category_id === selectedCategory.value && source.status === 'online').slice(0, 3)
    selectedIds.value = initial.map(source => source.id)
    syncMirrors()
  } catch {
    showError(t.value.apiError)
  } finally {
    loading.value = false
  }
})

onUnmounted(() => {
  if (errorTimer !== undefined) window.clearTimeout(errorTimer)
})
</script>

<template>
  <main class="page config-page">
    <section class="subhero compact-subhero"><div><p class="eyebrow">{{ t.configuration }}</p><h1>{{ t.configure }}</h1><p>{{ t.prevent }}</p></div></section>
    <div v-if="error" class="home-alert">{{ error }}</div>
    <section class="generator-workspace">
      <section class="generator-input panel">
        <div class="section-title"><div><p class="eyebrow">01 · {{ t.input }}</p><h2>{{ t.runtimeAndMirrors }}</h2></div></div>
        <label>{{ t.sourceCategories }}<select v-model="selectedCategory" @change="chooseCategory(selectedCategory)"><option v-for="item in categories" :key="item.id" :value="item.id">{{ item.slug }} · {{ item.name }}</option></select></label>
        <div class="configure-source-box">
          <div class="configure-source-head"><span>{{ t.sources }} · {{ selectedIds.length }} {{ t.selected }}</span><div><button type="button" class="text-button" @click="selectOnline">{{ t.online }}</button><button type="button" class="text-button" @click="clearSelection">{{ t.clearSelection }}</button></div></div>
          <input v-model="sourceQuery" :placeholder="t.search" />
          <template v-if="loading"><div class="configure-source-empty">{{ t.loading }}</div></template>
          <template v-else><label v-for="source in visibleSources" :key="source.id" class="configure-source-option"><input type="checkbox" :checked="selectedIds.includes(source.id)" @change="toggleSource(source.id)" /><span>{{ source.name }}<small>{{ source.base_url }}</small></span><b :class="'source-status-' + source.status">{{ source.status }}</b></label><div v-if="!visibleSources.length" class="configure-source-empty">{{ t.noResults }}</div></template>
        </div>
        <label>{{ t.runtime }}<select v-model="format"><template v-if="isDockerHub"><option value="docker">{{ t.docker }} · daemon.json</option><option value="1panel">1Panel · Docker runtime</option></template><option value="podman">{{ t.podman }} · registries.conf</option><template v-if="isDockerHub"><option value="containerd">Containerd · hosts.toml</option><option value="nerdctl">Nerdctl · hosts.toml</option></template><option v-else value="commands">{{ t.pullTagCommands }}</option></select></label>
        <div v-if="!isDockerHub" class="config-dialog-category-note"><strong>{{ category?.slug || t.sourceCategories }}</strong><span>{{ format === 'podman' ? t.podmanRegistryHint : t.registryCommandHint }}</span></div>
        <label>{{ t.mirrorUrls }}<textarea v-model="mirrors" rows="7" spellcheck="false" /></label>
      </section>
      <section class="generator-output panel"><div class="section-title"><div><p class="eyebrow">02 · {{ t.output }}</p><h2>{{ t.generatedConfig }}</h2></div><button class="refresh" type="button" @click="copy">{{ copied ? t.copied : t.copy }}</button></div><pre :aria-label="t.generatedConfig" tabindex="0"><code>{{ output }}</code></pre></section>
    </section>
  </main>
</template>
