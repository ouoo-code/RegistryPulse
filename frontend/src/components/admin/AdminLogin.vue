<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from '../../i18n'

defineProps<{ error?: string }>()
const emit = defineEmits<{ submit: [credentials: { username: string; password: string; totp_code?: string }] }>()
const { t } = useI18n()
const username = ref('')
const password = ref('')
const totp = ref('')

function submit() {
  emit('submit', { username: username.value, password: password.value, totp_code: totp.value || undefined })
}
</script>

<template>
  <main class="admin-login-screen">
    <section class="admin-login-card">
      <p class="eyebrow">ADMIN CONSOLE</p>
      <h1>{{ t.adminLoginTitle }}</h1>
      <p>{{ t.adminLoginDescription }}</p>
      <form @submit.prevent="submit">
        <label>{{ t.username }}<input v-model="username" autocomplete="username" required></label>
        <label>{{ t.password }}<input v-model="password" type="password" autocomplete="current-password" required></label>
        <label>{{ t.totp }}<input v-model="totp" inputmode="numeric" autocomplete="one-time-code" minlength="6" maxlength="6"></label>
        <button class="refresh" type="submit">{{ t.signIn }}</button>
      </form>
      <div v-if="error" class="alert">{{ error }}</div>
    </section>
  </main>
</template>
