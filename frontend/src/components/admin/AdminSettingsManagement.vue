<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { adminApi, adminRequest } from '../../admin-api'
import { type AdminSettings, type TotpSettings } from '../../api'
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
const totp = ref<TotpSettings>({ enabled: false, secret: '' })
const totpSecret = ref('')
const totpCode = ref('')
const fail = (error: unknown, fallback = t.value.apiError) => emit('error', error instanceof Error ? error.message : fallback)
function isPublicAPI(entry: { key: string }) { return entry.key === 'public_api_enabled' }
function publicAPILabel() { return locale.value === 'zh' ? '启用公共查询 API 接口' : 'Enable public query API' }
function setBooleanSetting(entry: { value: string }, event: Event) { entry.value = (event.target as HTMLInputElement).checked ? 'true' : 'false' }
async function refresh() {
  try {
    settings.value = await adminApi.settings(props.token)
    totp.value = await adminApi.totp(props.token)
    totpSecret.value = totp.value.secret || ''
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
async function generateTOTP() { try { totp.value = await adminRequest<TotpSettings>(props.token, '/admin/totp', { method: 'POST', body: JSON.stringify({ action: 'generate' }) }); totpSecret.value = totp.value.secret || ''; emit('notice', t.value.saveSuccess) } catch (error) { fail(error) } }
async function enableTOTP() { try { await adminRequest<void>(props.token, '/admin/totp', { method: 'POST', body: JSON.stringify({ action: 'enable', secret: totpSecret.value, code: totpCode.value }) }); totp.value.enabled = true; totpCode.value = ''; emit('notice', t.value.saveSuccess) } catch (error) { fail(error) } }
async function disableTOTP() { try { await adminRequest<void>(props.token, '/admin/totp', { method: 'POST', body: JSON.stringify({ action: 'disable', code: totpCode.value }) }); totp.value.enabled = false; totpCode.value = ''; emit('notice', t.value.saveSuccess) } catch (error) { fail(error) } }
onMounted(refresh)
watch(() => props.token, refresh)
defineExpose({ refresh })
</script>

<template>
  <div class="settings-layout">
    <section class="panel settings-card settings-totp">
      <div class="settings-card-heading"><div><p class="eyebrow">TOTP</p><h2>{{ t.totpTitle }}</h2></div><span class="enabled-state" :class="{ disabled: !totp.enabled }"><i></i>{{ totp.enabled ? t.enabled : t.disabled }}</span></div>
      <p class="settings-description">{{ t.totpDescription }}</p>
      <div class="settings-actions"><button class="icon-button" type="button" @click="generateTOTP">{{ t.generateSecret }}</button><button v-if="totpSecret && !totp.enabled" class="icon-button" type="button" @click="enableTOTP">{{ t.enableTotp }}</button><button v-if="totp.enabled" class="icon-button danger-button" type="button" @click="disableTOTP">{{ t.disableTotp }}</button></div>
      <div v-if="totpSecret" class="settings-secret-grid"><FormField :label="t.secret"><input v-model="totpSecret" readonly></FormField><FormField :label="t.verificationCode"><input v-model="totpCode" inputmode="numeric" maxlength="6" :placeholder="t.verificationCode"></FormField></div>
      <p v-if="totpSecret" class="settings-description">{{ t.totpHint }}</p>
    </section>

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
