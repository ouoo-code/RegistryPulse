<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { adminApi, adminRequest } from '../../admin-api'
import { type AdminRole, type AdminSettings, type AdminUser, type Category, type NotificationChannel, type NotificationRule, type ProbeNode, type TestImage, type TestImageInput, type TotpSettings } from '../../api'
import { useI18n } from '../../i18n'
import BaseDialog from '../BaseDialog.vue'
import BaseTable from '../BaseTable.vue'
import FormField from '../FormField.vue'
import { formatDateTime } from '../../time'
import AdminNotificationManagement from './AdminNotificationManagement.vue'
import AdminNotificationRuleManagement from './AdminNotificationRuleManagement.vue'
import AdminSettingsManagement from './AdminSettingsManagement.vue'

const props = defineProps<{ token: string; section: string }>()
const emit = defineEmits<{ error: [message: string]; notice: [message: string] }>()
const { t } = useI18n()
const nodes = ref<ProbeNode[]>([])
const users = ref<AdminUser[]>([])
const roles = ref<AdminRole[]>([])
const notifications = ref<NotificationChannel[]>([])
const notificationRules = ref<NotificationRule[]>([])
const testImages = ref<TestImage[]>([])
const categories = ref<Category[]>([])
const settings = ref<AdminSettings>({})
const settingEntries = ref<Array<{ key: string; value: string }>>([])
const totp = ref<TotpSettings>({ enabled: false, secret: '' })
const totpSecret = ref('')
const totpCode = ref('')
const probeIntervalMinutes = ref(5)
type TestImageForm = Omit<TestImageInput, 'max_bytes'> & { max_mib: number }
const probeModes = ['registry', 'manifest', 'http', 'docker_pull'] as const
const authStrategies = ['anonymous', 'optional', 'required'] as const
function emptyImageForm(): TestImageForm {
  return { reference: '', enabled: true, max_mib: 1, category_ids: [], probe_modes: [], auth_strategy: 'anonymous' }
}
const imageForm = ref<TestImageForm>(emptyImageForm())
const imageEditorOpen = ref(false)
const editingImageID = ref<string | null>(null)
const bytesPerMiB = 1024 * 1024
function bytesToMiB(bytes: number) {
  const value = bytes / bytesPerMiB
  return Number.isInteger(value) ? String(value) : value.toFixed(2).replace(/0+$/, '').replace(/\.$/, '')
}
const fail = (error: unknown, fallback: string) => emit('error', error instanceof Error ? error.message : fallback)

async function refresh() {
  try {
    if (props.section === 'nodes') nodes.value = await adminApi.nodes(props.token)
    if (props.section === 'users') users.value = await adminApi.users(props.token)
    if (props.section === 'roles') roles.value = await adminApi.roles(props.token)
    if (props.section === 'notifications') notifications.value = await adminApi.notifications(props.token)
    if (props.section === 'notification-rules') notificationRules.value = await adminApi.notificationRules(props.token)
    if (props.section === 'test-images') {
      const [imageList, categoryList] = await Promise.all([adminApi.testImages(props.token), adminApi.categories(props.token)])
      testImages.value = imageList
      categories.value = categoryList
    }
    if (props.section === 'settings') {
      settings.value = await adminApi.settings(props.token)
      totp.value = await adminApi.totp(props.token)
      totpSecret.value = totp.value.secret || ''
      const rawInterval = settings.value.probe_interval_minutes
      const intervalValue = typeof rawInterval === 'object' && rawInterval !== null && 'value' in rawInterval ? rawInterval.value : rawInterval
      probeIntervalMinutes.value = Number(intervalValue) || 5
      settingEntries.value = Object.entries(settings.value).filter(([key]) => key !== 'probe_interval_minutes').map(([key, value]) => ({ key, value: typeof value === 'string' ? value : JSON.stringify(value) }))
    }
  } catch (error) { fail(error, t.value.apiError) }
}
async function testNotification(id: string) { try { await adminRequest<void>(props.token, `/admin/notifications/${id}/test`, { method: 'POST' }); emit('notice', t.value.probeQueued) } catch (error) { fail(error, t.value.notificationTestError) } }
function openImageCreate() {
  editingImageID.value = null
  imageForm.value = emptyImageForm()
  imageEditorOpen.value = true
}
function openImageEdit(image: TestImage) {
  editingImageID.value = image.id
  imageForm.value = {
    id: image.id,
    reference: image.reference,
    enabled: image.enabled,
    max_mib: Number(bytesToMiB(image.max_bytes)),
    category_ids: [...(image.applicable_category_ids || image.category_ids || [])],
    probe_modes: [...(image.applicable_probe_modes || image.probe_modes || [])],
    auth_strategy: image.auth_strategy || image.auth?.strategy || image.auth_type || 'anonymous',
  }
  imageEditorOpen.value = true
}
function closeImageEditor() {
  imageEditorOpen.value = false
  editingImageID.value = null
}
async function saveImage() {
  const reference = imageForm.value.reference.trim()
  if (testImages.value.some(image => image.id !== editingImageID.value && image.reference.toLowerCase() === reference.toLowerCase())) {
    fail(new Error('Test image already exists'), t.value.testImageSaveError)
    return
  }
  const payload: TestImageInput = {
    id: editingImageID.value || undefined,
    reference,
    enabled: imageForm.value.enabled,
    max_bytes: Math.round(imageForm.value.max_mib * bytesPerMiB),
    category_ids: [...(imageForm.value.category_ids || [])],
    probe_modes: [...(imageForm.value.probe_modes || [])],
    auth_strategy: imageForm.value.auth_strategy || 'anonymous',
  }
  const path = editingImageID.value ? `/admin/test-images/${encodeURIComponent(editingImageID.value)}` : '/admin/test-images'
  const method = editingImageID.value ? 'PUT' : 'POST'
  try { await adminRequest<void>(props.token, path, { method, body: JSON.stringify(payload) }); emit('notice', t.value.saveSuccess); closeImageEditor(); await refresh() } catch (error) { fail(error, t.value.testImageSaveError) }
}
async function toggleImage(image: TestImage) { try { await adminRequest<void>(props.token, `/admin/test-images/${encodeURIComponent(image.id)}`, { method: 'PUT', body: JSON.stringify({ id: image.id, reference: image.reference, enabled: !image.enabled, max_bytes: image.max_bytes, category_ids: image.applicable_category_ids || image.category_ids || [], probe_modes: image.applicable_probe_modes || image.probe_modes || [], auth_strategy: image.auth_strategy || image.auth?.strategy || image.auth?.type || image.auth_type || 'anonymous' }) }); emit('notice', t.value.saveSuccess); await refresh() } catch (error) { fail(error, t.value.testImageSaveError) } }
async function setDefaultImage(image: TestImage) { try { await adminRequest<void>(props.token, `/admin/test-images/${encodeURIComponent(image.id)}/default`, { method: 'POST' }); emit('notice', t.value.saveSuccess); await refresh() } catch (error) { fail(error, t.value.testImageSaveError) } }
async function removeImage(id: string) {
  if (!confirm(t.value.confirmDelete)) return
  if (!id) { fail(new Error('Test image ID is missing'), t.value.testImageDeleteError); return }
  try { await adminRequest<void>(props.token, `/admin/test-images/${encodeURIComponent(id)}`, { method: 'DELETE' }); emit('notice', t.value.deleteSuccess); await refresh() } catch (error) { fail(error, t.value.testImageDeleteError) }
}
async function saveSettings() { try { const payload: AdminSettings = { probe_interval_minutes: Math.max(1, Math.min(1440, Math.round(probeIntervalMinutes.value))) }; for (const entry of settingEntries.value) { try { payload[entry.key] = JSON.parse(entry.value) } catch { payload[entry.key] = entry.value } } await adminRequest<void>(props.token, '/admin/settings', { method: 'PUT', body: JSON.stringify(payload) }); emit('notice', t.value.saveSuccess); await refresh() } catch (error) { fail(error, t.value.settingsSaveError) } }
async function generateTOTP() { try { const result = await adminRequest<TotpSettings>(props.token, '/admin/totp', { method: 'POST', body: JSON.stringify({ action: 'generate' }) }); totp.value = result; totpSecret.value = result.secret || ''; emit('notice', t.value.saveSuccess) } catch (error) { fail(error, t.value.apiError) } }
async function enableTOTP() { try { await adminRequest<void>(props.token, '/admin/totp', { method: 'POST', body: JSON.stringify({ action: 'enable', secret: totpSecret.value, code: totpCode.value }) }); totp.value.enabled = true; totpCode.value = ''; emit('notice', t.value.saveSuccess) } catch (error) { fail(error, t.value.apiError) } }
async function disableTOTP() { try { await adminRequest<void>(props.token, '/admin/totp', { method: 'POST', body: JSON.stringify({ action: 'disable', code: totpCode.value }) }); totp.value.enabled = false; totpCode.value = ''; emit('notice', t.value.saveSuccess) } catch (error) { fail(error, t.value.apiError) } }
function categoryName(id: string) {
  const category = categories.value.find(item => item.id === id)
  return category ? `${category.slug} · ${category.name}` : id
}
function scopeSummary(values: string[] | undefined, fallback: string) {
  return values?.length ? values.map(value => value === 'registry' ? t.value.registryProbe : value === 'manifest' ? t.value.manifestProbe : value === 'http' ? t.value.httpProbe : value === 'docker_pull' ? t.value.dockerPullProbe : categoryName(value)).join(', ') : fallback
}
function categoryScopeSummary(image: TestImage) {
  const values = [...(image.applicable_category_ids || []), ...(image.category_ids || []), ...(image.applicable_categories || []).map(value => typeof value === 'string' ? value : value.id || value.slug || '')].filter(Boolean)
  return scopeSummary(values, t.value.noLimit)
}
function authStrategyLabel(strategy: string | undefined) {
  if (strategy === 'anonymous') return t.value.authAnonymous
  if (strategy === 'optional') return t.value.authOptional
  if (strategy === 'required') return t.value.authRequired
  return strategy ? t.value.authConfigured : t.value.authNotConfigured
}
function authSummary(image: TestImage) {
  const strategy = image.auth_strategy || image.auth?.strategy || image.auth?.type || image.auth_type
  const configured = image.auth_configured ?? image.has_secret ?? image.auth?.configured ?? image.auth?.secret_configured ?? image.auth?.has_secret
  const status = configured === true ? t.value.authConfigured : configured === false ? t.value.authNotConfigured : t.value.authStatusUnknown
  return `${authStrategyLabel(strategy)} · ${status}`
}
onMounted(refresh)
watch(() => props.section, refresh)
defineExpose({ refresh })
</script>

<template>
  <template v-if="section === 'nodes'"><BaseTable :empty="!nodes.length ? t.noResults : ''"><template #head><thead><tr><th>{{ t.name }}</th><th>{{ t.region }}</th><th>{{ t.version }}</th><th>{{ t.status }}</th><th>{{ t.lastSeen }}</th></tr></thead></template><template #body><tbody><tr v-for="node in nodes" :key="node.id"><td>{{ node.name }}</td><td>{{ node.region }}</td><td>{{ node.version }}</td><td>{{ node.status }}</td><td>{{ formatDateTime(node.last_seen_at) || t.unknown }}</td></tr></tbody></template></BaseTable></template>
  <template v-else-if="section === 'users'"><BaseTable :empty="!users.length ? t.noResults : ''"><template #head><thead><tr><th>{{ t.username }}</th><th>{{ t.adminRoles }}</th><th>{{ t.enabled }}</th></tr></thead></template><template #body><tbody><tr v-for="user in users" :key="user.id"><td>{{ user.username }}</td><td>{{ user.roles }}</td><td>{{ user.active }}</td></tr></tbody></template></BaseTable></template>
  <template v-else-if="section === 'roles'"><BaseTable :empty="!roles.length ? t.noResults : ''"><template #head><thead><tr><th>{{ t.name }}</th><th>{{ t.permissions }}</th></tr></thead></template><template #body><tbody><tr v-for="role in roles" :key="role.name"><td>{{ role.name }}</td><td>{{ role.permissions }}</td></tr></tbody></template></BaseTable></template>
  <AdminNotificationManagement v-else-if="section === 'notifications'" :token="token" @error="emit('error', $event)" @notice="emit('notice', $event)" />
  <AdminNotificationRuleManagement v-else-if="section === 'notification-rules'" :token="token" @error="emit('error', $event)" @notice="emit('notice', $event)" />
  <template v-else-if="section === 'test-images'">
    <section class="panel admin-resource-section">
      <div class="section-heading admin-categories-heading"><span class="eyebrow">{{ t.testImagesTitle }}</span><button class="refresh" type="button" @click="openImageCreate">{{ t.add }}</button></div>
    </section>
    <BaseTable class="test-image-table" :empty="!testImages.length ? t.noResults : ''"><template #head><thead><tr><th>{{ t.reference }}</th><th>{{ t.applicableCategories }}</th><th>{{ t.applicableProbeModes }}</th><th>{{ t.authStrategy }}</th><th>{{ t.status }}</th><th>{{ t.maxBytes }}</th><th>{{ t.createdAt }}</th><th>{{ t.actions }}</th></tr></thead></template><template #body><tbody><tr v-for="image in testImages" :key="image.id"><td>{{ image.reference }} <span v-if="image.is_default" class="enabled-state">· {{ t.defaultImage }}</span></td><td>{{ categoryScopeSummary(image) }}</td><td>{{ scopeSummary(image.applicable_probe_modes || image.probe_modes, t.noLimit) }}</td><td>{{ authSummary(image) }}</td><td><span class="enabled-state" :class="{ disabled: !image.enabled }"><i></i>{{ image.enabled ? t.enabled : t.disabled }}</span></td><td>{{ bytesToMiB(image.max_bytes) }} M</td><td>{{ formatDateTime(image.created_at) || t.unknown }}</td><td class="table-actions"><button class="icon-button status-action" :class="image.enabled ? 'status-action-disable' : 'status-action-enable'" type="button" @click="toggleImage(image)">{{ image.enabled ? t.disable : t.enable }}</button><button class="icon-button" type="button" @click="openImageEdit(image)">{{ t.edit }}</button><button class="icon-button" type="button" :disabled="image.is_default || !image.enabled" @click="setDefaultImage(image)">{{ image.is_default ? t.defaultImage : t.setDefaultImage }}</button><button class="icon-button danger-button" type="button" @click="removeImage(image.id)">{{ t.remove }}</button></td></tr></tbody></template></BaseTable>
    <BaseDialog :open="imageEditorOpen" :title="`${editingImageID ? t.edit : t.add} ${t.testImagesTitle}`" @close="closeImageEditor">
      <form class="admin-editor-form test-image-editor-form" @submit.prevent="saveImage">
        <div class="test-image-editor-block test-image-editor-basics">
          <FormField class="form-field-image-reference" :label="t.reference"><input v-model="imageForm.reference" placeholder="library/alpine:latest" required></FormField>
          <FormField class="form-field-image-size" :label="t.maxBytes"><input v-model.number="imageForm.max_mib" type="number" min="1" max="4096" step="1" required></FormField>
          <label class="form-field form-field-image-auth"><span>{{ t.authStrategy }} <span class="auth-help-icon" role="img" :aria-label="t.authSecretHint" :title="t.authSecretHint">i</span></span><select v-model="imageForm.auth_strategy"><option v-for="strategy in authStrategies" :key="strategy" :value="strategy">{{ authStrategyLabel(strategy) }}</option></select></label>
        </div>
        <div class="test-image-editor-block test-image-editor-scope">
          <div class="form-field test-image-scope-field">
            <span>{{ t.applicableCategories }}</span>
            <div class="checkbox-list" role="group" :aria-label="t.applicableCategories">
              <label v-for="category in categories" :key="category.id" class="checkbox-list-item"><input v-model="imageForm.category_ids" type="checkbox" :value="category.id"><span>{{ category.slug }} · {{ category.name }}</span></label>
            </div>
            <small>{{ imageForm.category_ids?.length ? t.selectedScopeHint : t.noLimit }}</small>
          </div>
          <div class="form-field test-image-scope-field">
            <span>{{ t.applicableProbeModes }}</span>
            <div class="checkbox-list" role="group" :aria-label="t.applicableProbeModes">
              <label v-for="mode in probeModes" :key="mode" class="checkbox-list-item"><input v-model="imageForm.probe_modes" type="checkbox" :value="mode"><span>{{ mode === 'registry' ? t.registryProbe : mode === 'manifest' ? t.manifestProbe : mode === 'http' ? t.httpProbe : t.dockerPullProbe }}</span></label>
            </div>
            <small>{{ imageForm.probe_modes?.length ? t.selectedScopeHint : t.noLimit }}</small>
          </div>
        </div>
        <div class="test-image-editor-footer">
          <label class="checkbox-field test-image-enabled"><input v-model="imageForm.enabled" type="checkbox"><span>{{ t.enabled }}</span></label>
          <div class="editor-form-actions"><button class="refresh" type="submit">{{ editingImageID ? t.save : t.add }}</button><button class="icon-button" type="button" @click="closeImageEditor">{{ t.cancel }}</button></div>
        </div>
      </form>
    </BaseDialog>
  </template>
  <AdminSettingsManagement v-else-if="section === 'settings'" :token="token" @error="emit('error', $event)" @notice="emit('notice', $event)" />
  <template v-else><section class="panel admin-resource-section"><h2>{{ t.settingsTitle }}</h2><div class="setting-row setting-highlight"><label>{{ t.probeInterval }}<input v-model.number="probeIntervalMinutes" type="number" min="1" max="1440"></label><p>{{ t.probeIntervalHint }}</p></div><section class="setting-row totp-settings"><div class="setting-row-title"><strong>{{ t.totpTitle }}</strong><span class="enabled-state" :class="{ disabled: !totp.enabled }"><i></i>{{ totp.enabled ? t.enabled : t.disabled }}</span></div><p>{{ t.totpDescription }}</p><button class="icon-button" type="button" @click="generateTOTP">{{ t.generateSecret }}</button><label v-if="totpSecret">{{ t.secret }}<input v-model="totpSecret" readonly></label><label v-if="totp.otpauth_uri">{{ t.otpUri }}<input :value="totp.otpauth_uri" readonly></label><label v-if="totpSecret">{{ t.verificationCode }}<input v-model="totpCode" inputmode="numeric" maxlength="6" :placeholder="t.verificationCode"></label><p v-if="totpSecret" class="totp-uri">{{ t.totpHint }}</p><button v-if="totpSecret && !totp.enabled" class="refresh" type="button" @click="enableTOTP">{{ t.enableTotp }}</button><button v-if="totp.enabled" class="icon-button danger-button" type="button" @click="disableTOTP">{{ t.disableTotp }}</button></section><div v-for="entry in settingEntries" :key="entry.key" class="setting-row"><label>{{ entry.key }}<input v-model="entry.value"></label></div><button class="refresh" type="button" @click="saveSettings">{{ t.saveSettings }}</button><pre>{{ JSON.stringify(settings, null, 2) }}</pre></section></template>
</template>
