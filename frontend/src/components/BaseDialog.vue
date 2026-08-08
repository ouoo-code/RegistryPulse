<script setup lang="ts">
defineProps<{ open: boolean; title: string; size?: 'small' | 'category' | 'default' }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="dialog-backdrop" role="presentation" @click.self="emit('close')">
      <section class="dialog" :class="{ 'dialog-small': size === 'small', 'dialog-category': size === 'category' }" role="dialog" aria-modal="true" :aria-label="title">
        <header class="dialog-header"><h2>{{ title }}</h2><button type="button" class="icon-button" @click="emit('close')">×</button></header>
        <div class="dialog-body"><slot /></div>
      </section>
    </div>
  </Teleport>
</template>
