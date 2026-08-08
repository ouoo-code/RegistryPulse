<script setup lang="ts">
import { useI18n } from '../../i18n'
import type { Category, Source } from '../../api'
defineProps<{ categories: Category[]; sources: Source[]; selectedCategory: string }>()
const emit = defineEmits<{ select: [categoryId: string] }>()
const { t, locale } = useI18n()
function categoryCount(category: Category, sources: Source[]) { return sources.filter(source => source.category_id === category.id).length }
</script>

<template>
  <aside class="category-panel" role="tablist" :aria-label="t.categories">
    <div class="panel-heading"><div><span class="panel-kicker">EXPLORE</span><h2>{{ t.categories }}</h2></div><span class="count-badge">{{ categories.length }}</span></div>
    <button type="button" class="category-link" :class="{ active: !selectedCategory }" role="tab" :aria-selected="!selectedCategory" @click="emit('select', '')"><span><i class="category-mark all-mark"></i>{{ locale === 'zh' ? '全部镜源' : t.all }} ({{ sources.length }})</span></button>
    <button v-for="category in categories" :key="category.id" type="button" class="category-link" :class="{ active: selectedCategory === category.id }" role="tab" :aria-selected="selectedCategory === category.id" :title="category.name" @click="emit('select', category.id)"><span><i class="category-mark"></i>{{ category.slug }} ({{ categoryCount(category, sources) }})</span></button>
    <div class="category-note"><strong>{{ t.needConfig }}</strong><p>{{ t.needConfigHint }}</p><RouterLink to="/configure">{{ t.openGenerator }} <span>→</span></RouterLink></div>
  </aside>
</template>
