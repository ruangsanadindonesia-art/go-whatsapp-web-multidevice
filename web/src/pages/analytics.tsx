import { PageHeading } from '../components/ornament'
import { t } from '../i18n/id'

// Replaced by the analytics page in phase 2 (Task 9).
export function AnalyticsPage() {
  return (
    <>
      <PageHeading eyebrow={t.nav.analytics} title="Analisis" accent="chat CS" />
      <p className="text-teal-600">{t.common.soon}</p>
    </>
  )
}
