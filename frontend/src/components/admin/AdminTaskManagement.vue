<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { adminApi, adminRequest } from '../../admin-api'
import { type AdminTask, type ProbeNode, type Source } from '../../api'
import { useI18n } from '../../i18n'
import BaseTable from '../BaseTable.vue'
import SortIndicator from '../SortIndicator.vue'
import { formatDateTime } from '../../time'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ error: [message: string]; notice: [message: string] }>()
const { t, locale } = useI18n()
const tasks = ref<AdminTask[]>([])
const sources = ref<Source[]>([])
const nodes = ref<ProbeNode[]>([])
const taskSort = ref<'status' | 'created_at'>('created_at')
const taskAscending = ref(false)
const displayedTasks = computed(() => [...tasks.value].sort((left, right) => {
  const result = taskSort.value === 'status' ? left.status.localeCompare(right.status) : String(left.created_at || '').localeCompare(String(right.created_at || ''))
  return taskAscending.value ? result : -result
}))
const sourceByID = computed(() => new Map(sources.value.map(source => [source.id, source])))
const nodeByID = computed(() => new Map(nodes.value.map(node => [node.id, node])))
function sortTasks(key: 'status' | 'created_at') { if (taskSort.value === key) taskAscending.value = !taskAscending.value; else { taskSort.value = key; taskAscending.value = true } }
function sortActive(key: 'status' | 'created_at') { return taskSort.value === key }
function runtime(task: AdminTask): string { if (!task.started_at) return '—'; const startDate = new Date(task.started_at); if (Number.isNaN(startDate.getTime()) || startDate.getUTCFullYear() <= 1970) return '—'; const endDate = task.finished_at ? new Date(task.finished_at) : new Date(); const elapsedMs = endDate.getTime() - startDate.getTime(); if (Number.isNaN(elapsedMs) || elapsedMs < 0) return '—'; if (elapsedMs < 1000) return '<1s'; const seconds = Math.floor(elapsedMs / 1000); return seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s` }
function sourceName(task: AdminTask) { return sourceByID.value.get(task.source_id)?.name || task.source_id }
function sourceURL(task: AdminTask) { return sourceByID.value.get(task.source_id)?.base_url || '—' }
function nodeName(task: AdminTask) { return task.probe_node_id ? (nodeByID.value.get(task.probe_node_id)?.name || task.probe_node_id) : t.value.unknown }
async function refresh() { try { [tasks.value, sources.value, nodes.value] = await Promise.all([adminApi.tasks(props.token), adminApi.sources(props.token), adminApi.nodes(props.token)]) } catch (error) { emit('error', error instanceof Error ? error.message : t.value.apiError) } }
async function action(id: string, value: 'cancel' | 'retry') { try { await adminRequest<void>(props.token, `/admin/tasks/${id}`, { method: 'PUT', body: JSON.stringify({ action: value }) }); emit('notice', t.value.saveSuccess); await refresh() } catch (error) { emit('error', error instanceof Error ? error.message : t.value.taskUpdateError) } }
async function clearTasks() { if (!confirm(t.value.confirmDelete)) return; try { const result = await adminApi.clearTasks(props.token); emit('notice', `${t.value.saveSuccess} (${result.deleted})`); await refresh() } catch (error) { emit('error', error instanceof Error ? error.message : t.value.taskUpdateError) } }
onMounted(refresh)
defineExpose({ refresh })
</script>

<template>
  <section class="panel task-actions-bar"><div><h2>{{ t.probeTasks }}</h2><p>{{ locale === 'zh' ? '仅清理已完成、失败和已取消的任务，执行中的任务会保留。' : 'Only completed, failed, and cancelled tasks are cleared; active tasks are kept.' }}</p></div><button class="icon-button" type="button" @click="clearTasks">{{ locale === 'zh' ? '清空数据' : 'Clear data' }}</button></section>
  <BaseTable class="task-table" :empty="!tasks.length ? t.noResults : ''"><template #head><thead><tr><th>{{ t.type }}</th><th class="sortable-header" :class="{ active: sortActive('status') }" :aria-sort="sortActive('status') ? (taskAscending ? 'ascending' : 'descending') : 'none'" @click="sortTasks('status')">{{ t.status }} <SortIndicator :active="sortActive('status')" :ascending="taskAscending" /></th><th>{{ t.registry }}</th><th class="task-url-column">{{ t.url }}</th><th>{{ t.probeNodes }}</th><th>{{ t.runtimeDuration }}</th><th class="sortable-header" :class="{ active: sortActive('created_at') }" :aria-sort="sortActive('created_at') ? (taskAscending ? 'ascending' : 'descending') : 'none'" @click="sortTasks('created_at')">{{ t.createdAt }} <SortIndicator :active="sortActive('created_at')" :ascending="taskAscending" /></th><th>{{ t.actions }}</th></tr></thead></template><template #body><tbody><tr v-for="task in displayedTasks" :key="task.id"><td>{{ task.task_type || 'oci_probe' }}</td><td>{{ task.status }}</td><td>{{ sourceName(task) }}</td><td class="task-url-column">{{ sourceURL(task) }}</td><td>{{ nodeName(task) }}</td><td>{{ runtime(task) }}</td><td>{{ formatDateTime(task.created_at) || t.unknown }}</td><td class="table-actions"><button v-if="!['completed','failed','cancelled'].includes(task.status)" class="icon-button" type="button" @click="action(task.id, 'cancel')">{{ t.cancel }}</button><button v-if="['failed','cancelled'].includes(task.status)" class="icon-button" type="button" @click="action(task.id, 'retry')">{{ t.retry }}</button></td></tr></tbody></template></BaseTable>
</template>
