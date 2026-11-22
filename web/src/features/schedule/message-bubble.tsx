import { FileText, Link2, MapPin, Mic, Music, UserRound, Video } from 'lucide-react'
import { apiUrl } from '../../lib/api'
import type { MessagePreview } from './api'

/** WhatsApp-like rendering of *bold*, _italic_, ~strike~ is not attempted; text keeps its line breaks. */
function Body({ text }: { text?: string }) {
  if (!text) return null
  return <p className="text-[14px] leading-[1.5] whitespace-pre-wrap text-[#1c2b2a] [overflow-wrap:anywhere]">{text}</p>
}

function MediaBlock({ m, deviceId }: { m: MessagePreview; deviceId?: string }) {
  const media = m.media
  if (!media) return null
  const src = media.schedule_id
    ? apiUrl(`/api/app/campaigns/assets/${encodeURIComponent(media.schedule_id)}/${media.field}`, deviceId)
    : media.url
  if (m.type === 'image' || m.type === 'sticker') {
    return src ? (
      <img src={src} alt={media.filename ?? 'Gambar terjadwal'} className="mb-1.5 max-h-[320px] w-full rounded-md object-cover outline outline-1 -outline-offset-1 outline-black/10" />
    ) : null
  }
  if (m.type === 'video' && src) {
    return <video src={src} controls preload="metadata" className="mb-1.5 max-h-[320px] w-full rounded-md bg-teal-900" />
  }
  if (m.type === 'audio' && src) {
    return (
      <div className="mb-1.5 flex items-center gap-2">
        {m.voice_note ? <Mic size={16} className="text-teal-600" aria-label="Pesan suara" /> : <Music size={16} className="text-teal-600" aria-hidden />}
        <audio src={src} controls preload="metadata" className="h-9 w-full" />
      </div>
    )
  }
  const Icon = m.type === 'video' ? Video : FileText
  return (
    <a href={src} target="_blank" rel="noreferrer" className="mb-1.5 flex items-center gap-3 rounded-md bg-black/5 px-3 py-2.5 hover:bg-black/10">
      <Icon size={22} className="text-teal-600" aria-hidden />
      <span className="min-w-0 truncate text-[13px] font-medium text-teal-900">{media.filename ?? media.url ?? 'Lampiran'}</span>
    </a>
  )
}

export function MessageBubble({ m, deviceId, time }: { m: MessagePreview; deviceId?: string; time?: string }) {
  return (
    <div className="rounded-lg bg-[#efe7d8] p-4 md:p-6" aria-label="Pratinjau pesan">
      <div className="ml-auto max-w-[420px] rounded-lg rounded-tr-sm bg-[#d9fdd3] p-2 shadow-[0_1px_0.5px_rgba(11,20,26,0.13)]">
        {m.raw ? (
          <>
            <p className="mb-1 text-[12px] font-medium text-danger">Isi pesan tidak dapat dibaca. Data mentah:</p>
            <pre className="max-h-48 overflow-auto text-[12px] whitespace-pre-wrap text-teal-900">{m.raw}</pre>
          </>
        ) : (
          <div className="px-1 pt-0.5">
            <MediaBlock m={m} deviceId={deviceId} />
            {m.type === 'text' && <Body text={m.text} />}
            {(m.type === 'image' || m.type === 'video' || m.type === 'file' || m.type === 'link') && <Body text={m.caption} />}
            {m.type === 'link' && m.link && (
              <a href={m.link} target="_blank" rel="noreferrer" className="mt-1 flex items-center gap-1.5 text-[13px] break-all text-[#027eb5] underline">
                <Link2 size={14} aria-hidden /> {m.link}
              </a>
            )}
            {m.type === 'location' && (
              <a
                href={`https://www.google.com/maps?q=${m.latitude},${m.longitude}`}
                target="_blank"
                rel="noreferrer"
                className="flex items-center gap-2 text-[14px] text-teal-900 underline decoration-gold-500"
              >
                <MapPin size={16} className="text-danger" aria-hidden /> {m.latitude}, {m.longitude}
              </a>
            )}
            {m.type === 'contact' && (
              <p className="flex items-center gap-3 py-1">
                <span className="grid size-9 place-items-center rounded-full bg-cream-200">
                  <UserRound size={18} className="text-teal-600" aria-hidden />
                </span>
                <span>
                  <span className="block font-medium text-teal-900">{m.contact_name}</span>
                  <span className="text-[13px] text-teal-600 tabular-nums">{m.contact_phone}</span>
                </span>
              </p>
            )}
            {m.type === 'poll' && (
              <div className="py-1">
                <p className="font-semibold text-teal-900">{m.poll_question}</p>
                <p className="mb-2 text-[12px] text-teal-600">{m.poll_max_answer === 1 ? 'Pilih satu' : `Pilih hingga ${m.poll_max_answer ?? 'beberapa'}`}</p>
                <ul className="flex flex-col gap-1.5">
                  {(m.poll_options ?? []).map((o) => (
                    <li key={o} className="flex items-center gap-2 text-[14px] text-teal-900">
                      <span className="size-4 rounded-full border-2 border-teal-400" aria-hidden /> {o}
                    </li>
                  ))}
                </ul>
              </div>
            )}
            {m.type === 'forward' && <p className="text-[13px] text-teal-600 italic">Meneruskan pesan {m.forward_message_id}</p>}
            {m.type === 'audio' && !m.media && <p className="text-[13px] text-teal-600">Audio</p>}
          </div>
        )}
        <p className="mt-0.5 px-1 text-right text-[11px] text-[#667781] tabular-nums">
          {m.view_once && 'Sekali lihat · '}
          {time}
        </p>
      </div>
    </div>
  )
}
