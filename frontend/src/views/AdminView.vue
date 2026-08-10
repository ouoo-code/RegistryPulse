<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { api } from '../api'
import { adminHeaders } from '../admin-api'
import { useI18n } from '../i18n'
import AdminCategoryManagement from '../components/admin/AdminCategoryManagement.vue'
import AdminCredentialManagement from '../components/admin/AdminCredentialManagement.vue'
import AdminHistoryManagement from '../components/admin/AdminHistoryManagement.vue'
import AdminLayout, { type AdminSection } from '../components/admin/AdminLayout.vue'
import AdminLogin from '../components/admin/AdminLogin.vue'
import AdminMiscSections from '../components/admin/AdminMiscSections.vue'
import AdminSourceManagement from '../components/admin/AdminSourceManagement.vue'
import AdminTaskManagement from '../components/admin/AdminTaskManagement.vue'
import './admin.css'

const { t } = useI18n()
const token = ref(sessionStorage.getItem('admin-token') || '')
const activeSection = ref('sources')
const notice = ref('')
const error = ref('')
let feedbackTimer: number | undefined
const sourceManagement = ref<{ refresh: () => Promise<void>; openCreate: () => void } | null>(null)
const categoryManagement = ref<{ refresh: () => Promise<void> } | null>(null)
const taskManagement = ref<{ refresh: () => Promise<void> } | null>(null)
const historyManagement = ref<{ refresh: () => Promise<void> } | null>(null)
const miscSections = ref<{ refresh: () => Promise<void> } | null>(null)
const credentialManagement = ref<{ refresh: () => Promise<void> } | null>(null)
const adminSections = [
  { key: 'sources', label: 'adminSources' }, { key: 'categories', label: 'adminCategories' }, { key: 'tasks', label: 'adminTasks' }, { key: 'history', label: 'history' },
  { key: 'nodes', label: 'adminNodes' },
  { key: 'notifications', label: 'adminNotifications' }, { key: 'notification-rules', label: 'adminNotificationRules' },
  { key: 'users', label: 'adminUsers' }, { key: 'roles', label: 'adminRoles' },
  { key: 'test-images', label: 'adminTestImages' }, { key: 'credentials', label: 'adminCredentials' }, { key: 'settings', label: 'adminSettings' },
] as const satisfies readonly AdminSection[]
const visibleAdminSections = adminSections.filter(item => item.key !== 'users' && item.key !== 'roles')

async function signIn(credentials: { username: string; password: string; totp_code?: string }) {
  try {
    const result = await api<{ token: string }>('/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(credentials) })
    token.value = result.token
    sessionStorage.setItem('admin-token', result.token)
    error.value = ''
  } catch { handleError(t.value.apiError) }
}

async function signOut() {
  try { if (token.value) await api('/auth/logout', { method: 'POST', headers: adminHeaders(token.value) }) }
  finally { token.value = ''; sessionStorage.removeItem('admin-token'); activeSection.value = 'sources' }
}

async function changePassword(payload: { currentPassword: string; newPassword: string }) {
  try { await api('/auth/change-password', { method: 'POST', headers: adminHeaders(token.value), body: JSON.stringify({ current_password: payload.currentPassword, new_password: payload.newPassword }) }); handleNotice(t.value.savedPasswordHint); await signOut() } catch { handleError(t.value.apiError) }
}

function clearFeedbackTimer() {
  if (feedbackTimer !== undefined) window.clearTimeout(feedbackTimer)
  feedbackTimer = undefined
}
function scheduleFeedbackClear() {
  clearFeedbackTimer()
  feedbackTimer = window.setTimeout(() => {
    notice.value = ''
    error.value = ''
    feedbackTimer = undefined
  }, 5000)
}
function setSection(section: string) { activeSection.value = section; notice.value = ''; error.value = ''; clearFeedbackTimer() }
async function refreshActive() {
  if (activeSection.value === 'sources') await sourceManagement.value?.refresh()
  if (activeSection.value === 'categories') await categoryManagement.value?.refresh()
  if (activeSection.value === 'tasks') await taskManagement.value?.refresh()
  if (activeSection.value === 'history') await historyManagement.value?.refresh()
  if (activeSection.value === 'credentials') await credentialManagement.value?.refresh()
  if (!['sources', 'categories', 'tasks', 'history', 'credentials'].includes(activeSection.value)) await miscSections.value?.refresh()
}
function handleError(message: string) { error.value = message; notice.value = ''; scheduleFeedbackClear() }
function handleNotice(message: string) { notice.value = message; error.value = ''; scheduleFeedbackClear() }

onMounted(() => { if (!token.value) sessionStorage.removeItem('admin-token') })
onUnmounted(clearFeedbackTimer)
</script>

<template>
  <header v-if="!token" class="admin-standalone-header">
    <div class="admin-brand"><span class="brand-mark" aria-hidden="true"><i></i></span><span class="admin-brand-copy"><strong>Registry Pulse</strong><small>{{ t.brand }}</small></span></div>
    <RouterLink class="admin-login-back" to="/">{{ t.backToSite }}</RouterLink>
  </header>
  <AdminLogin v-if="!token" :error="error" @submit="signIn" />
  <AdminLayout v-else :token="token" :active-section="activeSection" :sections="visibleAdminSections" @section="setSection" @access="setSection" @refresh="refreshActive" @add="sourceManagement?.openCreate()" @sign-out="signOut" @change-password="changePassword" @error="handleError" @notice="handleNotice">
    <div v-if="error" class="alert">{{ error }}</div><div v-if="notice" class="success">{{ notice }}</div>
    <AdminSourceManagement v-if="activeSection === 'sources'" ref="sourceManagement" :token="token" @error="handleError" @notice="handleNotice" />
    <AdminCategoryManagement v-else-if="activeSection === 'categories'" ref="categoryManagement" :token="token" @error="handleError" @notice="handleNotice" />
    <AdminTaskManagement v-else-if="activeSection === 'tasks'" ref="taskManagement" :token="token" @error="handleError" @notice="handleNotice" />
    <AdminHistoryManagement v-else-if="activeSection === 'history'" ref="historyManagement" :token="token" @error="handleError" />
    <AdminCredentialManagement v-else-if="activeSection === 'credentials'" ref="credentialManagement" :token="token" @error="handleError" @notice="handleNotice" />
    <AdminMiscSections v-else ref="miscSections" :token="token" :section="activeSection" @error="handleError" @notice="handleNotice" />
  </AdminLayout>
</template>
