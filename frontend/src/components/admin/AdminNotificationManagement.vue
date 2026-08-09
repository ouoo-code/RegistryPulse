<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { adminApi, adminRequest } from '../../admin-api'
import { type NotificationChannel } from '../../api'
import { useI18n } from '../../i18n'
import BaseDialog from '../BaseDialog.vue'
import BaseTable from '../BaseTable.vue'
import FormField from '../FormField.vue'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ error: [message: string]; notice: [message: string] }>()
const { t } = useI18n()
const channels = ref<NotificationChannel[]>([])
const dialogOpen = ref(false)
const editing = ref<NotificationChannel | null>(null)
const form = ref({ name: '', type: 'webhook', enabled: true, url: '', token: '', secret: '', host: '', port: '587', username: '', password: '', from: '', to: '' })
const fail = (error: unknown) => emit('error', error instanceof Error ? error.message : t.value.apiError)
function openCreate() { editing.value = null; form.value = { name: '', type: 'webhook', enabled: true, url: '', token: '', secret: '', host: '', port: '587', username: '', password: '', from: '', to: '' }; dialogOpen.value = true }
function openEdit(channel: NotificationChannel) {
  const config = channel.config || {}
  editing.value = channel
  form.value = { name: channel.name, type: channel.type, enabled: channel.enabled !== false, url: String(config.url || ''), token: '', secret: '', host: String(config.host || ''), port: String(config.port || '587'), username: String(config.username || ''), password: '', from: String(config.from || ''), to: String(config.to || '') }
  dialogOpen.value = true
}
function configForForm() {
  if (form.value.type === 'gotify') return { url: form.value.url, token: form.value.token || '***' }
  if (form.value.type === 'webhook') return { url: form.value.url, secret: form.value.secret || '***' }
  return { host: form.value.host, port: form.value.port, username: form.value.username, password: form.value.password || '***', from: form.value.from, to: form.value.to }
}
async function save() {
  try {
    const input = { name: form.value.name.trim(), type: form.value.type, enabled: form.value.enabled, config: configForForm() }
    if (!input.name) throw new Error(t.value.name)
    if (editing.value) await adminApi.updateNotification(props.token, editing.value.id, input)
    else await adminApi.createNotification(props.token, input)
    dialogOpen.value = false; emit('notice', t.value.saveSuccess); await refresh()
  } catch (error) { fail(error) }
}
async function remove(channel: NotificationChannel) {
  if (!confirm(t.value.confirmDelete)) return
  try { await adminApi.deleteNotification(props.token, channel.id); emit('notice', t.value.deleteSuccess); await refresh() } catch (error) { fail(error) }
}
async function test(channel: NotificationChannel) { try { await adminRequest<void>(props.token, `/admin/notifications/${channel.id}/test`, { method: 'POST' }); emit('notice', t.value.probeQueued) } catch (error) { fail(error) } }
async function refresh() { try { channels.value = await adminApi.notifications(props.token) } catch (error) { fail(error) } }
onMounted(refresh)
defineExpose({ refresh })
</script>

<template>
  <section class="panel admin-resource-section notification-management"><div class="section-heading"><h2>{{ t.notificationsTitle }}</h2><button class="icon-button" type="button" @click="openCreate">{{ t.add }}</button></div>
    <BaseTable :empty="!channels.length ? t.noResults : ''"><template #head><thead><tr><th>{{ t.name }}</th><th>{{ t.type }}</th><th>{{ t.enabled }}</th><th>{{ t.actions }}</th></tr></thead></template><template #body><tbody><tr v-for="channel in channels" :key="channel.id"><td>{{ channel.name }}</td><td>{{ channel.type }}</td><td>{{ channel.enabled !== false ? t.enabled : t.disabled }}</td><td class="table-actions"><button class="icon-button" type="button" @click="test(channel)">{{ t.sendTest }}</button><button class="icon-button" type="button" @click="openEdit(channel)">{{ t.edit }}</button><button class="icon-button" type="button" @click="remove(channel)">{{ t.remove }}</button></td></tr></tbody></template></BaseTable>
  </section>
  <BaseDialog :open="dialogOpen" :title="editing ? t.edit : t.add" @close="dialogOpen = false"><form class="notification-form" autocomplete="off" @submit.prevent="save"><FormField :label="t.name"><input v-model="form.name" autocomplete="off" required></FormField><FormField :label="t.type"><select v-model="form.type" autocomplete="off"><option value="webhook">Webhook</option><option value="gotify">Gotify</option><option value="smtp">SMTP Email</option></select></FormField><FormField v-if="form.type !== 'smtp'" :label="t.url"><input v-model="form.url" type="url" autocomplete="off" required></FormField><FormField v-if="form.type === 'gotify'" label="Token"><input v-model="form.token" type="password" autocomplete="new-password" :placeholder="editing ? '******' : ''" :required="!editing"></FormField><FormField v-if="form.type === 'webhook'" label="Secret"><input v-model="form.secret" type="password" autocomplete="new-password" :placeholder="editing ? '******' : ''"></FormField><template v-if="form.type === 'smtp'"><FormField label="Host"><input v-model="form.host" autocomplete="off" required></FormField><FormField label="Port"><input v-model="form.port" autocomplete="off" required></FormField><FormField label="Username"><input v-model="form.username" autocomplete="off"></FormField><FormField label="Password"><input v-model="form.password" type="password" autocomplete="new-password" :placeholder="editing ? '******' : ''"></FormField><FormField label="From"><input v-model="form.from" type="email" autocomplete="off" required></FormField><FormField label="To"><input v-model="form.to" type="email" autocomplete="off" required></FormField></template><label class="checkbox-field"><input v-model="form.enabled" type="checkbox">{{ t.enabled }}</label><div class="editor-form-actions"><button class="icon-button" type="button" @click="dialogOpen = false">{{ t.cancel }}</button><button class="icon-button" type="submit">{{ t.save }}</button></div></form></BaseDialog>
</template>
