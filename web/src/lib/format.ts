const WIB = 'Asia/Jakarta'

/** "17.10": 24h clock in WIB with a dot separator (Ruang Sanad copy style). */
export function formatWIBTime(ms: number): string {
  const parts = new Intl.DateTimeFormat('id-ID', { timeZone: WIB, hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })
    .formatToParts(new Date(ms))
  const hh = parts.find((p) => p.type === 'hour')?.value ?? '00'
  const mm = parts.find((p) => p.type === 'minute')?.value ?? '00'
  return hh + '.' + mm
}

const DAY_NAMES = ['Ahad', 'Senin', 'Selasa', 'Rabu', 'Kamis', 'Jumat', 'Sabtu']
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']

/** Wall-clock parts of an instant in WIB. */
function wibParts(ms: number) {
  const d = new Date(ms + 7 * 3600_000) // WIB is UTC+7 with no DST
  return { dow: d.getUTCDay(), day: d.getUTCDate(), month: d.getUTCMonth() }
}

/** "Ahad, 22 Nov · 14.05" in WIB (Ruang Sanad copy style: "Ahad", dot in times). */
export function formatWIBDateTime(iso: string): string {
  const ms = Date.parse(iso)
  const p = wibParts(ms)
  return `${DAY_NAMES[p.dow]}, ${p.day} ${MONTHS[p.month]} · ${formatWIBTime(ms)}`
}

export { DAY_NAMES }

/** Three-letter day names (Ahad → Ahd), Sunday first. */
export const DAY_SHORT = ['Ahd', 'Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab']
