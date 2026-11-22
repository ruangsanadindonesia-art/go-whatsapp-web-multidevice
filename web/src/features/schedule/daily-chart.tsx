import { useState } from 'react'
import type { DayPoint } from './api'
import { shortDay } from './format'

// Validated with the dataviz validator on #fefbf7 (all checks pass).
export const SERIES = [
  { key: 'sent', label: 'Terkirim', color: '#2a8360' },
  { key: 'failed', label: 'Gagal', color: '#a4432f' },
  { key: 'upcoming', label: 'Dijadwalkan', color: '#b79947' },
] as const

const H = 180
const GAP = 2

function niceMax(v: number): number {
  if (v <= 4) return 4
  const p = 10 ** Math.floor(Math.log10(v))
  // Even multiples keep the half-way gridline a whole number.
  for (const m of [1, 2, 3, 4, 6, 8, 10]) if (m * p >= v && (m * p) % 2 === 0) return m * p
  return 10 * p
}

export function DailyChart({ data }: { data: DayPoint[] }) {
  const [hover, setHover] = useState<number | null>(null)
  const [table, setTable] = useState(false)
  const max = niceMax(Math.max(1, ...data.map((d) => d.sent + d.failed + d.upcoming)))
  const n = data.length
  const slot = 100 / n
  const barW = Math.min(slot * 0.62, 6) // % of width; thin marks
  const labelEvery = Math.ceil(n / 8)
  const y = (v: number) => (v / max) * H

  return (
    <figure className="rounded-lg bg-cream-50 p-4 shadow-card md:p-5">
      <figcaption className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <span className="font-display text-[20px] font-semibold text-teal-900">Kiriman per hari</span>
        <span className="flex flex-wrap items-center gap-4 text-[13px] text-teal-800">
          {SERIES.map((s) => (
            <span key={s.key} className="flex items-center gap-1.5">
              <svg width="12" height="12" aria-hidden>
                {s.key === 'upcoming' ? (
                  <>
                    <rect width="12" height="12" rx="3" fill="url(#rs-hatch)" stroke={s.color} />
                  </>
                ) : (
                  <rect width="12" height="12" rx="3" fill={s.color} />
                )}
              </svg>
              {s.label}
            </span>
          ))}
          <button
            onClick={() => setTable(!table)}
            className="h-8 rounded-full border border-sage-300 px-3 text-[12px] font-semibold text-teal-800 transition-colors hover:border-gold-500"
            aria-pressed={table}
          >
            {table ? 'Lihat grafik' : 'Lihat tabel'}
          </button>
        </span>
      </figcaption>

      {table ? (
        <div className="max-h-[260px] overflow-auto">
          <table className="w-full text-[13px] tabular-nums">
            <thead className="sticky top-0 bg-cream-50 text-left text-teal-600">
              <tr>
                <th className="py-1.5 font-medium">Tanggal</th>
                {SERIES.map((s) => (
                  <th key={s.key} className="py-1.5 text-right font-medium">
                    {s.label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-sage-300 text-teal-900">
              {data.map((d) => (
                <tr key={d.day}>
                  <td className="py-1.5">{shortDay(d.day)}</td>
                  <td className="py-1.5 text-right">{d.sent}</td>
                  <td className="py-1.5 text-right">{d.failed}</td>
                  <td className="py-1.5 text-right">{d.upcoming}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="relative">
          <svg viewBox={`0 0 100 ${H}`} preserveAspectRatio="none" className="h-[180px] w-full overflow-visible" role="img" aria-label="Grafik kiriman per hari">
            <defs>
              <pattern id="rs-hatch" width="4" height="4" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
                <rect width="4" height="4" fill="#f1e7cc" />
                <line x1="0" y1="0" x2="0" y2="4" stroke="#b79947" strokeWidth="1.6" />
              </pattern>
            </defs>
            {[0.5, 1].map((f) => (
              <line key={f} x1="0" x2="100" y1={H - y(max * f)} y2={H - y(max * f)} stroke="#c9d0c7" strokeWidth="1" strokeDasharray="2 3" vectorEffect="non-scaling-stroke" />
            ))}
            <line x1="0" x2="100" y1={H} y2={H} stroke="#c9d0c7" strokeWidth="1" vectorEffect="non-scaling-stroke" />
            {data.map((d, i) => {
              const x = i * slot + (slot - barW) / 2
              let base = H
              const segs = SERIES.map((s) => {
                const v = d[s.key]
                if (!v) return null
                const h = Math.max(y(v) - GAP, 1.5)
                base -= h + GAP
                return (
                  <rect
                    key={s.key}
                    x={x}
                    y={base + GAP}
                    width={barW}
                    height={h}
                    fill={s.key === 'upcoming' ? 'url(#rs-hatch)' : s.color}
                    opacity={hover === null || hover === i ? 1 : 0.45}
                  />
                )
              })
              return (
                <g key={d.day}>
                  {segs}
                  <rect
                    x={i * slot}
                    y={0}
                    width={slot}
                    height={H}
                    fill="transparent"
                    onMouseEnter={() => setHover(i)}
                    onMouseLeave={() => setHover(null)}
                  />
                </g>
              )
            })}
          </svg>
          <div className="pointer-events-none absolute -left-1 top-0 flex h-[180px] -translate-x-full flex-col justify-between pr-1 text-right text-[11px] text-teal-600 tabular-nums max-md:hidden">
            <span>{max}</span>
            <span>{max / 2}</span>
            <span>0</span>
          </div>
          <div className="mt-2 flex text-[11px] text-teal-600 tabular-nums">
            {data.map((d, i) => (
              <span key={d.day} className="text-center" style={{ width: slot + '%' }}>
                {i % labelEvery === 0 ? shortDay(d.day) : ''}
              </span>
            ))}
          </div>
          {hover !== null && (
            <div
              className="pointer-events-none absolute top-2 z-10 min-w-[150px] rounded-md bg-teal-900 px-3 py-2 text-[12px] text-cream-100 shadow-raised"
              style={{
                left: `${Math.min(Math.max((hover + 0.5) * slot, 12), 88)}%`,
                transform: 'translateX(-50%)',
              }}
            >
              <p className="mb-1 font-semibold">{shortDay(data[hover].day)}</p>
              {SERIES.map((s) => (
                <p key={s.key} className="flex justify-between gap-4 tabular-nums">
                  <span className="text-on-dark-muted">{s.label}</span>
                  <span className="font-semibold">{data[hover][s.key]}</span>
                </p>
              ))}
            </div>
          )}
        </div>
      )}
    </figure>
  )
}
