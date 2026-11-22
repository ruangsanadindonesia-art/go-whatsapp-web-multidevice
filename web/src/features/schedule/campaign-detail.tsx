import { ArrowLeft, CircleAlert, CircleCheck, Pause, Play, X } from 'lucide-react'
import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { PageHeading } from '../../components/ornament'
import { chatLabel } from '../analytics/format'
import { formatWIBDateTime } from '../../lib/format'
import { useCampaign, useCampaignAction } from './api'
import { CAMPAIGN_STATUS, SCHEDULE_STATUS, describeSchedule, typeLabel } from './describe'
import { MessageBubble } from './message-bubble'

const btn =
  'inline-flex h-10 items-center gap-2 rounded-full px-4 text-[13px] font-semibold transition-[background-color,border-color,transform] duration-150 active:scale-[0.97] disabled:opacity-50 disabled:pointer-events-none'

const CONFIRM = {
  pause: 'Jeda semua kiriman terjadwal di campaign ini?',
  resume: 'Lanjutkan kiriman yang dijeda? Kiriman berulang yang terlewat tidak dikirim ulang, tetapi lanjut ke jadwal berikutnya.',
  cancel: 'Batalkan campaign ini? Kiriman yang belum terkirim tidak akan dikirim dan tidak bisa dilanjutkan lagi.',
}

function jid(phone: string) {
  return phone.includes('@') ? phone : phone + '@s.whatsapp.net'
}

export function CampaignDetailPage({ deviceId }: { deviceId: string | undefined }) {
  const { key = '' } = useParams()
  const detail = useCampaign(deviceId, key)
  const action = useCampaignAction(deviceId, key)
  const [note, setNote] = useState<string | null>(null)

  const run = (a: 'pause' | 'resume' | 'cancel') => {
    if (!window.confirm(CONFIRM[a])) return
    setNote(null)
    action.mutate(a, {
      onSuccess: (r) => setNote(`${r.changed} jadwal diubah${r.skipped ? `, ${r.skipped} dilewati (statusnya tidak sesuai)` : ''}.`),
      onError: () => setNote('Aksi gagal. Coba lagi.'),
    })
  }

  if (detail.isPending) return <p className="text-teal-600">Memuat…</p>
  if (detail.isError || !detail.data)
    return (
      <div>
        <p className="mb-4 text-danger">Campaign tidak ditemukan.</p>
        <Link to="/jadwal" className="font-medium text-teal-900 underline decoration-gold-500 underline-offset-4">
          Kembali ke Jadwal
        </Link>
      </div>
    )

  const d = detail.data
  const st = CAMPAIGN_STATUS[d.status]
  const counts = d.status_counts
  const canPause = (counts.active ?? 0) > 0
  const canResume = (counts.paused ?? 0) > 0
  const canCancel = (counts.active ?? 0) + (counts.paused ?? 0) + (counts.failed ?? 0) > 0

  return (
    <>
      <Link to="/jadwal/daftar" className="mb-4 inline-flex items-center gap-1.5 text-[13px] font-medium text-teal-600 hover:text-teal-900">
        <ArrowLeft size={14} aria-hidden /> Semua campaign
      </Link>
      <PageHeading eyebrow={`Campaign · ${typeLabel(d.message_type)}`} title={d.summary ? d.summary.slice(0, 60) : 'Pesan terjadwal'} />

      <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]">
        <div className="flex flex-col gap-6">
          <MessageBubble m={d.message} deviceId={deviceId} time={describeSchedule(d).split(' · ').at(-1)} />
          <dl className="grid grid-cols-2 gap-x-6 gap-y-4 rounded-lg bg-cream-50 p-5 text-[13px] shadow-card">
            <div className="col-span-2">
              <dt className="text-teal-600">Jadwal</dt>
              <dd className="mt-0.5 font-semibold text-teal-900">{describeSchedule(d)}</dd>
            </div>
            <div>
              <dt className="text-teal-600">Status</dt>
              <dd className={'mt-0.5 flex items-center gap-2 font-semibold ' + (st?.tone ?? '')}>
                <span className={'size-2 rounded-full ' + (st?.dot ?? '')} aria-hidden /> {st?.label ?? d.status}
              </dd>
            </div>
            <div>
              <dt className="text-teal-600">Kiriman berikutnya</dt>
              <dd className="mt-0.5 font-semibold text-teal-900 tabular-nums">{d.next_run_at ? formatWIBDateTime(d.next_run_at) : '—'}</dd>
            </div>
            <div>
              <dt className="text-teal-600">Terkirim / gagal</dt>
              <dd className="mt-0.5 font-semibold text-teal-900 tabular-nums">
                {d.sent} / <span className={d.failed ? 'text-danger' : ''}>{d.failed}</span>
              </dd>
            </div>
            <div>
              <dt className="text-teal-600">Dibuat</dt>
              <dd className="mt-0.5 font-semibold text-teal-900 tabular-nums">{formatWIBDateTime(d.created_at)}</dd>
            </div>
          </dl>
          <div className="flex flex-wrap gap-2">
            <button disabled={!canPause || action.isPending} onClick={() => run('pause')} className={btn + ' border border-sage-300 bg-cream-50 text-teal-800 hover:border-gold-500'}>
              <Pause size={16} aria-hidden /> Jeda
            </button>
            <button disabled={!canResume || action.isPending} onClick={() => run('resume')} className={btn + ' bg-teal-800 text-white shadow-button hover:bg-teal-900'}>
              <Play size={16} aria-hidden /> Lanjutkan
            </button>
            <button disabled={!canCancel || action.isPending} onClick={() => run('cancel')} className={btn + ' border border-danger/40 text-danger hover:bg-danger/10'}>
              <X size={16} aria-hidden /> Batalkan
            </button>
          </div>
          {note && (
            <p role="status" className="text-[13px] text-teal-900">
              {note}
            </p>
          )}
        </div>

        <div className="flex flex-col gap-8">
          <section>
            <h2 className="mb-3 font-display text-[20px] font-semibold text-teal-900">
              Penerima <span className="text-gold-600 lining-nums">{d.recipients}</span>
            </h2>
            <ul className="max-h-[420px] divide-y divide-sage-300 overflow-auto rounded-lg bg-cream-50 shadow-card">
              {d.recipient_list.map((r) => (
                <li key={r.schedule_id} className="flex items-start gap-3 px-5 py-3">
                  <span className="min-w-0 flex-1">
                    <span className="block truncate font-medium text-teal-900">{chatLabel(jid(r.phone), r.name)}</span>
                    {r.last_error && r.status !== 'completed' && <span className="block text-[12px] text-danger [overflow-wrap:anywhere]">{r.last_error}</span>}
                  </span>
                  <span className="shrink-0 text-right text-[12px] tabular-nums">
                    <span className={'block font-semibold ' + (r.status === 'failed' ? 'text-danger' : 'text-teal-900')}>{SCHEDULE_STATUS[r.status] ?? r.status}</span>
                    <span className="text-teal-600">
                      {r.next_run_at ? formatWIBDateTime(r.next_run_at) : r.last_run_at ? `Terakhir ${formatWIBDateTime(r.last_run_at)}` : ''}
                    </span>
                  </span>
                </li>
              ))}
            </ul>
          </section>

          <section>
            <h2 className="mb-3 font-display text-[20px] font-semibold text-teal-900">Riwayat kiriman</h2>
            {d.runs.length === 0 ? (
              <p className="rounded-lg border border-sage-300 px-5 py-6 text-[13px] text-teal-600">
                Belum ada kiriman tercatat. Riwayat mulai dicatat sejak fitur ini aktif.
              </p>
            ) : (
              <ol className="max-h-[360px] divide-y divide-sage-300 overflow-auto rounded-lg bg-cream-50 shadow-card">
                {d.runs.map((r, i) => (
                  <li key={i} className="flex items-start gap-3 px-5 py-3 text-[13px]">
                    {r.status === 'sent' ? (
                      <CircleCheck size={16} className="mt-0.5 shrink-0 text-sent" aria-label="Terkirim" />
                    ) : (
                      <CircleAlert size={16} className="mt-0.5 shrink-0 text-danger" aria-label="Gagal" />
                    )}
                    <span className="min-w-0 flex-1">
                      <span className="block truncate font-medium text-teal-900">{chatLabel(jid(r.phone), '')}</span>
                      {r.error && <span className="block text-danger [overflow-wrap:anywhere]">{r.error}</span>}
                    </span>
                    <span className="shrink-0 text-teal-600 tabular-nums">{formatWIBDateTime(r.executed_at)}</span>
                  </li>
                ))}
              </ol>
            )}
          </section>
        </div>
      </div>
    </>
  )
}
