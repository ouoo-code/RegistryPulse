<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

defineProps<{ empty?: string }>()
const headerPointer = ref<{ x: number; y: number } | null>(null)
const suppressHeaderClick = ref(false)
const tableRoot = ref<HTMLElement | null>(null)
let activeResize: { header: HTMLElement; startX: number; startWidth: number } | null = null

function resizeMove(event: PointerEvent) {
  if (!activeResize) return
  const width = Math.max(72, activeResize.startWidth + event.clientX - activeResize.startX)
  activeResize.header.style.width = `${width}px`
  activeResize.header.style.minWidth = `${width}px`
}

function resizeEnd() {
  if (!activeResize) return
  activeResize = null
  suppressHeaderClick.value = true
  document.body.classList.remove('table-column-resizing')
  window.removeEventListener('pointermove', resizeMove)
  window.removeEventListener('pointerup', resizeEnd)
}

function resizeStart(event: PointerEvent) {
  const handle = event.currentTarget as HTMLElement
  const header = handle.parentElement
  if (!header) return
  event.preventDefault()
  event.stopPropagation()
  activeResize = { header, startX: event.clientX, startWidth: header.getBoundingClientRect().width }
  document.body.classList.add('table-column-resizing')
  window.addEventListener('pointermove', resizeMove)
  window.addEventListener('pointerup', resizeEnd)
}

function addResizeHandles() {
  tableRoot.value?.querySelectorAll('th').forEach(header => {
    if (header.querySelector('.table-column-resizer')) return
    const handle = document.createElement('span')
    handle.className = 'table-column-resizer'
    handle.setAttribute('aria-hidden', 'true')
    handle.addEventListener('pointerdown', resizeStart)
    header.appendChild(handle)
  })
}

function startHeaderPointer(event: PointerEvent) {
  if ((event.target as HTMLElement).closest('th')) {
    headerPointer.value = { x: event.clientX, y: event.clientY }
    suppressHeaderClick.value = false
  }
}

function trackHeaderPointer(event: PointerEvent) {
  if (!headerPointer.value) return
  if (Math.hypot(event.clientX - headerPointer.value.x, event.clientY - headerPointer.value.y) > 4) suppressHeaderClick.value = true
}

function finishHeaderPointer() {
  headerPointer.value = null
}

function ignoreResizeClick(event: MouseEvent) {
  if (!suppressHeaderClick.value) return
  event.preventDefault()
  event.stopPropagation()
  suppressHeaderClick.value = false
}

onMounted(addResizeHandles)
onBeforeUnmount(() => {
  resizeEnd()
  tableRoot.value?.querySelectorAll('.table-column-resizer').forEach(handle => handle.removeEventListener('pointerdown', resizeStart))
})
</script>

<template>
  <section ref="tableRoot" class="base-table panel">
    <div class="base-table-scroll" @pointerdown.capture="startHeaderPointer" @pointermove="trackHeaderPointer" @pointerup="finishHeaderPointer" @click.capture="ignoreResizeClick">
      <table><slot name="head" /><slot name="body" /></table>
    </div>
    <div v-if="empty" class="empty">{{ empty }}</div>
  </section>
</template>
