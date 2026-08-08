<script setup lang="ts">
defineProps<{ label: string; wide?: boolean }>()
</script>

<template>
  <label class="form-field" :class="{ 'form-field-wide': wide }"><span>{{ label }}</span><slot /></label>
</template>
