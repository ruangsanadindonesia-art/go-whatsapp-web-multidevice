const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
export const MONTHS_LONG = ['Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni', 'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember']

/** "22 Nov" from YYYY-MM-DD. */
export function shortDay(day: string): string {
  const [, m, d] = day.split('-').map(Number)
  return `${d} ${MONTHS[m - 1]}`
}

/** "16–22 Nov 2026", "28 Okt – 3 Nov 2026", "22 Nov 2026". */
export function formatDayRange(from: string, to: string): string {
  const [fy, fm, fd] = from.split('-').map(Number)
  const [ty, tm, td] = to.split('-').map(Number)
  if (from === to) return `${fd} ${MONTHS[fm - 1]} ${fy}`
  if (fy === ty && fm === tm) return `${fd}–${td} ${MONTHS[tm - 1]} ${ty}`
  if (fy === ty) return `${fd} ${MONTHS[fm - 1]} – ${td} ${MONTHS[tm - 1]} ${ty}`
  return `${fd} ${MONTHS[fm - 1]} ${fy} – ${td} ${MONTHS[tm - 1]} ${ty}`
}

export function percent(x: number): string {
  return Math.round(x * 100) + '%'
}
