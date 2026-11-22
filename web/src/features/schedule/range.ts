// Date-range filter for the schedule menu. Dates are inclusive YYYY-MM-DD in WIB.

export type Preset = 'today' | 'yesterday' | '7d' | '14d' | '30d' | 'month' | 'date' | 'custom'
export interface RangeState {
  preset: Preset
  from?: string
  to?: string
}

export const PRESETS: { id: Preset; label: string }[] = [
  { id: 'today', label: 'Hari ini' },
  { id: 'yesterday', label: 'Kemarin' },
  { id: '7d', label: '7 hari' },
  { id: '14d', label: '14 hari' },
  { id: '30d', label: '30 hari' },
  { id: 'month', label: 'Bulan ini' },
]

const DAY = 86400_000
const WIB_OFFSET = 7 * 3600_000
const ISO = /^\d{4}-\d{2}-\d{2}$/

export function wibToday(now = Date.now()): string {
  return new Date(now + WIB_OFFSET).toISOString().slice(0, 10)
}

export function addDays(day: string, n: number): string {
  return new Date(Date.parse(day + 'T00:00:00Z') + n * DAY).toISOString().slice(0, 10)
}

function valid(d?: string): d is string {
  return !!d && ISO.test(d) && !Number.isNaN(Date.parse(d + 'T00:00:00Z'))
}

export function rangeFor(state: RangeState, now = Date.now()): { from: string; to: string } {
  const today = wibToday(now)
  switch (state.preset) {
    case 'today':
      return { from: today, to: today }
    case 'yesterday':
      return { from: addDays(today, -1), to: addDays(today, -1) }
    case '14d':
      return { from: addDays(today, -13), to: today }
    case '30d':
      return { from: addDays(today, -29), to: today }
    case 'month': {
      const [y, m] = today.split('-').map(Number)
      const last = new Date(Date.UTC(y, m, 0)).getUTCDate()
      return { from: `${today.slice(0, 7)}-01`, to: `${today.slice(0, 7)}-${String(last).padStart(2, '0')}` }
    }
    case 'date':
      if (valid(state.from)) return { from: state.from, to: state.from }
      break
    case 'custom':
      if (valid(state.from) && valid(state.to)) {
        return state.from <= state.to ? { from: state.from, to: state.to } : { from: state.to, to: state.from }
      }
      break
  }
  return { from: addDays(today, -6), to: today }
}

/** Read the filter from URL search params (?range=&from=&to=). */
export function rangeFromParams(p: URLSearchParams): RangeState {
  const preset = (p.get('range') ?? '7d') as Preset
  const known: Preset[] = ['today', 'yesterday', '7d', '14d', '30d', 'month', 'date', 'custom']
  return { preset: known.includes(preset) ? preset : '7d', from: p.get('from') ?? undefined, to: p.get('to') ?? undefined }
}
