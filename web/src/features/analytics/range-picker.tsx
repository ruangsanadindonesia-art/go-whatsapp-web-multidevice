export const RANGES = [7, 30, 90] as const
export type RangeDays = (typeof RANGES)[number]

export function RangePicker({ value, onChange }: { value: RangeDays; onChange: (d: RangeDays) => void }) {
  return (
    <div role="radiogroup" aria-label="Rentang waktu" className="inline-flex rounded-full bg-cream-200 p-1">
      {RANGES.map((d) => (
        <button
          key={d}
          role="radio"
          aria-checked={value === d}
          onClick={() => onChange(d)}
          className={
            'h-8 rounded-full px-4 text-[13px] font-semibold transition-[background-color,color,box-shadow] duration-150 active:scale-[0.97] ' +
            (value === d ? 'bg-cream-50 text-teal-900 shadow-card' : 'text-teal-600 hover:text-teal-900')
          }
        >
          {d} hari
        </button>
      ))}
    </div>
  )
}
