<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { adminApi, adminRequest } from '../../admin-api'
import { type AdminRole, type AdminRoleInput, type AdminSettings, type AdminUser, type AdminUserInput, type Category, type NotificationChannel, type NotificationRule, type ProbeNode, type TestImage, type TestImageInput } from '../../api'
import { useI18n } from '../../i18n'
import BaseDialog from '../BaseDialog.vue'
import BaseTable from '../BaseTable.vue'
import FormField from '../FormField.vue'
import { formatDateTime } from '../../time'
import AdminNotificationManagement from './AdminNotificationManagement.vue'
import AdminNotificationRuleManagement from './AdminNotificationRuleManagement.vue'
import AdminSettingsManagement from './AdminSettingsManagement.vue'
import { useAccessCopy } from '../../access-copy'

const props = defineProps<{ token: string; section: string }>()
const emit = defineEmits<{ error: [message: string]; notice: [message: string] }>()
const { t } = useI18n()
const accessCopy = useAccessCopy()
const nodes = ref<ProbeNode[]>([])
const users = ref<AdminUser[]>([])
const roles = ref<AdminRole[]>([])
const userEditorOpen = ref(false)
const roleEditorOpen = ref(false)
const editingUserID = ref<string | null>(null)
const editingRoleName = ref<string | null>(null)
const userForm = ref<AdminUserInput>({ username: '', password: '', role: 'viewer', active: true })
const roleForm = ref<AdminRoleInput>({ name: '', permissions: [] })
const permissionOptions = ['source.read', 'source.write', 'probe.read', 'probe.write', 'incident.read', 'settings.read', 'settings.write', 'audit.read', 'agent.manage']
const notifications = ref<NotificationChannel[]>([])
const notificationRules = ref<NotificationRule[]>([])
const testImages = ref<TestImage[]>([])
const categories = ref<Category[]>([])
const settings = ref<AdminSettings>({})
const settingEntries = ref<Array<{ key: string; value: string }>>([])
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
    if (props.section === 'users') {
      const [userList, roleList] = await Promise.all([adminApi.users(props.token), adminApi.roles(props.token)])
      users.value = userList
      roles.value = roleList
    }
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
      const rawInterval = settings.value.probe_interval_minutes
      const intervalValue = typeof rawInterval === 'object' && rawInterval !== null && 'value' in rawInterval ? rawInterval.value : rawInterval
      probeIntervalMinutes.value = Number(intervalValue) || 5
      settingEntries.value = Object.entries(settings.value).filter(([key]) => key !== 'probe_interval_minutes').map(([key, value]) => ({ key, value: typeof value === 'string' ? value : JSON.stringify(value) }))
    }
  } catch (error) { fail(error, t.value.apiError) }
}
function parseList(value: string[] | string | undefined) {
  return Array.isArray(value) ? value : (value || '').split(',').map(item => item.trim()).filter(Boolean)
}
function openUserCreate() { editingUserID.value = null; userForm.value = { username: '', password: '', role: 'viewer', active: true }; userEditorOpen.value = true }
function openUserEdit(user: AdminUser) { editingUserID.value = user.id; userForm.value = { username: user.username, password: '', role: parseList(user.roles)[0] || 'viewer', active: user.active !== false }; userEditorOpen.value = true }
function openRoleCreate() { editingRoleName.value = null; roleForm.value = { name: '', permissions: [] }; roleEditorOpen.value = true }
function openRoleEdit(role: AdminRole) { editingRoleName.value = role.name; roleForm.value = { name: role.name, permissions: parseList(role.permissions) }; roleEditorOpen.value = true }
async function saveUser() {
  const payload = { ...userForm.value, username: userForm.value.username.trim(), password: userForm.value.password || undefined }
  try { if (editingUserID.value) await adminApi.updateUser(props.token, editingUserID.value, payload); else await adminApi.createUser(props.token, payload); userEditorOpen.value = false; emit('notice', t.value.saveSuccess); await refresh() } catch (error) { fail(error, accessCopy.value.userSaveError) }
}
async function toggleUser(user: AdminUser) {
  try { await adminApi.updateUser(props.token, user.id, { username: user.username, role: parseList(user.roles)[0] || 'viewer', active: user.active === false }); emit('notice', t.value.saveSuccess); await refresh() } catch (error) { fail(error, accessCopy.value.userSaveError) }
}
async function saveRole() {
  const payload = { name: roleForm.value.name.trim(), permissions: [...roleForm.value.permissions] }
  try { if (editingRoleName.value) await adminApi.updateRole(props.token, editingRoleName.value, payload); else await adminApi.createRole(props.token, payload); roleEditorOpen.value = false; emit('notice', t.value.saveSuccess); await refresh() } catch (error) { fail(error, accessCopy.value.roleSaveError) }
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
async function deleteUser(user: AdminUser) {
  if (user.active !== false || !confirm(t.value.confirmDelete)) return
  try { await adminApi.deleteUser(props.token, user.id); emit('notice', t.value.deleteSuccess); await refresh() } catch (error) { fail(error, accessCopy.value.userSaveError) }
}
async function resetUserTotp(user: AdminUser) {
  if (!confirm(t.value.confirmDelete)) return
  try { await adminApi.resetUserTotp(props.token, user.id); emit('notice', t.value.saveSuccess); await refresh() } catch (error) { fail(error, accessCopy.value.userSaveError) }
}
async function deleteRole(role: AdminRole) {
  if (role.name === 'admin' || role.name === 'operator' || role.name === 'viewer' || (role.user_count || 0) > 0 || !confirm(t.value.confirmDelete)) return
  try { await adminApi.deleteRole(props.token, role.name); emit('notice', t.value.deleteSuccess); await refresh() } catch (error) { fail(error, accessCopy.value.roleSaveError) }
}
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
  <template v-else-if="section === 'users'">
    <section class="panel admin-resource-section access-management"><div class="section-heading"><div><span class="eyebrow">{{ t.usersTitle }}</span><h2>{{ accessCopy.userManagement }}</h2></div><button class="refresh" type="button" @click="openUserCreate">{{ t.add }}</button></div><p class="settings-description">{{ accessCopy.userManagementHint }}</p></section>
    <BaseTable :empty="!users.length ? t.noResults : ''"><template #head><thead><tr><th>{{ t.username }}</th><th>{{ t.adminRoles }}</th><th>{{ t.status }}</th><th>{{ t.actions }}</th></tr></thead></template><template #body><tbody><tr v-for="user in users" :key="user.id"><td>{{ user.username }}</td><td>{{ parseList(user.roles).join(', ') || accessCopy.noRole }}</td><td><span class="enabled-state" :class="{ disabled: user.active === false }"><i></i>{{ user.active === false ? t.disabled : t.enabled }}</span></td><td class="table-actions"><button class="icon-button" type="button" @click="openUserEdit(user)">{{ t.edit }}</button><button class="icon-button" type="button" @click="toggleUser(user)">{{ user.active === false ? t.enable : t.disable }}</button><button class="icon-button" type="button" :disabled="!user.totp_enabled" @click="resetUserTotp(user)">{{ t.resetTotp }}</button><button class="icon-button danger-button" type="button" :disabled="user.active !== false" @click="deleteUser(user)">{{ t.delete }}</button></td></tr></tbody></template></BaseTable>
    <BaseDialog :open="userEditorOpen" :title="editingUserID ? accessCopy.editUser : accessCopy.addUser" size="small" @close="userEditorOpen = false"><form class="notification-form access-editor-form" @submit.prevent="saveUser"><FormField :label="t.username"><input v-model="userForm.username" autocomplete="username" required></FormField><FormField :label="t.password"><input v-model="userForm.password" type="password" autocomplete="new-password" :placeholder="editingUserID ? accessCopy.passwordKeepHint : ''" :required="!editingUserID"></FormField><FormField :label="t.adminRoles"><select v-model="userForm.role"><option v-for="role in roles" :key="role.name" :value="role.name">{{ role.name }}</option></select></FormField><label class="checkbox-field"><input v-model="userForm.active" type="checkbox">{{ t.enabled }}</label><div class="editor-form-actions"><button class="icon-button" type="button" @click="userEditorOpen = false">{{ t.cancel }}</button><button class="icon-button" type="submit">{{ t.save }}</button></div></form></BaseDialog>
  </template>
  <template v-else-if="section === 'roles'">
    <section class="panel admin-resource-section access-management"><div class="section-heading"><div><span class="eyebrow">{{ t.rolesTitle }}</span><h2>{{ accessCopy.roleManagement }}</h2></div><button class="refresh" type="button" @click="openRoleCreate">{{ t.add }}</button></div><p class="settings-description">{{ accessCopy.roleManagementHint }}</p></section>
    <BaseTable :empty="!roles.length ? t.noResults : ''"><template #head><thead><tr><th>{{ t.name }}</th><th>{{ t.permissions }}</th><th>{{ t.associatedUsers }}</th><th>{{ t.actions }}</th></tr></thead></template><template #body><tbody><tr v-for="role in roles" :key="role.name"><td>{{ role.name }}</td><td>{{ role.name === 'admin' ? accessCopy.allPermissions : parseList(role.permissions).join(', ') || accessCopy.noPermissions }}</td><td>{{ role.user_count || 0 }}</td><td class="table-actions"><button class="icon-button" type="button" :disabled="role.name === 'admin'" @click="openRoleEdit(role)">{{ t.edit }}</button><button class="icon-button danger-button" type="button" :disabled="role.name === 'admin' || role.name === 'operator' || role.name === 'viewer' || (role.user_count || 0) > 0" @click="deleteRole(role)">{{ t.deleteRole }}</button></td></tr></tbody></template></BaseTable>
    <BaseDialog :open="roleEditorOpen" :title="editingRoleName ? accessCopy.editRole : accessCopy.addRole" size="small" @close="roleEditorOpen = false"><form class="notification-form access-editor-form" @submit.prevent="saveRole"><FormField :label="t.name"><input v-model="roleForm.name" :readonly="!!editingRoleName" required></FormField><div class="permission-list"><strong>{{ t.permissions }}</strong><label v-for="permission in permissionOptions" :key="permission" class="checkbox-field"><input v-model="roleForm.permissions" type="checkbox" :value="permission">{{ permission }}</label></div><div class="editor-form-actions"><button class="icon-button" type="button" @click="roleEditorOpen = false">{{ t.cancel }}</button><button class="icon-button" type="submit">{{ t.save }}</button></div></form></BaseDialog>
  </template>
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
  <template v-else></template>
</template>
