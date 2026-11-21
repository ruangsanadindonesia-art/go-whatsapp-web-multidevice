/** Seconds as "1j 1m" / "1m 5d" / "42d" (jam, menit, detik). */
export function formatDuration(totalSeconds: number): string {
  const s = Math.max(0, Math.round(totalSeconds))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  if (h > 0) return `${h}j ${m}m`
  if (m > 0) return sec > 0 ? `${m}m ${sec}d` : `${m}m`
  return `${sec}d`
}

function phoneOf(jid: string): string | undefined {
  const [user, server] = jid.split('@')
  if (server !== 's.whatsapp.net' || !/^\d+$/.test(user)) return undefined
  return user
}

/** Display name for a chat: stored name, else a formatted number. */
export function chatLabel(jid: string, name: string): string {
  const phone = phoneOf(jid)
  // Chat storage falls back to the bare number as the name; format it instead.
  if (name && name !== phone && name !== jid) return name
  if (!phone) return 'Kontak tanpa nomor'
  if (phone.startsWith('62') && phone.length >= 10) {
    const rest = phone.slice(2)
    return `+62 ${rest.slice(0, 3)}-${rest.slice(3, 7)}-${rest.slice(7)}`
  }
  return '+' + phone
}

export function waLink(jid: string): string | undefined {
  const phone = phoneOf(jid)
  return phone ? 'https://wa.me/' + phone : undefined
}

/** "08:00-15:00,19:00-21:00" → "08.00–15.00 & 19.00–21.00 WIB". */
export function formatServiceHours(raw: string): string {
  return (
    raw
      .split(',')
      .map((w) => w.trim().replaceAll(':', '.').replace('-', '–'))
      .join(' & ') + ' WIB'
  )
}
