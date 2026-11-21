import { CircleAlert, CircleCheck, CircleDashed } from 'lucide-react'
import { PageHeading } from '../components/ornament'
import { t } from '../i18n/id'
import { formatWIBTime } from '../lib/format'
import { deviceLabel, useDeviceStatus, type Device } from '../lib/session'

function statusView(s: { is_connected: boolean; is_logged_in: boolean }) {
  if (s.is_connected && s.is_logged_in) return { text: t.device.loggedIn, tone: 'text-success', Icon: CircleCheck }
  if (s.is_connected) return { text: t.device.connectedNotLoggedIn, tone: 'text-gold-600', Icon: CircleDashed }
  return { text: t.device.disconnected, tone: 'text-danger', Icon: CircleAlert }
}

export function HomePage({ device }: { device: Device | undefined }) {
  const status = useDeviceStatus(device?.id)
  return (
    <>
      <PageHeading eyebrow={t.home.eyebrow} title={t.home.title} accent={t.home.titleAccent} />
      {!device ? (
        <p className="text-teal-600">{t.device.none}</p>
      ) : (
        <section className="max-w-[560px] rounded-lg bg-cream-50 p-6 shadow-card">
          <p className="text-[13px] font-medium text-teal-600">{deviceLabel(device)}</p>
          {status.isPending && <p className="mt-3 text-teal-600">{t.common.loading}</p>}
          {status.isError && (
            <div className="mt-3 flex items-center gap-3">
              <p className="text-danger">{t.common.error}</p>
              <button
                className="h-9 rounded-full border border-sage-300 bg-cream-50 px-4 text-[13px] font-semibold text-teal-800 transition-colors hover:border-gold-500 active:scale-[0.97]"
                onClick={() => status.refetch()}
              >
                {t.common.retry}
              </button>
            </div>
          )}
          {status.data &&
            (() => {
              const v = statusView(status.data)
              return (
                <>
                  <p className={'mt-2 flex items-center gap-2.5 font-display text-[28px] font-semibold ' + v.tone}>
                    <v.Icon size={24} strokeWidth={2} aria-hidden /> {v.text}
                  </p>
                  <p className="mt-3 text-[13px] text-teal-600 tabular-nums">
                    {t.home.lastCheck}{' '}
                    {formatWIBTime(status.dataUpdatedAt)} WIB
                  </p>
                  {!status.data.is_logged_in && (
                    <p className="mt-4 border-t border-sage-300 pt-4 text-[13px] text-teal-600">{t.device.loginHint}</p>
                  )}
                </>
              )
            })()}
        </section>
      )}
    </>
  )
}
