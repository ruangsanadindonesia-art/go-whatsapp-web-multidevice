import { MessageCircle } from 'lucide-react'
import { formatWIBDateTime } from '../../lib/format'
import type { Unanswered } from './api'
import { chatLabel, formatDuration, waLink } from './format'

export function UnansweredList({ items }: { items: Unanswered[] }) {
  return (
    <section>
      <div className="mb-3 flex items-baseline justify-between gap-4">
        <h2 className="font-display text-[22px] font-semibold text-teal-900">Menunggu balasan</h2>
        <p className="text-[13px] text-teal-600">Terlama di atas</p>
      </div>
      {items.length === 0 ? (
        <p className="rounded-lg border border-sage-300 px-5 py-6 text-teal-600">
          Tidak ada chat yang menunggu balasan pada rentang ini.
        </p>
      ) : (
        <ul className="divide-y divide-sage-300 overflow-hidden rounded-lg bg-cream-50 shadow-card">
          {items.map((u) => {
            const link = waLink(u.chat_jid)
            return (
              <li key={u.chat_jid} className="flex items-center gap-4 px-5 py-3.5">
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium text-teal-900">{chatLabel(u.chat_jid, u.name)}</p>
                  <p className="text-[13px] text-teal-600 tabular-nums">Sejak {formatWIBDateTime(u.waiting_since)}</p>
                </div>
                <p className="shrink-0 text-right tabular-nums">
                  <span className="block text-[15px] font-semibold text-danger">{formatDuration(u.waiting_service_s)}</span>
                  <span className="block text-[12px] text-teal-600">jam layanan</span>
                </p>
                {link && (
                  <a
                    href={link}
                    target="_blank"
                    rel="noreferrer"
                    aria-label={'Buka chat ' + chatLabel(u.chat_jid, u.name)}
                    className="grid size-10 shrink-0 place-items-center rounded-full border border-sage-300 text-teal-800 transition-colors hover:border-gold-500 hover:bg-gold-100"
                  >
                    <MessageCircle size={16} strokeWidth={2} aria-hidden />
                  </a>
                )}
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}
