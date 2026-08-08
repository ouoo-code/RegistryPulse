<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Category, Source } from '../../api'
import { renderContainerdMirrors, renderDockerMirrors, renderNerdctlMirrors, renderOnePanelMirrors, renderPodmanMirrors, renderRegistryPullCommands } from '../../config-generator'
import { useI18n } from '../../i18n'
import BaseDialog from '../BaseDialog.vue'

type Runtime = 'docker' | '1panel' | 'podman' | 'containerd' | 'nerdctl' | 'commands'
const props = defineProps<{ open: boolean; sources: Source[]; category?: Category }>()
const emit = defineEmits<{ close: []; clear: [] }>()
const { t } = useI18n()
const format = ref<Runtime>('docker')
const copied = ref(false)
const isDockerHub = computed(() => props.category?.slug === 'dockerhub')
const mirrors = computed(() => props.sources.map(source => source.base_url).join('\n'))
const output = computed(() => {
  const urls = mirrors.value.split(/\r?\n|,/).map(value => value.trim()).filter(Boolean)
  if (!isDockerHub.value && format.value === 'podman') return renderPodmanMirrors(urls, props.category?.slug || 'custom')
  if (!isDockerHub.value) return renderRegistryPullCommands(urls, props.category?.slug || 'custom')
  if (format.value === 'docker') return renderDockerMirrors(urls)
  if (format.value === '1panel') return renderOnePanelMirrors(urls)
  if (format.value === 'podman') return renderPodmanMirrors(urls)
  if (format.value === 'nerdctl') return renderNerdctlMirrors(urls)
  return renderContainerdMirrors(urls)
})

watch(() => [props.open, props.category?.slug], () => {
  format.value = isDockerHub.value ? 'docker' : 'commands'
})

async function copy() {
  try { await navigator.clipboard?.writeText(output.value) } catch { /* clipboard permissions are optional */ }
  copied.value = true
  setTimeout(() => copied.value = false, 1500)
}
</script>

<template>
  <BaseDialog :open="open" :title="t.configDialogTitle" @close="emit('close')">
    <div class="home-config-dialog">
      <div class="config-dialog-meta"><span class="live-dot"></span>{{ t.selectedSourcesPrefix }}{{ sources.length }}{{ t.selectedSourcesSuffix }}<span class="config-dialog-hint">{{ isDockerHub ? t.dockerHubConfigHint : t.registryConfigHint }}</span></div>
      <div class="config-dialog-grid">
        <section class="config-dialog-input">
          <label>{{ t.runtime }}<select v-model="format"><template v-if="isDockerHub"><option value="docker">{{ t.docker }} · daemon.json</option><option value="1panel">1Panel · Docker runtime</option></template><option value="podman">{{ t.podman }} · registries.conf</option><template v-if="isDockerHub"><option value="containerd">Containerd · hosts.toml</option><option value="nerdctl">Nerdctl · hosts.toml</option></template><option v-else value="commands">{{ t.pullTagCommands }}</option></select></label>
          <div v-if="!isDockerHub" class="config-dialog-category-note"><strong>{{ category?.slug || t.sourceCategories }}</strong><span>{{ format === 'podman' ? t.podmanRegistryHint : t.registryCommandHint }}</span></div>
          <div class="selected-source-list"><span v-for="source in sources" :key="source.id" :title="source.base_url">{{ source.name }} · {{ source.base_url }}</span></div>
        </section>
        <section class="config-dialog-output"><div class="config-dialog-output-head"><span class="eyebrow">{{ t.generatedConfig }}</span><button class="refresh" type="button" @click="copy">{{ copied ? t.copied : t.copy }}</button></div><pre tabindex="0"><code>{{ output }}</code></pre></section>
      </div>
      <div class="config-dialog-actions"><button class="icon-button" type="button" @click="emit('clear'); emit('close')">{{ t.clearSelection }}</button><button class="refresh" type="button" @click="emit('close')">{{ t.done }}</button></div>
    </div>
  </BaseDialog>
</template>
