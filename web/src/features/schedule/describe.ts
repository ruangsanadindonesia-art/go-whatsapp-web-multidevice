import { DAY_NAMES, formatWIBDateTime, formatWIBTime } from '../../lib/format'
import { addDays } from './range'

export interface ScheduleShape {
  recurrence: string
  scheduled_at: string
  weekdays?: number[]
  day_of_month?: number
  occurrence_limit?: number
  end_at?: string
}

/** Human description of a schedule pattern, in WIB. */
export function describeSchedule(s: ScheduleShape): string {
  const time = formatWIBTime(Date.parse(s.scheduled_at))
  let text: string
  switch (s.recurrence) {
    case 'daily':
      text = `Setiap hari · ${time}`
      break
    case 'weekly': {
      const order = [1, 2, 3, 4, 5, 6, 0]
      const days = [...(s.weekdays ?? [])].sort((a, b) => order.indexOf(a) - order.indexOf(b)).map((d) => DAY_NAMES[d])
      text = `Setiap ${days.length ? days.join(' & ') : 'pekan'} · ${time}`
      break
    }
    case 'monthly':
      text = `Setiap tanggal ${s.day_of_month ?? '?'} · ${time}`
      break
    default:
      return `Sekali · ${formatWIBDateTime(s.scheduled_at)}`
  }
  if (s.occurrence_limit) text += ` · ${s.occurrence_limit} kali`
  if (s.end_at) text += ` · sampai ${formatWIBDateTime(s.end_at).split(' · ')[0]}`
  return text
}

const TYPES: Record<string, string> = {
  text: 'Teks',
  image: 'Gambar',
  video: 'Video',
  file: 'Dokumen',
  audio: 'Audio',
  sticker: 'Stiker',
  link: 'Tautan',
  location: 'Lokasi',
  contact: 'Kontak',
  poll: 'Polling',
  forward: 'Teruskan',
}
export function typeLabel(t: string): string {
  return TYPES[t] ?? t
}

/** Campaign status → label + tone class (text colour token). */
export const CAMPAIGN_STATUS: Record<string, { label: string; tone: string; dot: string }> = {
  aktif: { label: 'Aktif', tone: 'text-teal-900', dot: 'bg-gold-500' },
  dijeda: { label: 'Dijeda', tone: 'text-teal-600', dot: 'bg-teal-400' },
  selesai: { label: 'Selesai', tone: 'text-success', dot: 'bg-success' },
  gagal: { label: 'Gagal', tone: 'text-danger', dot: 'bg-danger' },
  dibatalkan: { label: 'Dibatalkan', tone: 'text-teal-600', dot: 'bg-sage-300' },
}

/** Raw schedule status (per recipient) → label. */
export const SCHEDULE_STATUS: Record<string, string> = {
  active: 'Terjadwal',
  running: 'Sedang dikirim',
  paused: 'Dijeda',
  completed: 'Selesai',
  failed: 'Gagal',
  cancelled: 'Dibatalkan',
}

/** Weeks (Senin first) covering a month; each cell is YYYY-MM-DD. month is 0-based. */
export function monthGrid(year: number, month: number): string[][] {
  const first = new Date(Date.UTC(year, month, 1))
  const lead = (first.getUTCDay() + 6) % 7 // days since Monday
  const start = addDays(first.toISOString().slice(0, 10), -lead)
  const last = new Date(Date.UTC(year, month + 1, 0)).toISOString().slice(0, 10)
  const weeks: string[][] = []
  for (let day = start; day <= last || weeks.at(-1)?.length !== 7; ) {
    if (!weeks.length || weeks.at(-1)!.length === 7) {
      if (day > last) break
      weeks.push([])
    }
    weeks.at(-1)!.push(day)
    day = addDays(day, 1)
  }
  return weeks
}
