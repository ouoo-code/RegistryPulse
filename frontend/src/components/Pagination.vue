<script setup lang="ts">
import { useI18n } from '../i18n'

defineProps<{ page: number; pageCount: number; total: number; pageSize: number; pageSizeOptions?: readonly number[] }>()
const emit = defineEmits<{ change: [page: number]; 'update:page-size': [size: number] }>()
const { locale } = useI18n()
</script>

<template>
  <nav v-if="total > 0" class="pagination" aria-label="Pagination">
    <label class="pagination-page-size"><span>{{ locale === 'zh' ? '每页' : 'Per page' }}</span><select :value="pageSize" @change="emit('update:page-size', Number(($event.target as HTMLSelectElement).value))"><option v-for="size in (pageSizeOptions || [10, 25, 50, 100])" :key="size" :value="size">{{ size }}</option></select></label>
    <span class="pagination-summary">{{ page }} / {{ pageCount }} · {{ total }}</span>
    <button type="button" class="icon-button" :disabled="page <= 1" :title="locale === 'zh' ? '第一页' : 'First page'" @click="emit('change', 1)">|‹</button>
    <button type="button" class="icon-button" :disabled="page <= 1" :title="locale === 'zh' ? '上一页' : 'Previous page'" @click="emit('change', page - 1)">‹</button>
    <button type="button" class="icon-button" :disabled="page >= pageCount" :title="locale === 'zh' ? '下一页' : 'Next page'" @click="emit('change', page + 1)">›</button>
    <button type="button" class="icon-button" :disabled="page >= pageCount" :title="locale === 'zh' ? '最后一页' : 'Last page'" @click="emit('change', pageCount)">›|</button>
  </nav>
</template>
