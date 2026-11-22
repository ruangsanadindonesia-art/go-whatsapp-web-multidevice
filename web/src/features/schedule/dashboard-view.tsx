import { AlertTriangle } from 'lucide-react'
import { Link } from 'react-router-dom'
import { chatLabel } from '../analytics/format'
import { formatWIBDateTime } from '../../lib/format'
import type { Dashboard } from './api'
import { DailyChart, SERIES } from './daily-chart'
import { CAMPAIGN_STATUS, typeLabel } from './describe'
import { percent } from './format'

function Figure({ label, value, note }: { label: string; value: string; note?: string }) {
  return (
    <div>
      <p className="text-[12px] font-medium text-teal-600">{label}</p>
      <p className="mt-0.5 text-[20px] font-semibold text-teal-900 tabular-nums">{value}</p>
      {note && <p className="text-[12px] text-teal-600">{note}</p>}
    </div>
  )
}

export function DashboardView({ data }: { data: Dashboard }) {
  const t = data.totals
  const ran = t.sent + t.failed
  const typeMax = Math.max(1, ...data.by_type.map((x) => x.sent + x.failed))
  return (
    <div className="flex flex-col gap-8">
      {data.approximate && (
        <p className="rounded-md bg-gold-100 px-4 py-2.5 text-[13px] text-teal-900">
          {data.log_since
            ? `Riwayat kiriman tercatat sejak ${formatWIBDateTime(data.log_since)}. Angka sebelum itu tidak lengkap.`
            : 'Belum ada kiriman terjadwal yang tercatat. Angka akan terisi setelah jadwal pertama berjalan.'}
        </p>
      )}

      <section className="grid grid-cols-[minmax(0,1fr)] gap-6 rounded-lg bg-cream-50 p-5 shadow-card md:p-6 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,2fr)]">
        <div>
          <p className="eyebrow">Terkirim</p>
          <p className="mt-2 font-display text-[44px] leading-none md:text-[52px] font-semibold text-teal-900 lining-nums tabular-nums">{t.sent.toLocaleString('id-ID')}</p>
          <div className="mt-4 flex items-center gap-3">
            <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-cream-200" role="img" aria-label={`Tingkat berhasil ${percent(t.success_rate)}`}>
              <div className="h-full rounded-full" style={{ width: ran ? percent(t.success_rate) : '0%', background: SERIES[0].color }} />
            </div>
            <p className="text-[13px] font-semibold text-teal-900 tabular-nums">{ran ? percent(t.success_rate) : '—'} berhasil</p>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-x-4 gap-y-5 border-t border-sage-300 pt-5 sm:grid-cols-4 lg:border-t-0 lg:border-l lg:pt-0 lg:pl-6">
          <Figure label="Gagal" value={t.failed.toLocaleString('id-ID')} note={t.failed ? 'perlu dicek' : undefined} />
          <Figure label="Dijadwalkan" value={t.upcoming.toLocaleString('id-ID')} note="sisa rentang ini" />
          <Figure label="Penerima" value={t.recipients.toLocaleString('id-ID')} note="nomor unik" />
          <Figure label="Campaign aktif" value={String(t.active_campaigns)} note={`${t.campaigns} berjalan di rentang ini`} />
        </div>
      </section>

      <DailyChart data={data.daily} />

      <div className="grid grid-cols-[minmax(0,1fr)] gap-8 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <section>
          <h2 className="mb-3 font-display text-[20px] font-semibold text-teal-900">Campaign teraktif</h2>
          {data.top_campaigns.length === 0 ? (
            <p className="rounded-lg border border-sage-300 px-5 py-6 text-teal-600">Belum ada kiriman pada rentang ini.</p>
          ) : (
            <ol className="divide-y divide-sage-300 overflow-hidden rounded-lg bg-cream-50 shadow-card">
              {data.top_campaigns.map((c, i) => {
                const st = CAMPAIGN_STATUS[c.status]
                return (
                  <li key={c.key}>
                    <Link to={`/jadwal/c/${c.key}`} className="flex items-center gap-3 px-4 py-3.5 transition-colors hover:bg-gold-100/50 md:gap-4 md:px-5">
                      <span className="w-5 font-display text-[18px] text-gold-600 lining-nums">{i + 1}</span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate font-medium text-teal-900">{c.summary || '(tanpa teks)'}</span>
                        <span className="text-[12px] text-teal-600">
                          {typeLabel(c.message_type)} · <span className={st?.tone}>{st?.label ?? c.status}</span>
                        </span>
                      </span>
                      <span className="text-right text-[13px] tabular-nums">
                        <span className="block font-semibold text-teal-900">{c.sent} terkirim</span>
                        {c.failed > 0 && <span className="block text-danger">{c.failed} gagal</span>}
                      </span>
                    </Link>
                  </li>
                )
              })}
            </ol>
          )}
        </section>

        <section>
          <h2 className="mb-3 font-display text-[20px] font-semibold text-teal-900">Per jenis pesan</h2>
          <div className="rounded-lg bg-cream-50 p-5 shadow-card">
            {data.by_type.length === 0 && <p className="text-teal-600">Belum ada data.</p>}
            <ul className="flex flex-col gap-3">
              {data.by_type.map((x) => (
                <li key={x.type}>
                  <div className="mb-1 flex justify-between text-[13px]">
                    <span className="font-medium text-teal-900">{typeLabel(x.type)}</span>
                    <span className="text-teal-600 tabular-nums">
                      {x.sent} terkirim{x.failed ? ` · ${x.failed} gagal` : ''}
                    </span>
                  </div>
                  <div className="flex h-2 gap-[2px]" role="img" aria-label={`${typeLabel(x.type)}: ${x.sent} terkirim, ${x.failed} gagal`}>
                    {x.sent > 0 && <span className="rounded-l-[3px]" style={{ width: (x.sent / typeMax) * 100 + '%', background: SERIES[0].color }} />}
                    {x.failed > 0 && <span className="rounded-r-[3px]" style={{ width: (x.failed / typeMax) * 100 + '%', background: SERIES[1].color }} />}
                  </div>
                </li>
              ))}
            </ul>
          </div>
        </section>
      </div>

      <section>
        <h2 className="mb-3 flex items-center gap-2 font-display text-[20px] font-semibold text-teal-900">
          Kiriman gagal terbaru
        </h2>
        {data.recent_failures.length === 0 ? (
          <p className="rounded-lg border border-sage-300 px-5 py-6 text-teal-600">Tidak ada kiriman gagal pada rentang ini.</p>
        ) : (
          <ul className="divide-y divide-sage-300 overflow-hidden rounded-lg bg-cream-50 shadow-card">
            {data.recent_failures.map((r, i) => (
              <li key={i}>
                <Link to={r.campaign_key ? `/jadwal/c/${r.campaign_key}` : '#'} className="flex items-start gap-3 px-4 py-3.5 transition-colors hover:bg-gold-100/50 md:px-5">
                  <AlertTriangle size={16} strokeWidth={2} className="mt-0.5 shrink-0 text-danger" aria-label="Gagal" />
                  <span className="min-w-0 flex-1">
                    <span className="flex items-baseline justify-between gap-3">
                      <span className="truncate font-medium text-teal-900">
                        {chatLabel(r.phone.includes('@') ? r.phone : r.phone + '@s.whatsapp.net', '')}
                      </span>
                      <span className="shrink-0 text-[12px] text-teal-600 tabular-nums">{formatWIBDateTime(r.executed_at).split(', ')[1]}</span>
                    </span>
                    <span className="block truncate text-[13px] text-teal-600">{r.summary}</span>
                    <span className="block text-[13px] text-danger [overflow-wrap:anywhere]">{r.error || 'Gagal terkirim'}</span>
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}
