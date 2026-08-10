<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { adminApi, adminRequest } from '../../admin-api'
import { type AdminSettings } from '../../api'
import { useI18n } from '../../i18n'
import FormField from '../FormField.vue'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ error: [message: string]; notice: [message: string] }>()
const { locale, t } = useI18n()
const settings = ref<AdminSettings>({})
const settingEntries = ref<Array<{ key: string; value: string }>>([])
const probeIntervalMinutes = ref(30)
const probeRetryIntervalMinutes = ref(3)
const publicAPIEntry = computed(() => settingEntries.value.find((entry) => isPublicAPI(entry)))
const otherSettingEntries = computed(() => settingEntries.value.filter((entry) => !isPublicAPI(entry)))
const fail = (error: unknown, fallback = t.value.apiError) => emit('error', error instanceof Error ? error.message : fallback)
function isPublicAPI(entry: { key: string }) { return entry.key === 'public_api_enabled' }
function publicAPILabel() { return locale.value === 'zh' ? '启用公共查询 API 接口' : 'Enable public query API' }
function setBooleanSetting(entry: { value: string }, event: Event) { entry.value = (event.target as HTMLInputElement).checked ? 'true' : 'false' }
async function refresh() {
  try {
    settings.value = await adminApi.settings(props.token)
    const rawInterval = settings.value.probe_interval_minutes
    const interval = typeof rawInterval === 'object' && rawInterval !== null && 'value' in rawInterval ? rawInterval.value : rawInterval
    probeIntervalMinutes.value = Number(interval) || 30
    const rawRetryInterval = settings.value.probe_retry_interval_minutes
    const retryInterval = typeof rawRetryInterval === 'object' && rawRetryInterval !== null && 'value' in rawRetryInterval ? rawRetryInterval.value : rawRetryInterval
    probeRetryIntervalMinutes.value = Number(retryInterval) || 3
    settingEntries.value = Object.entries(settings.value).filter(([key]) => !['probe_interval_minutes', 'probe_retry_interval_minutes'].includes(key)).map(([key, value]) => ({ key, value: typeof value === 'string' ? value : JSON.stringify(value) }))
  } catch (error) { fail(error) }
}
async function saveSettings() {
  try {
    const payload: AdminSettings = { probe_interval_minutes: Math.max(1, Math.min(1440, Math.round(probeIntervalMinutes.value))), probe_retry_interval_minutes: Math.max(1, Math.min(1440, Math.round(probeRetryIntervalMinutes.value))) }
    for (const entry of settingEntries.value) { try { payload[entry.key] = JSON.parse(entry.value) } catch { payload[entry.key] = entry.value } }
    await adminRequest<void>(props.token, '/admin/settings', { method: 'PUT', body: JSON.stringify(payload) }); emit('notice', t.value.saveSuccess); await refresh()
  } catch (error) { fail(error, t.value.settingsSaveError) }
}
onMounted(refresh)
watch(() => props.token, refresh)
defineExpose({ refresh })
</script>

<template>
  <div class="settings-layout">
    <section class="panel settings-card settings-advanced">
      <div class="settings-card-heading"><div><p class="eyebrow">SYSTEM</p><h2>{{ t.settingsTitle }}</h2></div></div>
      <div class="settings-option-grid">
        <div class="settings-option-card">
          <h3>{{ t.probeInterval }}</h3>
          <p>{{ t.probeIntervalHint }}</p>
          <input v-model.number="probeIntervalMinutes" type="number" min="1" max="1440">
        </div>
        <div class="settings-option-card">
          <h3>{{ t.probeRetryInterval }}</h3>
          <p>{{ t.probeRetryIntervalHint }}</p>
          <input v-model.number="probeRetryIntervalMinutes" type="number" min="1" max="1440">
        </div>
        <div v-if="publicAPIEntry" class="settings-option-card settings-public-option">
          <h3>{{ publicAPILabel() }}</h3>
          <p>{{ locale === 'zh' ? '允许公开页面查询镜像源状态。' : 'Allow public pages to query registry status.' }}</p>
          <label class="settings-toggle"><input type="checkbox" :checked="publicAPIEntry.value.value === 'true'" @change="setBooleanSetting(publicAPIEntry.value, $event)"><span>{{ publicAPIEntry.value.value === 'true' ? t.enabled : t.disabled }}</span></label>
        </div>
      </div>
      <div v-if="otherSettingEntries.length" class="settings-other-grid"><FormField v-for="entry in otherSettingEntries" :key="entry.key" :label="entry.key"><input v-model="entry.value"></FormField></div>
      <div class="settings-footer"><button class="refresh" type="button" @click="saveSettings">{{ t.saveSettings }}</button></div>
    </section>
  </div>
</template>
