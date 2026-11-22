import { CalendarDays } from 'lucide-react'
import { useState } from 'react'
import { formatDayRange } from './format'
import { PRESETS, rangeFor, type Preset, type RangeState } from './range'

const pill =
  'h-8 shrink-0 rounded-full px-3.5 text-[13px] font-semibold transition-[background-color,color,box-shadow] duration-150 active:scale-[0.97]'
const on = 'bg-cream-50 text-teal-900 shadow-card'
const off = 'text-teal-600 hover:text-teal-900'
const input =
  'h-9 rounded-md border border-sage-300 bg-white px-2.5 text-[14px] text-teal-900 tabular-nums transition-colors hover:border-gold-500'

/** Presets (Hari ini … Bulan ini) plus "Tanggal" and "Rentang" with native date inputs. */
export function DateRangeFilter({ value, onChange }: { value: RangeState; onChange: (v: RangeState) => void }) {
  const current = rangeFor(value)
  const [mode, setMode] = useState<'date' | 'custom' | null>(value.preset === 'date' || value.preset === 'custom' ? value.preset : null)
  const pick = (p: Preset) => {
    setMode(null)
    onChange({ preset: p })
  }
  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <div role="radiogroup" aria-label="Rentang waktu" className="flex max-w-full flex-wrap gap-0.5 rounded-[20px] bg-cream-200 p-1">
          {PRESETS.map((p) => (
            <button key={p.id} role="radio" aria-checked={value.preset === p.id} onClick={() => pick(p.id)} className={pill + ' ' + (value.preset === p.id ? on : off)}>
              {p.label}
            </button>
          ))}
          {(['date', 'custom'] as const).map((m) => (
            <button
              key={m}
              role="radio"
              aria-checked={value.preset === m}
              onClick={() => setMode(mode === m ? null : m)}
              className={pill + ' flex items-center gap-1.5 ' + (value.preset === m || mode === m ? on : off)}
            >
              <CalendarDays size={14} strokeWidth={2} aria-hidden /> {m === 'date' ? 'Tanggal' : 'Rentang'}
            </button>
          ))}
        </div>
        <p className="text-[13px] text-teal-600 tabular-nums" aria-live="polite">
          {formatDayRange(current.from, current.to)}
        </p>
      </div>
      {mode && (
        <form
          className="flex flex-wrap items-end gap-3 rounded-lg bg-cream-50 p-3 shadow-card"
          onSubmit={(e) => {
            e.preventDefault()
            const f = new FormData(e.currentTarget)
            const from = String(f.get('from') ?? '')
            const to = String(f.get('to') ?? from)
            if (from) onChange({ preset: mode, from, to: mode === 'date' ? from : to })
          }}
        >
          <label className="flex flex-col gap-1 text-[12px] font-medium text-teal-600">
            {mode === 'date' ? 'Tanggal' : 'Dari'}
            <input type="date" name="from" required defaultValue={current.from} className={input} />
          </label>
          {mode === 'custom' && (
            <label className="flex flex-col gap-1 text-[12px] font-medium text-teal-600">
              Sampai
              <input type="date" name="to" required defaultValue={current.to} className={input} />
            </label>
          )}
          <button className="h-9 rounded-full bg-teal-800 px-4 text-[13px] font-semibold text-white shadow-button transition-colors hover:bg-teal-900 active:scale-[0.97]">
            Terapkan
          </button>
        </form>
      )}
    </div>
  )
}
