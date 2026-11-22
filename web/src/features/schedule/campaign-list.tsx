import { Search } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { formatWIBDateTime } from '../../lib/format'
import { useCampaigns } from './api'
import { CAMPAIGN_STATUS, describeSchedule, typeLabel } from './describe'

const STATUSES = ['', 'aktif', 'dijeda', 'selesai', 'gagal', 'dibatalkan']

export function CampaignList({ deviceId }: { deviceId: string | undefined }) {
  const [status, setStatus] = useState('')
  const [search, setSearch] = useState('')
  const list = useCampaigns(deviceId, status, search)
  return (
    <section>
      <div className="mb-4 flex flex-wrap items-center gap-3">
        <label className="relative w-full flex-1 sm:w-auto md:max-w-[320px]">
          <span className="sr-only">Cari isi pesan atau nomor</span>
          <Search size={16} className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-teal-600" aria-hidden />
          <input
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Cari isi pesan atau nomor"
            className="h-10 w-full rounded-md border border-sage-300 bg-white pr-3 pl-9 text-[14px] text-teal-900 placeholder:text-teal-600 hover:border-gold-500"
          />
        </label>
        <label>
          <span className="sr-only">Status</span>
          <select value={status} onChange={(e) => setStatus(e.target.value)} className="h-10 rounded-md border border-sage-300 bg-white px-2.5 text-[14px] text-teal-900 hover:border-gold-500">
            {STATUSES.map((s) => (
              <option key={s} value={s}>
                {s ? CAMPAIGN_STATUS[s].label : 'Semua status'}
              </option>
            ))}
          </select>
        </label>
        {list.data && <p className="text-[13px] text-teal-600">{list.data.length} campaign</p>}
      </div>
      {list.isPending && <p className="text-teal-600">Memuat…</p>}
      {list.isError && <p className="text-danger">Gagal memuat daftar.</p>}
      {list.data?.length === 0 && (
        <p className="rounded-lg border border-sage-300 px-5 py-6 text-teal-600">Tidak ada campaign yang cocok.</p>
      )}
      {!!list.data?.length && (
        <ul className="divide-y divide-sage-300 overflow-hidden rounded-lg bg-cream-50 shadow-card">
          {list.data.map((c) => {
            const st = CAMPAIGN_STATUS[c.status]
            return (
              <li key={c.key}>
                <Link to={`/jadwal/c/${c.key}`} className="grid gap-x-6 gap-y-1 px-4 py-3.5 md:px-5 md:py-4 transition-colors hover:bg-gold-100/50 md:grid-cols-[1fr_auto_auto]">
                  <span className="min-w-0">
                    <span className="block truncate font-medium text-teal-900">{c.summary || '(tanpa teks)'}</span>
                    <span className="text-[13px] text-teal-600">
                      {typeLabel(c.message_type)} · {describeSchedule(c)}
                    </span>
                  </span>
                  <span className="text-[13px] text-teal-600 tabular-nums md:text-right">
                    <span className="block font-semibold text-teal-900">{c.recipients} penerima</span>
                    {c.sent} terkirim{c.failed ? <span className="text-danger"> · {c.failed} gagal</span> : null}
                  </span>
                  <span className="flex items-center gap-2 text-[13px] md:w-[180px] md:justify-end">
                    <span className={'size-2 rounded-full ' + (st?.dot ?? 'bg-sage-300')} aria-hidden />
                    <span className={'font-semibold ' + (st?.tone ?? '')}>{st?.label ?? c.status}</span>
                    {c.next_run_at && <span className="text-teal-600 tabular-nums">· {formatWIBDateTime(c.next_run_at).split(', ')[1]}</span>}
                  </span>
                </Link>
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}
