import { useState } from 'react'
import { PageHeading } from '../components/ornament'
import { useSummary, useVolume } from '../features/analytics/api'
import { formatServiceHours } from '../features/analytics/format'
import { KpiRow } from '../features/analytics/kpi-row'
import { RangePicker, type RangeDays } from '../features/analytics/range-picker'
import { UnansweredList } from '../features/analytics/unanswered-list'
import { VolumeHeatmap } from '../features/analytics/volume-heatmap'
import { t } from '../i18n/id'
import { ApiError } from '../lib/api'

function ErrorNote({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const msg =
    error instanceof ApiError && error.code === 'VALIDATION_ERROR' ? error.message.replace(/^analytics: /, '') : t.common.error
  return (
    <div className="flex items-center gap-3 rounded-lg border border-sage-300 px-5 py-4">
      <p className="text-danger">{msg}</p>
      <button
        onClick={onRetry}
        className="h-9 rounded-full border border-sage-300 bg-cream-50 px-4 text-[13px] font-semibold text-teal-800 transition-colors hover:border-gold-500 active:scale-[0.97]"
      >
        {t.common.retry}
      </button>
    </div>
  )
}

export function AnalyticsPage({ deviceId }: { deviceId: string | undefined }) {
  const [days, setDays] = useState<RangeDays>(7)
  const summary = useSummary(deviceId, days)
  const volume = useVolume(deviceId, days)
  const hours = summary.data?.service_hours ?? volume.data?.service_hours

  return (
    <>
      <PageHeading eyebrow={t.nav.analytics} title="Respons" accent="CS" />
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <RangePicker value={days} onChange={setDays} />
        {hours && <p className="text-[13px] text-teal-600">Waktu dihitung dalam jam layanan {formatServiceHours(hours)}</p>}
      </div>

      {!deviceId && <p className="text-teal-600">{t.device.none}</p>}

      <div className="flex flex-col gap-10">
        {summary.isPending && deviceId && <p className="text-teal-600">{t.common.loading}</p>}
        {summary.isError && <ErrorNote error={summary.error} onRetry={() => summary.refetch()} />}
        {summary.data && (
          <>
            <KpiRow summary={summary.data.summary} />
            <UnansweredList items={summary.data.summary.unanswered} />
          </>
        )}
        {volume.data && <VolumeHeatmap buckets={volume.data.buckets} serviceHours={volume.data.service_hours} />}
        <p className="max-w-[760px] border-t border-sage-300 pt-4 text-[13px] text-teal-600">
          Catatan: balasan dari AI dan dari HP ikut dihitung sebagai balasan CS sampai fitur atribusi pengirim aktif. Hanya
          chat pribadi yang dihitung (grup dan siaran tidak). Pertanyaan dihitung pada rentang tempat ia pertama masuk.
        </p>
      </div>
    </>
  )
}
