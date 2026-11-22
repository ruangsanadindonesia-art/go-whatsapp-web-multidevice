import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { DAY_NAMES, formatWIBTime } from '../../lib/format'
import { useCampaignCalendar, type CalendarEvent } from './api'
import { monthGrid, typeLabel } from './describe'
import { MONTHS_LONG, shortDay } from './format'
import { wibToday } from './range'

const KIND: Record<CalendarEvent['kind'], { label: string; cls: string; dot: string }> = {
  sent: { label: 'Terkirim', cls: 'bg-sent/12 text-sent-ink before:bg-sent', dot: 'bg-sent' },
  failed: { label: 'Gagal', cls: 'bg-danger/10 text-danger before:bg-danger', dot: 'bg-danger' },
  partial: { label: 'Sebagian gagal', cls: 'bg-danger/10 text-danger before:bg-danger', dot: 'bg-danger' },
  upcoming: { label: 'Dijadwalkan', cls: 'bg-gold-100 text-teal-900 before:bg-gold-500', dot: 'bg-gold-500' },
  paused: { label: 'Dijeda', cls: 'bg-cream-200 text-teal-600 before:bg-teal-400', dot: 'bg-teal-400' },
}

function dayOf(iso: string): string {
  return new Date(Date.parse(iso) + 7 * 3600_000).toISOString().slice(0, 10)
}

function Chip({ e }: { e: CalendarEvent }) {
  const k = KIND[e.kind]
  return (
    <Link
      to={`/jadwal/c/${e.key}`}
      title={`${formatWIBTime(Date.parse(e.at))} · ${e.summary} · ${k.label} · ${e.recipients} penerima`}
      className={
        'relative block truncate rounded-sm py-0.5 pr-1.5 pl-2.5 text-[11px] leading-4 font-medium transition-opacity hover:opacity-80 ' +
        "before:absolute before:inset-y-0.5 before:left-0.5 before:w-[3px] before:rounded-full before:content-[''] " +
        k.cls
      }
    >
      <span className="tabular-nums">{formatWIBTime(Date.parse(e.at))}</span> {e.summary || typeLabel(e.message_type)}
    </Link>
  )
}

export function CalendarView({ deviceId }: { deviceId: string | undefined }) {
  const today = wibToday()
  const [ym, setYm] = useState(() => ({ y: Number(today.slice(0, 4)), m: Number(today.slice(5, 7)) - 1 }))
  const [selected, setSelected] = useState<string>(today)
  const weeks = useMemo(() => monthGrid(ym.y, ym.m), [ym])
  const from = weeks[0][0]
  const to = weeks.at(-1)![6]
  const cal = useCampaignCalendar(deviceId, from, to)
  const byDay = useMemo(() => {
    const m = new Map<string, CalendarEvent[]>()
    for (const e of cal.data ?? []) {
      const d = dayOf(e.at)
      m.set(d, [...(m.get(d) ?? []), e])
    }
    return m
  }, [cal.data])
  const shift = (n: number) =>
    setYm(({ y, m }) => {
      const d = new Date(Date.UTC(y, m + n, 1))
      return { y: d.getUTCFullYear(), m: d.getUTCMonth() }
    })
  const monthKey = `${ym.y}-${String(ym.m + 1).padStart(2, '0')}`
  const agenda = byDay.get(selected) ?? []

  return (
    <div className="grid gap-6 xl:grid-cols-[1fr_300px]">
      <section className="rounded-lg bg-cream-50 p-4 shadow-card md:p-5">
        <header className="mb-4 flex items-center justify-between gap-3">
          <h2 className="font-display text-[22px] font-semibold text-teal-900">
            {MONTHS_LONG[ym.m]} <span className="text-gold-600 lining-nums">{ym.y}</span>
          </h2>
          <div className="flex items-center gap-1">
            <button aria-label="Bulan sebelumnya" onClick={() => shift(-1)} className="grid size-10 place-items-center rounded-full text-teal-800 transition-colors hover:bg-gold-100">
              <ChevronLeft size={18} aria-hidden />
            </button>
            <button
              onClick={() => {
                setYm({ y: Number(today.slice(0, 4)), m: Number(today.slice(5, 7)) - 1 })
                setSelected(today)
              }}
              className="h-9 rounded-full border border-sage-300 px-3.5 text-[13px] font-semibold text-teal-800 transition-colors hover:border-gold-500"
            >
              Hari ini
            </button>
            <button aria-label="Bulan berikutnya" onClick={() => shift(1)} className="grid size-10 place-items-center rounded-full text-teal-800 transition-colors hover:bg-gold-100">
              <ChevronRight size={18} aria-hidden />
            </button>
          </div>
        </header>
        <div className="grid grid-cols-7 border-b border-sage-300 pb-2 text-center text-[12px] font-medium text-teal-600">
          {['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Ahd'].map((d) => (
            <span key={d}>{d}</span>
          ))}
        </div>
        {cal.isPending && <p className="py-10 text-center text-teal-600">Memuat…</p>}
        {cal.isError && <p className="py-10 text-center text-danger">Gagal memuat kalender.</p>}
        <div className="grid grid-cols-7">
          {weeks.flat().map((day) => {
            const events = byDay.get(day) ?? []
            const inMonth = day.startsWith(monthKey)
            const isToday = day === today
            return (
              <button
                key={day}
                onClick={() => setSelected(day)}
                aria-pressed={selected === day}
                aria-label={`${shortDay(day)}, ${events.length} jadwal`}
                className={
                  'flex min-h-[64px] flex-col gap-1 border-b border-sage-300/70 p-1 text-left align-top transition-colors md:min-h-[104px] md:p-1.5 ' +
                  (selected === day ? 'bg-gold-100/60 ' : 'hover:bg-cream-200/60 ') +
                  (inMonth ? '' : 'opacity-45')
                }
              >
                <span
                  className={
                    'grid size-6 place-items-center rounded-full text-[12px] tabular-nums ' +
                    (isToday ? 'bg-teal-800 font-semibold text-white' : 'text-teal-800')
                  }
                >
                  {Number(day.slice(8))}
                </span>
                <span className="hidden min-w-0 flex-col gap-0.5 md:flex">
                  {events.slice(0, 3).map((e, i) => (
                    <Chip key={i} e={e} />
                  ))}
                  {events.length > 3 && <span className="px-1 text-[11px] font-medium text-teal-600">+{events.length - 3} lagi</span>}
                </span>
                {events.length > 0 && (
                  <span className="flex gap-0.5 md:hidden" aria-hidden>
                    {events.slice(0, 4).map((e, i) => (
                      <span key={i} className={'size-1.5 rounded-full ' + KIND[e.kind].dot} />
                    ))}
                  </span>
                )}
              </button>
            )
          })}
        </div>
        <p className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-[12px] text-teal-600">
          {(['upcoming', 'sent', 'partial', 'paused'] as const).map((k) => (
            <span key={k} className="flex items-center gap-1.5">
              <span className={'h-3 w-[3px] rounded-full ' + KIND[k].dot} />
              {KIND[k].label}
            </span>
          ))}
        </p>
      </section>

      <aside className="rounded-lg bg-cream-50 p-5 shadow-card">
        <p className="eyebrow">Agenda</p>
        <h3 className="mt-1 font-display text-[20px] font-semibold text-teal-900">
          {DAY_NAMES[new Date(selected + 'T00:00:00Z').getUTCDay()]}, {shortDay(selected)}
        </h3>
        {agenda.length === 0 ? (
          <p className="mt-4 text-[13px] text-teal-600">Tidak ada jadwal pada hari ini.</p>
        ) : (
          <ol className="mt-4 flex flex-col gap-3">
            {agenda.map((e, i) => (
              <li key={i}>
                <Link to={`/jadwal/c/${e.key}`} className="group flex gap-3">
                  <span className="w-11 shrink-0 pt-0.5 text-[13px] font-semibold text-teal-900 tabular-nums">{formatWIBTime(Date.parse(e.at))}</span>
                  <span className="min-w-0 flex-1 border-l-2 border-gold-500/60 pl-3">
                    <span className="block truncate font-medium text-teal-900 group-hover:text-gold-600">{e.summary || typeLabel(e.message_type)}</span>
                    <span className="text-[12px] text-teal-600">
                      {KIND[e.kind].label} · {e.recipients} penerima
                      {e.failed > 0 && <span className="text-danger"> · {e.failed} gagal</span>}
                    </span>
                  </span>
                </Link>
              </li>
            ))}
          </ol>
        )}
      </aside>
    </div>
  )
}
