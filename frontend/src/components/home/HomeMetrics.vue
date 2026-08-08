<script setup lang="ts">
import { useI18n } from '../../i18n'
defineProps<{ counts: Record<string, number> }>()
const { t } = useI18n()
</script>

<template>
  <section class="metric-grid" aria-label="Monitoring summary">
    <article class="metric-card total-card"><div class="metric-icon">◉</div><div><span>{{ t.total }}</span><strong>{{ counts.total || 0 }}</strong><small>{{ t.monitoredSites }}</small></div></article>
    <article class="metric-card online-card"><div class="metric-icon">✓</div><div><span>{{ t.online }}</span><strong>{{ counts.online || 0 }}</strong><small>{{ t.operational }}</small></div></article>
    <article class="metric-card degraded-card"><div class="metric-icon">~</div><div><span>{{ t.degraded }}</span><strong>{{ counts.degraded || 0 }}</strong><small>{{ t.needsAttention }}</small></div></article>
    <article class="metric-card offline-card"><div class="metric-icon">!</div><div><span>{{ t.offline }}</span><strong>{{ counts.offline || 0 }}</strong><small>{{ t.unavailable }}</small></div></article>
  </section>
</template>
