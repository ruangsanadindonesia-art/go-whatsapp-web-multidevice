import { t } from '../../i18n/id'
import { deviceLabel, type Device } from '../../lib/session'

export function DevicePicker({
  devices,
  value,
  onChange,
  compact = false,
}: {
  devices: Device[]
  value: string | undefined
  onChange: (id: string) => void
  compact?: boolean // mobile top bar: one line, right-aligned
}) {
  if (devices.length === 0) return <p className="text-[13px] text-teal-600">{t.device.none}</p>
  if (devices.length === 1) {
    return compact ? (
      <p className="min-w-0 truncate text-right text-[13px] font-medium text-teal-900">
        <span className="text-teal-600">{t.device.label} · </span>
        {deviceLabel(devices[0])}
      </p>
    ) : (
      <div>
        <p className="text-[12px] font-medium text-teal-600">{t.device.label}</p>
        <p className="truncate font-medium text-teal-900">{deviceLabel(devices[0])}</p>
      </div>
    )
  }
  const select = (
    <select
      aria-label={t.device.label}
      className={
        'rounded-md border border-sage-300 bg-white px-2.5 text-[14px] text-teal-900 transition-colors duration-150 hover:border-gold-500 ' +
        (compact ? 'h-9 max-w-[180px]' : 'mt-1 h-10 w-full')
      }
      value={value}
      onChange={(e) => onChange(e.target.value)}
    >
      {devices.map((d) => (
        <option key={d.id} value={d.id}>
          {deviceLabel(d)}
        </option>
      ))}
    </select>
  )
  if (compact) return select
  return (
    <label className="block">
      <span className="text-[12px] font-medium text-teal-600">{t.device.label}</span>
      {select}
    </label>
  )
}
