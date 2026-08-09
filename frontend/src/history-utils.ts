import type { History } from './api'

export type FaultEvent = {
  startedAt: string
  endedAt: string
  status: string
  message?: string
  count: number
}

export function historyTime(item: History): string {
  return item.checked_at || item.created_at || ''
}

export function sortHistoryNewest(history: History[]): History[] {
  return [...history].sort((left, right) => {
    const leftTimestamp = Date.parse(historyTime(left))
    const rightTimestamp = Date.parse(historyTime(right))
    const timeDifference = (Number.isFinite(rightTimestamp) ? rightTimestamp : 0) - (Number.isFinite(leftTimestamp) ? leftTimestamp : 0)
    if (timeDifference !== 0) return timeDifference
    return `${right.source_id}:${right.id || ''}`.localeCompare(`${left.source_id}:${left.id || ''}`)
  })
}

export function isFault(item: History): boolean {
  return item.status === 'offline' || item.status === 'maintenance' || item.status === 'degraded' || Boolean(item.error)
}

export function buildFaultTimeline(history: History[]): FaultEvent[] {
  const ordered = [...history].sort((a, b) => historyTime(a).localeCompare(historyTime(b)))
  const events: FaultEvent[] = []
  for (const item of ordered) {
    if (!isFault(item)) continue
    const at = historyTime(item) || '—'
    const previous = events[events.length - 1]
    if (previous && previous.status === item.status && previous.endedAt !== '—') {
      previous.endedAt = at
      previous.message = previous.message || item.error
      previous.count += 1
    } else {
      events.push({ startedAt: at, endedAt: at, status: item.status, message: item.error, count: 1 })
    }
  }
  return events.reverse()
}

export function trendPoints(history: History[], width = 640, height = 180) {
  const ordered = [...history].sort((a, b) => historyTime(a).localeCompare(historyTime(b)))
  const values = ordered.map((item) => Number(item.response_ms)).filter((value) => Number.isFinite(value) && value >= 0)
  const max = Math.max(...values, 1)
  const step = ordered.length > 1 ? width / (ordered.length - 1) : width
  const points = ordered.map((item, index) => {
    const value = Number(item.response_ms)
    const ratio = Number.isFinite(value) && value >= 0 ? value / max : 0
    return { x: index * step, y: height - ratio * (height - 18), item }
  })
  return { points, max, polyline: points.map((point) => point.x + ',' + point.y).join(' ') }
}
