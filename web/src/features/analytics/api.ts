import { useQuery } from '@tanstack/react-query'
import { apiGet } from '../../lib/api'

export interface Unanswered {
  chat_jid: string
  name: string
  waiting_since: string
  waiting_service_s: number
}
export interface Summary {
  inbound: number
  outbound: number
  turns: number
  answered: number
  median_response_s: number
  p90_response_s: number
  unanswered: Unanswered[]
}
export interface HourBucket {
  day: string // YYYY-MM-DD (WIB)
  hour: number
  inbound: number
  outbound: number
}
interface Envelope {
  service_hours: string
  from: string
  to: string
}

/** Inclusive WIB date range ending today. */
export function rangeFor(days: number, now = Date.now()): { from: string; to: string } {
  const wibToday = new Date(now + 7 * 3600_000)
  const to = wibToday.toISOString().slice(0, 10)
  const from = new Date(wibToday.getTime() - (days - 1) * 86400_000).toISOString().slice(0, 10)
  return { from, to }
}

export function useSummary(deviceId: string | undefined, days: number) {
  const { from, to } = rangeFor(days)
  return useQuery({
    queryKey: ['analytics-summary', deviceId, from, to],
    enabled: !!deviceId,
    refetchInterval: 60_000,
    queryFn: () =>
      apiGet<Envelope & { summary: Summary }>(`/api/app/analytics/summary?from=${from}&to=${to}`, deviceId),
  })
}

export function useVolume(deviceId: string | undefined, days: number) {
  const { from, to } = rangeFor(days)
  return useQuery({
    queryKey: ['analytics-volume', deviceId, from, to],
    enabled: !!deviceId,
    queryFn: () => apiGet<Envelope & { buckets: HourBucket[] }>(`/api/app/analytics/volume?from=${from}&to=${to}`, deviceId),
  })
}

/** Inbound messages per weekday (0 = Ahad) × hour, summed over the range. */
export function weekdayGrid(buckets: HourBucket[]): number[][] {
  const grid = Array.from({ length: 7 }, () => Array<number>(24).fill(0))
  for (const b of buckets) {
    const dow = new Date(b.day + 'T00:00:00Z').getUTCDay()
    grid[dow][b.hour] += b.inbound
  }
  return grid
}

/** Hours (0-23) whose start lies inside a service window, from "08:00-15:00,19:00-21:00". */
export function serviceHourSet(raw: string): Set<number> {
  const set = new Set<number>()
  for (const w of raw.split(',')) {
    const [a, b] = w.split('-').map((x) => {
      const [h, m] = x.trim().split(':').map(Number)
      return h * 60 + m
    })
    for (let h = 0; h < 24; h++) if (h * 60 >= a && h * 60 < b) set.add(h)
  }
  return set
}
