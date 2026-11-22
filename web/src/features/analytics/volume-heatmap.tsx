import { DAY_NAMES, DAY_SHORT } from '../../lib/format'
import { serviceHourSet, weekdayGrid, type HourBucket } from './api'

// Teal ramp on cream; empty cells stay paper-coloured.
const RAMP = ['#f5ecda', '#d6e1dc', '#a9c3bc', '#6f9a95', '#2e6466', '#16484a']

export function VolumeHeatmap({ buckets, serviceHours }: { buckets: HourBucket[]; serviceHours: string }) {
  const grid = weekdayGrid(buckets)
  const max = Math.max(1, ...grid.flat())
  const service = serviceHourSet(serviceHours)
  const order = [1, 2, 3, 4, 5, 6, 0] // Senin … Ahad
  const level = (v: number) => (v === 0 ? 0 : Math.min(RAMP.length - 1, 1 + Math.floor((v / max) * (RAMP.length - 1))))

  return (
    <section>
      <div className="mb-3 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <h2 className="font-display text-[22px] font-semibold text-teal-900">Pola pesan masuk</h2>
        <p className="flex items-center gap-2 text-[13px] text-teal-600">
          <span className="inline-block h-0.5 w-5 bg-gold-500" aria-hidden /> jam layanan
        </p>
      </div>
      <div className="overflow-x-auto rounded-lg bg-cream-50 p-3 shadow-card md:p-5">
        <table className="w-full table-fixed border-separate border-spacing-[2px] md:min-w-[640px] md:border-spacing-[3px]" aria-label="Jumlah pesan masuk per hari dan jam">
          <thead>
            <tr>
              <th className="w-9 md:w-14" />
              {Array.from({ length: 24 }, (_, h) => (
                <th key={h} scope="col" className="relative pb-1.5 text-[11px] font-medium text-teal-600 tabular-nums">
                  <span className="max-md:hidden">{h % 3 === 0 ? String(h).padStart(2, '0') : ''}</span>
                  <span className="md:hidden">{h % 6 === 0 ? String(h).padStart(2, '0') : ''}</span>
                  {service.has(h) && <span className="absolute inset-x-0 bottom-0 h-0.5 bg-gold-500" aria-hidden />}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {order.map((dow) => (
              <tr key={dow}>
                <th scope="row" className="pr-2 text-left text-[12px] font-medium text-teal-600">
                  <span className="md:hidden">{DAY_SHORT[dow]}</span>
                  <span className="max-md:hidden">{DAY_NAMES[dow]}</span>
                </th>
                {grid[dow].map((v, h) => (
                  <td
                    key={h}
                    title={`${DAY_NAMES[dow]} ${String(h).padStart(2, '0')}.00 · ${v} pesan masuk`}
                    className="h-5 rounded-[2px] md:h-6 md:rounded-[3px]"
                    style={{ backgroundColor: RAMP[level(v)] }}
                  />
                ))}
              </tr>
            ))}
          </tbody>
        </table>
        <p className="mt-3 text-[12px] text-teal-600">Jumlah pesan masuk dijumlahkan per hari dalam sepekan dan per jam (WIB).</p>
      </div>
    </section>
  )
}
