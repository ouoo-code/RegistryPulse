<script setup lang="ts">
import { ref, watch } from 'vue'
import { adminApi, adminRequest } from '../../admin-api'
import type { TotpSettings } from '../../api'
import { useI18n } from '../../i18n'
import BaseDialog from '../BaseDialog.vue'
import FormField from '../FormField.vue'

const props = defineProps<{ open: boolean; token: string }>()
const emit = defineEmits<{ close: []; error: [message: string]; notice: [message: string] }>()
const { t } = useI18n()
const settings = ref<TotpSettings>({ enabled: false, configured: false })
const secret = ref('')
const otpauthURI = ref('')
const code = ref('')
const busy = ref(false)

function resetTransient() {
  secret.value = ''
  otpauthURI.value = ''
  code.value = ''
}

async function load() {
  if (!props.open || !props.token) return
  try {
    settings.value = await adminApi.totp(props.token)
    resetTransient()
  } catch (error) {
    emit('error', error instanceof Error ? error.message : t.value.apiError)
  }
}

async function generate() {
  busy.value = true
  try {
    const result = await adminRequest<TotpSettings>(props.token, '/admin/totp', { method: 'POST', body: JSON.stringify({ action: 'generate' }) })
    secret.value = result.secret || ''
    otpauthURI.value = result.otpauth_uri || ''
    code.value = ''
  } catch (error) {
    emit('error', error instanceof Error ? error.message : t.value.apiError)
  } finally { busy.value = false }
}

async function enable() {
  busy.value = true
  try {
    await adminRequest<void>(props.token, '/admin/totp', { method: 'POST', body: JSON.stringify({ action: 'enable', secret: secret.value, code: code.value }) })
    settings.value = { enabled: true, configured: true }
    code.value = ''
    emit('notice', t.value.saveSuccess)
  } catch (error) {
    emit('error', error instanceof Error ? error.message : t.value.apiError)
  } finally { busy.value = false }
}

async function disable() {
  busy.value = true
  try {
    await adminRequest<void>(props.token, '/admin/totp', { method: 'POST', body: JSON.stringify({ action: 'disable', code: code.value }) })
    settings.value = { enabled: false, configured: true }
    code.value = ''
    emit('notice', t.value.saveSuccess)
  } catch (error) {
    emit('error', error instanceof Error ? error.message : t.value.apiError)
  } finally { busy.value = false }
}

watch(() => props.open, (open) => { if (open) load(); else resetTransient() })
</script>

<template>
  <BaseDialog :open="open" :title="t.totpAccount" size="small" @close="emit('close')">
    <div class="totp-dialog-form">
      <p class="settings-description">{{ t.totpAccountHint }}</p>
      <div class="totp-status-row"><span>{{ t.status }}</span><span class="enabled-state" :class="{ disabled: !settings.enabled }"><i></i>{{ settings.enabled ? t.enabled : t.disabled }}</span></div>
      <div class="editor-form-actions totp-dialog-actions">
        <button class="icon-button" type="button" :disabled="busy" @click="generate">{{ t.generateSecret }}</button>
        <button v-if="settings.enabled" class="icon-button danger-button" type="button" :disabled="busy" @click="disable">{{ t.disableTotp }}</button>
      </div>
      <template v-if="secret">
        <FormField :label="t.secret"><input :value="secret" readonly></FormField>
        <FormField v-if="otpauthURI" :label="t.otpUri"><input :value="otpauthURI" readonly></FormField>
        <FormField :label="t.verificationCode"><input v-model="code" inputmode="numeric" maxlength="6" autocomplete="one-time-code" required></FormField>
        <p class="settings-description">{{ t.totpHint }}</p>
        <div class="editor-form-actions"><button class="refresh" type="button" :disabled="busy" @click="enable">{{ t.enableTotp }}</button></div>
      </template>
      <p v-else-if="settings.configured" class="settings-description">{{ t.totpConfigured }}</p>
    </div>
  </BaseDialog>
</template>
