<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { adminApi, adminRequest } from '../../admin-api'
import { type AdminProxy, type AdminSettings, type Category, type ProxyConfig } from '../../api'
import { useI18n } from '../../i18n'
import { formatDateTime } from '../../time'
import FormField from '../FormField.vue'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ error: [message: string]; notice: [message: string] }>()
const { locale, t } = useI18n()
const settings = ref<AdminSettings>({})
const categories = ref<Category[]>([])
const settingEntries = ref<Array<{ key: string; value: string }>>([])
const probeIntervalMinutes = ref(30)
const probeRetryIntervalMinutes = ref(3)
const proxyData = ref<AdminProxy | null>(null)
const proxySaving = ref(false)
const proxyRefreshing = ref(false)
const proxyForm = ref<ProxyConfig>(defaultProxyConfig())

const publicAPIEntry = computed(() => settingEntries.value.find((entry) => entry.key === 'public_api_enabled'))
const otherSettingEntries = computed(() => settingEntries.value.filter((entry) => entry.key !== 'public_api_enabled'))
const proxyStatusLabel = computed(() => {
  const data = proxyData.value
  if (!data?.status_available) return t.value.proxyStatusUnavailable
  if (!data.status.running) return t.value.proxyStatusStopped
  if (!data.status.enabled) return t.value.proxyStatusDisabled
  return data.status.ready ? t.value.proxyStatusReady : t.value.proxyStatusNoRoute
})
const proxyStatusClass = computed(() => {
  const data = proxyData.value
  if (!data?.status_available) return 'unavailable'
  if (!data.status.running) return 'stopped'
  if (!data.status.enabled) return 'disabled'
  return data.status.ready ? 'ready' : 'no-route'
})

function defaultProxyConfig(): ProxyConfig {
  return { enabled: true, transport_mode: 'forward', category_id: 'dockerhub', route_max_age_minutes: 120, failure_cooldown_seconds: 30, max_concurrent: 64, max_range_mb: 256, max_manifest_mb: 8 }
}

const fail = (error: unknown, fallback = t.value.apiError) => emit('error', error instanceof Error ? error.message : fallback)
function setBooleanSetting(entry: { value: string }, event: Event) { entry.value = (event.target as HTMLInputElement).checked ? 'true' : 'false' }
function parseSettingValue(value: unknown) {
  if (typeof value === 'object' && value !== null && 'value' in value) return (value as { value: unknown }).value
  return value
}

async function refresh() {
  try {
    const [systemSettings, currentProxy, categoryList] = await Promise.all([adminApi.settings(props.token), adminApi.proxy(props.token), adminApi.categories(props.token)])
    settings.value = systemSettings
    categories.value = categoryList
    proxyData.value = currentProxy
    proxyForm.value = { ...defaultProxyConfig(), ...currentProxy.config }
    probeIntervalMinutes.value = Number(parseSettingValue(systemSettings.probe_interval_minutes)) || 30
    probeRetryIntervalMinutes.value = Number(parseSettingValue(systemSettings.probe_retry_interval_minutes)) || 3
    settingEntries.value = Object.entries(systemSettings)
      .filter(([key]) => !['probe_interval_minutes', 'probe_retry_interval_minutes'].includes(key))
      .map(([key, value]) => ({ key, value: typeof value === 'string' ? value : JSON.stringify(value) }))
  } catch (error) { fail(error) }
}

async function refreshProxy() {
  proxyRefreshing.value = true
  try {
    const currentProxy = await adminApi.proxy(props.token)
    proxyData.value = currentProxy
    proxyForm.value = { ...defaultProxyConfig(), ...currentProxy.config }
  } catch (error) { fail(error) } finally { proxyRefreshing.value = false }
}

async function saveSettings() {
  try {
    const payload: AdminSettings = {
      probe_interval_minutes: Math.max(1, Math.min(1440, Math.round(probeIntervalMinutes.value))),
      probe_retry_interval_minutes: Math.max(1, Math.min(1440, Math.round(probeRetryIntervalMinutes.value))),
    }
    for (const entry of settingEntries.value) {
      try { payload[entry.key] = JSON.parse(entry.value) } catch { payload[entry.key] = entry.value }
    }
    await adminRequest<void>(props.token, '/admin/settings', { method: 'PUT', body: JSON.stringify(payload) })
    emit('notice', t.value.saveSuccess)
    await refresh()
  } catch (error) { fail(error, t.value.settingsSaveError) }
}

async function saveProxy() {
  proxySaving.value = true
  try {
    const result = await adminApi.updateProxy(props.token, { ...proxyForm.value })
    proxyData.value = result
    proxyForm.value = { ...defaultProxyConfig(), ...result.config }
    emit('notice', t.value.proxySaveSuccess)
  } catch (error) { fail(error, t.value.proxySaveError) } finally { proxySaving.value = false }
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
          <h3>{{ locale === 'zh' ? '启用公共查询 API 接口' : 'Enable public query API' }}</h3>
          <p>{{ locale === 'zh' ? '允许公开页面查询镜像源状态。' : 'Allow public pages to query registry status.' }}</p>
          <label class="settings-toggle"><input type="checkbox" :checked="publicAPIEntry.value === 'true'" @change="setBooleanSetting(publicAPIEntry, $event)"><span>{{ publicAPIEntry.value === 'true' ? t.enabled : t.disabled }}</span></label>
        </div>
      </div>
      <div v-if="otherSettingEntries.length" class="settings-other-grid"><FormField v-for="entry in otherSettingEntries" :key="entry.key" :label="entry.key"><input v-model="entry.value"></FormField></div>
      <div class="settings-footer"><button class="refresh" type="button" @click="saveSettings">{{ t.saveSettings }}</button></div>
    </section>

    <section class="panel settings-card proxy-settings-card">
      <div class="settings-card-heading">
        <div><p class="eyebrow">REGISTRY PROXY</p><h2>{{ t.proxySettingsTitle }}</h2></div>
        <span class="proxy-status-chip" :class="proxyStatusClass"><i></i>{{ proxyStatusLabel }}</span>
      </div>
      <p class="settings-description">{{ t.proxySettingsDescription }}</p>
      <div class="proxy-status-row">
        <span>{{ t.proxyRuntimeStatus }}：{{ proxyData?.status_available ? `${proxyData.status.actual_port} · ${proxyData.status.candidate_count} ${t.proxyCandidates}` : '—' }}</span>
        <span v-if="proxyData?.status.last_seen_at">{{ t.proxyLastSeen }}：{{ formatDateTime(proxyData.status.last_seen_at) }}</span>
        <button class="refresh" type="button" :disabled="proxyRefreshing" @click="refreshProxy">{{ proxyRefreshing ? t.loading : t.refresh }}</button>
      </div>
      <div class="proxy-settings-grid">
        <div class="settings-option-card proxy-enable-card">
          <h3>{{ t.proxyEnabled }}</h3>
          <p>{{ t.proxyEnabledHint }}</p>
          <label class="settings-toggle"><input v-model="proxyForm.enabled" type="checkbox"><span>{{ proxyForm.enabled ? t.enabled : t.disabled }}</span></label>
        </div>
        <div class="settings-option-card">
          <h3>{{ t.proxyTransportMode }}</h3>
          <p>{{ t.proxyTransportModeHint }}</p>
          <select v-model="proxyForm.transport_mode">
            <option value="forward">{{ t.proxyForwardMode }}</option>
            <option value="redirect">{{ t.proxyRedirectMode }}</option>
          </select>
        </div>
        <div class="settings-option-card proxy-category-card">
          <h3>{{ t.proxyCategory }}</h3>
          <p>{{ t.proxyCategoryHint }}</p>
          <select v-model="proxyForm.category_id">
            <option v-if="proxyForm.category_id && !categories.some(category => category.id === proxyForm.category_id)" :value="proxyForm.category_id">{{ proxyForm.category_id }}</option>
            <option v-for="category in categories" :key="category.id" :value="category.id">{{ category.slug }} · {{ category.name }}</option>
          </select>
        </div>
        <div class="settings-option-card proxy-route-age-card">
          <h3>{{ t.proxyRouteAge }}</h3>
          <p>{{ t.proxyRouteAgeHint }}</p>
          <input v-model.number="proxyForm.route_max_age_minutes" type="number" min="1" max="10080">
        </div>
        <div class="settings-option-card proxy-cooldown-card">
          <h3>{{ t.proxyCooldown }}</h3>
          <p>{{ t.proxyCooldownHint }}</p>
          <input v-model.number="proxyForm.failure_cooldown_seconds" type="number" min="1" max="3600">
        </div>
        <div class="settings-option-card proxy-forward-only-card" :class="{ 'is-locked': proxyForm.transport_mode !== 'forward' }">
          <h3>{{ t.proxyConcurrency }}</h3>
          <p>{{ t.proxyConcurrencyHint }}</p>
          <input v-model.number="proxyForm.max_concurrent" type="number" min="1" max="1024" :disabled="proxyForm.transport_mode !== 'forward'">
        </div>
        <div class="settings-option-card proxy-forward-only-card" :class="{ 'is-locked': proxyForm.transport_mode !== 'forward' }">
          <h3>{{ t.proxyRangeLimit }}</h3>
          <p>{{ t.proxyRangeLimitHint }}</p>
          <input v-model.number="proxyForm.max_range_mb" type="number" min="1" max="4096" :disabled="proxyForm.transport_mode !== 'forward'">
        </div>
        <div class="settings-option-card proxy-forward-only-card" :class="{ 'is-locked': proxyForm.transport_mode !== 'forward' }">
          <h3>{{ t.proxyManifestLimit }}</h3>
          <p>{{ t.proxyManifestLimitHint }}</p>
          <input v-model.number="proxyForm.max_manifest_mb" type="number" min="1" max="64" :disabled="proxyForm.transport_mode !== 'forward'">
        </div>
      </div>
      <div class="settings-footer"><button class="refresh" type="button" :disabled="proxySaving" @click="saveProxy">{{ proxySaving ? t.saving : t.proxySave }}</button></div>
    </section>
  </div>
</template>
