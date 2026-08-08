<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from '../../i18n'
import BaseDialog from '../BaseDialog.vue'

const emit = defineEmits<{ signOut: []; changePassword: [payload: { currentPassword: string; newPassword: string }] }>()
const { t } = useI18n()
const open = ref(false)
const passwordDialogOpen = ref(false)
const currentPassword = ref('')
const newPassword = ref('')

function openPasswordDialog() {
  open.value = false
  passwordDialogOpen.value = true
}

function closePasswordDialog() {
  passwordDialogOpen.value = false
  currentPassword.value = ''
  newPassword.value = ''
}

function submitPassword() {
  emit('changePassword', { currentPassword: currentPassword.value, newPassword: newPassword.value })
  closePasswordDialog()
}
</script>

<template>
  <div class="admin-menu-wrap">
    <button class="admin-menu-toggle" type="button" :aria-expanded="open" :aria-label="t.account" @click="open = !open">{{ t.account }} <span>⌄</span></button>
    <div v-if="open" class="admin-menu-popover">
      <button class="admin-menu-item" type="button" @click="openPasswordDialog">{{ t.changePassword }}<span>›</span></button>
      <button class="admin-menu-item admin-signout" type="button" @click="emit('signOut')">{{ t.signOut }}<span>›</span></button>
    </div>
    <BaseDialog :open="passwordDialogOpen" :title="t.changePassword" size="small" @close="closePasswordDialog">
      <form class="password-form" @submit.prevent="submitPassword">
        <p class="password-form-hint">{{ t.adminLoginDescription }}</p>
        <label>{{ t.currentPassword }}<input v-model="currentPassword" type="password" autocomplete="current-password" required></label>
        <label>{{ t.newPassword }}<input v-model="newPassword" type="password" autocomplete="new-password" minlength="12" required></label>
        <div class="password-form-actions"><button class="refresh" type="submit">{{ t.changePassword }}</button><button class="icon-button" type="button" @click="closePasswordDialog">{{ t.cancel }}</button></div>
      </form>
    </BaseDialog>
  </div>
</template>
