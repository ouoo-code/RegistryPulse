<script setup lang="ts">
import { computed, ref, onMounted, watch } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { useI18n } from './i18n'

const savedTheme = localStorage.getItem('theme')
const dark = ref(savedTheme ? savedTheme === 'dark' : true)
const menuOpen = ref(false)
const { locale, t, toggleLocale } = useI18n()
const route = useRoute()
const isAdminRoute = computed(() => route.path === '/admin' || route.path.startsWith('/admin/'))

watch(locale, (value) => {
  document.title = value === 'zh' ? '镜像脉动 - Registry Pulse' : 'Registry Pulse - Container Registry Monitor'
}, { immediate: true })

function toggleTheme() {
  dark.value = !dark.value
  localStorage.setItem('theme', dark.value ? 'dark' : 'light')
  document.documentElement.classList.toggle('dark', dark.value)
}

onMounted(() => document.documentElement.classList.toggle('dark', dark.value))
</script>

<template>
  <div v-if="isAdminRoute" class="admin-shell">
    <RouterView />
  </div>
  <div v-else class="shell">
    <header class="topbar">
      <RouterLink to="/" class="brand">
        <span class="brand-mark"><i></i></span>
        <span><strong>Registry Pulse</strong><small>{{ t.brand }}</small></span>
      </RouterLink>
      <button class="menu-toggle icon-button" :aria-expanded="menuOpen" aria-label="Open menu" @click="menuOpen = !menuOpen">☰</button>
      <nav class="site-nav" :class="{ 'menu-open': menuOpen }" :aria-label="t.home">
        <RouterLink to="/" @click="menuOpen = false">{{ t.home }}</RouterLink>
        <RouterLink to="/configure" @click="menuOpen = false">{{ t.configure }}</RouterLink>
        <RouterLink to="/tutorial" @click="menuOpen = false">{{ t.tutorial }}</RouterLink>
        <RouterLink to="/admin" @click="menuOpen = false">{{ t.admin }}</RouterLink>
        <RouterLink to="/about" @click="menuOpen = false">{{ t.about }}</RouterLink>
        <button class="icon-button" @click="toggleLocale">{{ locale === 'zh' ? 'EN' : '中文' }}</button>
        <button class="icon-button" @click="toggleTheme" :aria-label="dark ? 'Light mode' : 'Dark mode'">{{ dark ? '☀' : '☾' }}</button>
      </nav>
    </header>
    <RouterView />
    <footer class="site-footer"><div><strong><span class="footer-live"></span> Registry Pulse</strong><p>{{ t.footer }}</p></div><div class="footer-meta"><span>{{ t.softwareName }} {{ t.softwareVersion }}</span><span>© {{ t.copyrightHolder }}</span><span>{{ t.licenseTitle }}</span><a href="https://soft.54bb.com" target="_blank" rel="noopener noreferrer">{{ t.projectWebsite }}</a></div></footer>
  </div>
</template>
