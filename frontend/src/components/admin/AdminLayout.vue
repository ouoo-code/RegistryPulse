<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from '../../i18n'
import AdminAccountMenu from './AdminAccountMenu.vue'

export type AdminSection = { key: string; label: string }
const props = defineProps<{ activeSection: string; sections: readonly AdminSection[]; token: string }>()
const emit = defineEmits<{ section: [key: string]; refresh: []; add: []; signOut: []; changePassword: [payload: { currentPassword: string; newPassword: string }]; access: [section: 'users' | 'roles']; error: [message: string]; notice: [message: string] }>()
const { t, locale, toggleLocale } = useI18n()
const dark = ref(document.documentElement.classList.contains('dark'))
function toggleTheme() {
  dark.value = !dark.value
  localStorage.setItem('theme', dark.value ? 'dark' : 'light')
  document.documentElement.classList.toggle('dark', dark.value)
}
onMounted(() => { dark.value = document.documentElement.classList.contains('dark') })
</script>

<template>
  <div class="admin-layout">
    <header class="admin-standalone-header">
      <div class="admin-brand"><span class="brand-mark" aria-hidden="true"><i></i></span><span class="admin-brand-copy"><strong>Registry Pulse</strong><small>{{ t.brand }}</small></span></div>
      <div class="admin-header-actions">
        <RouterLink class="admin-header-link" to="/">{{ t.backToSite }}</RouterLink>
        <RouterLink class="admin-header-link" to="/about">{{ t.about }}</RouterLink>
        <button class="icon-button admin-header-icon" type="button" @click="toggleLocale">{{ locale === 'zh' ? 'EN' : '中文' }}</button>
        <button class="icon-button admin-header-icon" type="button" @click="toggleTheme" :aria-label="dark ? 'Light mode' : 'Dark mode'">{{ dark ? '☀' : '☾' }}</button>
        <AdminAccountMenu :token="props.token" @access="emit('access', $event)" @sign-out="emit('signOut')" @change-password="emit('changePassword', $event)" @error="emit('error', $event)" @notice="emit('notice', $event)" />
      </div>
    </header>
    <main class="page admin-page">
      <section class="subhero"><p class="eyebrow">ADMIN CONSOLE</p><h1>{{ t.adminTitle }}</h1><p>{{ locale === 'zh' ? '登录后管理镜像源、检测任务和系统设置' : 'Manage registries, probe tasks, and system settings after sign-in' }}</p></section>
      <nav class="panel admin-nav" aria-label="Admin sections"><button v-for="item in sections" :key="item.key" type="button" class="icon-button" :class="{ active: activeSection === item.key }" @click="emit('section', item.key)">{{ t[item.label] }}</button><button class="refresh-button admin-nav-refresh" type="button" @click="emit('refresh')"><span>↻</span>{{ t.refresh }}</button></nav>
      <slot />
    </main>
    <footer class="site-footer"><div><strong><span class="footer-live"></span> Registry Pulse</strong><p>{{ t.footer }}</p></div><div class="footer-meta"><span>{{ t.softwareName }} {{ t.softwareVersion }}</span><span>© {{ t.copyrightHolder }}</span><span>{{ t.licenseTitle }}</span><a href="https://soft.54bb.com" target="_blank" rel="noopener noreferrer">{{ t.projectWebsite }}</a></div></footer>
  </div>
</template>
