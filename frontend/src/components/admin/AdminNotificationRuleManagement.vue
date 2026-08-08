<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { adminApi } from '../../admin-api'
import { type NotificationChannel, type NotificationRule } from '../../api'
import { useI18n } from '../../i18n'
import BaseDialog from '../BaseDialog.vue'
import BaseTable from '../BaseTable.vue'
import FormField from '../FormField.vue'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ error: [message: string]; notice: [message: string] }>()
const { t, locale } = useI18n()
const rules = ref<NotificationRule[]>([])
const channels = ref<NotificationChannel[]>([])
const dialogOpen = ref(false)
const editing = ref<NotificationRule | null>(null)
const form = ref({ channel_id: '', event_type: 'incident_opened', enabled: true, cooldown_seconds: 300, aggregation_seconds: 0, template: '' })
const eventTypes = ['incident_opened', 'incident_resolved', 'degraded_detected', 'certificate_expiring', 'certificate_expiring_critical']
const channelName = (id: string) => channels.value.find(channel => channel.id === id)?.name || id
const fail = (error: unknown) => emit('error', error instanceof Error ? error.message : t.value.apiError)
function openCreate() { editing.value = null; form.value = { channel_id: channels.value[0]?.id || '', event_type: 'incident_opened', enabled: true, cooldown_seconds: 300, aggregation_seconds: 0, template: '' }; dialogOpen.value = true }
function openEdit(rule: NotificationRule) { editing.value = rule; form.value = { channel_id: rule.channel_id, event_type: rule.event_type, enabled: rule.enabled !== false, cooldown_seconds: rule.cooldown_seconds, aggregation_seconds: rule.aggregation_seconds, template: rule.template || '' }; dialogOpen.value = true }
async function save() { try { if (!form.value.channel_id) throw new Error(t.value.channel); if (editing.value) await adminApi.updateNotificationRule(props.token, editing.value.id, form.value); else await adminApi.createNotificationRule(props.token, form.value); dialogOpen.value = false; emit('notice', t.value.saveSuccess); await refresh() } catch (error) { fail(error) } }
async function remove(rule: NotificationRule) { if (!confirm(t.value.confirmDelete)) return; try { await adminApi.deleteNotificationRule(props.token, rule.id); emit('notice', t.value.deleteSuccess); await refresh() } catch (error) { fail(error) } }
async function refresh() { try { [channels.value, rules.value] = await Promise.all([adminApi.notifications(props.token), adminApi.notificationRules(props.token)]) } catch (error) { fail(error) } }
onMounted(refresh)
defineExpose({ refresh })
</script>

<template>
  <section class="panel admin-resource-section notification-rule-management"><div class="section-heading"><h2>{{ t.notificationRulesTitle }}</h2><button class="icon-button" type="button" :disabled="!channels.length" @click="openCreate">{{ t.add }}</button></div><BaseTable :empty="!rules.length ? t.noResults : ''"><template #head><thead><tr><th>{{ t.event }}</th><th>{{ t.channel }}</th><th>{{ t.cooldown }}</th><th>{{ t.aggregation }}</th><th>{{ t.enabled }}</th><th>{{ t.actions }}</th></tr></thead></template><template #body><tbody><tr v-for="rule in rules" :key="rule.id"><td>{{ rule.event_type }}</td><td>{{ channelName(rule.channel_id) }}</td><td>{{ rule.cooldown_seconds }}s</td><td>{{ rule.aggregation_seconds }}s</td><td>{{ rule.enabled !== false ? t.enabled : t.disabled }}</td><td class="table-actions"><button class="icon-button" type="button" @click="openEdit(rule)">{{ t.edit }}</button><button class="icon-button" type="button" @click="remove(rule)">{{ t.remove }}</button></td></tr></tbody></template></BaseTable></section>
  <BaseDialog :open="dialogOpen" :title="editing ? t.edit : t.add" @close="dialogOpen = false"><form class="notification-form" @submit.prevent="save"><FormField :label="t.channel"><select v-model="form.channel_id" required><option v-for="channel in channels" :key="channel.id" :value="channel.id">{{ channel.name }}</option></select></FormField><FormField :label="t.event"><select v-model="form.event_type"><option v-for="eventType in eventTypes" :key="eventType" :value="eventType">{{ eventType }}</option></select></FormField><FormField :label="t.cooldown"><input v-model.number="form.cooldown_seconds" type="number" min="0" max="86400" step="1"></FormField><FormField :label="t.aggregation"><input v-model.number="form.aggregation_seconds" type="number" min="0" max="86400" step="1"></FormField><div class="notification-template-help"><strong>{{ locale === 'zh' ? '模板使用说明' : 'Template guide' }}</strong><p>{{ locale === 'zh' ? '模板用于生成通知正文。变量会在发送时替换为当前事件的数据；留空则使用系统默认消息。' : 'The template generates the notification body. Variables are replaced when the event is sent; leave it empty to use the default message.' }}</p><ul><li><code>{source_name}</code>：{{ locale === 'zh' ? '镜像源名称' : 'registry source name' }}</li><li><code>{event}</code>：{{ locale === 'zh' ? '事件类型' : 'event type' }}</li><li><code>{message}</code>：{{ locale === 'zh' ? '系统生成的事件消息' : 'system event message' }}</li><li><code>{status}</code>：{{ locale === 'zh' ? '当前状态' : 'current status' }}</li></ul><pre>{{ locale === 'zh' ? '镜像源：{source_name}\n事件：{event}\n状态：{status}\n详情：{message}' : 'Source: {source_name}\nEvent: {event}\nStatus: {status}\nDetails: {message}' }}</pre></div><FormField :label="t.description" wide><textarea v-model="form.template" rows="5" placeholder="{source_name} {event} {message} {status}"></textarea></FormField><label class="checkbox-field"><input v-model="form.enabled" type="checkbox">{{ t.enabled }}</label><div class="editor-form-actions"><button class="icon-button" type="button" @click="dialogOpen = false">{{ t.cancel }}</button><button class="icon-button" type="submit">{{ t.save }}</button></div></form></BaseDialog>
</template>
