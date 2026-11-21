const WIB = 'Asia/Jakarta'

/** "17.10": 24h clock in WIB with a dot separator (Ruang Sanad copy style). */
export function formatWIBTime(ms: number): string {
  const parts = new Intl.DateTimeFormat('id-ID', { timeZone: WIB, hour: '2-digit', minute: '2-digit', hourCycle: 'h23' })
    .formatToParts(new Date(ms))
  const hh = parts.find((p) => p.type === 'hour')?.value ?? '00'
  const mm = parts.find((p) => p.type === 'minute')?.value ?? '00'
  return hh + '.' + mm
}
