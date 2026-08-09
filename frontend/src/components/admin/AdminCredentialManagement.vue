<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { adminApi } from '../../admin-api'
import { type Category, type CredentialProfile, type CredentialProfileInput, type Source } from '../../api'
import { useI18n } from '../../i18n'
import BaseTable from '../BaseTable.vue'
import FormField from '../FormField.vue'
import { formatDateTime } from '../../time'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ notice: [message: string]; error: [message: string] }>()
const { t } = useI18n()
const profiles = ref<CredentialProfile[]>([])
const sources = ref<Source[]>([])
const categories = ref<Category[]>([])
const editingID = ref('')
const form = ref<CredentialProfileInput>(emptyForm())

function emptyForm(): CredentialProfileInput {
  return { name: '', auth_type: 'basic', username: '', secret: '', source_id: '', registry_host: '', category_id: '', enabled: true }
}
function reset() { editingID.value = ''; form.value = emptyForm() }
async function refresh() {
  try {
    const [profileList, sourceList, categoryList] = await Promise.all([adminApi.credentialProfiles(props.token), adminApi.sources(props.token), adminApi.categories(props.token)])
    profiles.value = profileList
    sources.value = sourceList
    categories.value = categoryList
  } catch (error) { emit('error', error instanceof Error ? error.message : t.value.apiError) }
}
function edit(profile: CredentialProfile) {
  editingID.value = profile.id
  form.value = { id: profile.id, name: profile.name, auth_type: profile.auth_type, username: profile.username || '', secret: '', source_id: profile.source_id || '', registry_host: profile.registry_host || '', category_id: profile.category_id || '', enabled: profile.enabled }
}
async function save() {
  try {
    const payload = { ...form.value, name: form.value.name.trim(), username: form.value.username?.trim(), registry_host: form.value.registry_host?.trim() }
    if (editingID.value) await adminApi.updateCredentialProfile(props.token, editingID.value, payload)
    else await adminApi.createCredentialProfile(props.token, payload)
    emit('notice', t.value.saveSuccess)
    reset()
    await refresh()
  } catch (error) { emit('error', error instanceof Error ? error.message : t.value.credentialSaveError) }
}
async function remove(profile: CredentialProfile) {
  if (!confirm(t.value.confirmDelete)) return
  try { await adminApi.deleteCredentialProfile(props.token, profile.id); emit('notice', t.value.deleteSuccess); await refresh() }
  catch (error) { emit('error', error instanceof Error ? error.message : t.value.credentialDeleteError) }
}
function sourceName(id?: string) { return sources.value.find(source => source.id === id)?.name || id || t.value.noLimit }
function categoryName(id?: string) { return categories.value.find(category => category.id === id)?.slug || id || t.value.noLimit }
onMounted(refresh)
defineExpose({ refresh })
</script>

<template>
  <section class="panel admin-resource-section credential-management">
    <div class="section-heading admin-categories-heading"><span class="eyebrow">{{ t.adminCredentials }}</span><button v-if="editingID" class="icon-button" type="button" @click="reset">{{ t.cancel }}</button></div>
    <p class="settings-description">{{ t.credentialSelectorHint }}</p>
    <form class="admin-editor-form credential-editor-form" @submit.prevent="save">
      <FormField :label="t.credentialName"><input v-model="form.name" required></FormField>
      <FormField :label="t.credentialType"><select v-model="form.auth_type"><option value="basic">{{ t.credentialBasic }}</option><option value="bearer">{{ t.credentialBearer }}</option><option value="token">{{ t.credentialToken }}</option></select></FormField>
      <FormField :label="t.username"><input v-model="form.username" autocomplete="off"></FormField>
      <FormField :label="t.credentialSecret"><input v-model="form.secret" type="password" autocomplete="new-password" :placeholder="editingID ? t.credentialSecretPlaceholder : ''" :required="!editingID"></FormField>
      <FormField :label="t.credentialSource"><select v-model="form.source_id"><option value="">{{ t.noLimit }}</option><option v-for="source in sources" :key="source.id" :value="source.id">{{ source.name }} · {{ source.registry_host || source.base_url }}</option></select></FormField>
      <FormField :label="t.credentialHost"><input v-model="form.registry_host" placeholder="registry.example.com"></FormField>
      <FormField :label="t.credentialCategory"><select v-model="form.category_id"><option value="">{{ t.noLimit }}</option><option v-for="category in categories" :key="category.id" :value="category.id">{{ category.slug }} · {{ category.name }}</option></select></FormField>
      <label class="checkbox-field credential-enabled"><input v-model="form.enabled" type="checkbox"><span>{{ t.enabled }}</span></label>
      <div class="editor-form-actions"><button class="refresh" type="submit">{{ editingID ? t.save : t.add }}</button><button v-if="editingID" class="icon-button" type="button" @click="reset">{{ t.cancel }}</button></div>
    </form>
  </section>
  <BaseTable class="credential-table" :empty="!profiles.length ? t.noResults : ''"><template #head><thead><tr><th>{{ t.credentialName }}</th><th>{{ t.credentialType }}</th><th>{{ t.credentialSelector }}</th><th>{{ t.credentialSecret }}</th><th>{{ t.status }}</th><th>{{ t.createdAt }}</th><th>{{ t.actions }}</th></tr></thead></template><template #body><tbody><tr v-for="profile in profiles" :key="profile.id"><td>{{ profile.name }}</td><td>{{ profile.auth_type }}</td><td>{{ profile.source_id ? sourceName(profile.source_id) : profile.registry_host || categoryName(profile.category_id) }}</td><td>{{ profile.has_secret ? (profile.secret_masked || '****') : t.credentialNoSecret }}</td><td><span class="enabled-state" :class="{ disabled: !profile.enabled }"><i></i>{{ profile.enabled ? t.enabled : t.disabled }}</span></td><td>{{ formatDateTime(profile.created_at) || t.unknown }}</td><td class="table-actions"><button class="icon-button" type="button" @click="edit(profile)">{{ t.edit }}</button><button class="icon-button danger-button" type="button" @click="remove(profile)">{{ t.delete }}</button></td></tr></tbody></template></BaseTable>
</template>
