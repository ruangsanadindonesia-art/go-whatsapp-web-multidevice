import type { Summary } from './api'
import { formatDuration } from './format'

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-[12px] font-medium text-teal-600">{label}</p>
      <p className="mt-0.5 text-[18px] font-semibold text-teal-900 tabular-nums">{value}</p>
    </div>
  )
}

export function KpiRow({ summary }: { summary: Summary }) {
  const answeredPct = summary.turns ? Math.round((summary.answered / summary.turns) * 100) : 0
  return (
    <section className="grid grid-cols-[minmax(0,1fr)] gap-6 rounded-lg bg-cream-50 p-5 shadow-card md:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)] md:p-6">
      <div>
        <p className="eyebrow">Median waktu respons</p>
        <p className="mt-2 font-display text-[44px] leading-none font-semibold text-teal-900 lining-nums tabular-nums">
          {summary.answered ? formatDuration(summary.median_response_s) : '—'}
        </p>
        <p className="mt-3 text-[13px] text-teal-600 tabular-nums">
          9 dari 10 pertanyaan dibalas dalam{' '}
          <span className="font-semibold text-teal-900">{summary.answered ? formatDuration(summary.p90_response_s) : '—'}</span>
        </p>
      </div>
      <div className="grid grid-cols-2 gap-x-4 gap-y-4 border-t border-sage-300 pt-5 md:border-t-0 md:border-l md:pt-0 md:pl-6">
        <Stat label="Pertanyaan terjawab" value={`${summary.answered}/${summary.turns} · ${answeredPct}%`} />
        <Stat label="Menunggu balasan" value={String(summary.unanswered.length)} />
        <Stat label="Pesan masuk" value={summary.inbound.toLocaleString('id-ID')} />
        <Stat label="Pesan keluar" value={summary.outbound.toLocaleString('id-ID')} />
      </div>
    </section>
  )
}
