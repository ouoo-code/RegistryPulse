import { describe, expect, it } from 'vitest'
import { buildFaultTimeline, trendPoints } from '../src/history-utils'
import type { History } from '../src/api'

const history: History[] = [
  { status: 'online', response_ms: 40, checked_at: '2026-08-07T10:00:00Z' },
  { status: 'offline', response_ms: 0, checked_at: '2026-08-07T10:01:00Z', error: 'timeout' },
  { status: 'offline', response_ms: 0, checked_at: '2026-08-07T10:02:00Z', error: 'timeout' },
  { status: 'online', response_ms: 55, checked_at: '2026-08-07T10:03:00Z' }
]

describe('history detail helpers', () => {
  it('groups consecutive fault checks into a timeline event', () => {
    const events = buildFaultTimeline(history)
    expect(events).toHaveLength(1)
    expect(events[0]).toMatchObject({ status: 'offline', count: 2, startedAt: '2026-08-07T10:01:00Z', endedAt: '2026-08-07T10:02:00Z' })
  })

  it('normalizes latency into an SVG polyline', () => {
    const result = trendPoints(history, 100, 50)
    expect(result.max).toBe(55)
    expect(result.points).toHaveLength(4)
    expect(result.polyline).toContain('0,')
  })
})
