<script setup lang="ts">
defineProps<{
  active: boolean
  ascending: boolean
}>()
</script>

<template>
  <span class="sort-indicator" :class="{ active }" aria-hidden="true">
    {{ active ? (ascending ? '↑' : '↓') : '↕' }}
  </span>
</template>
