import { Link } from 'react-router-dom'
import { PageHeading } from '../components/ornament'
import { t } from '../i18n/id'

export function NotFoundPage() {
  return (
    <>
      <PageHeading eyebrow="404" title={t.notFound.title} />
      <Link to="/" className="font-medium text-teal-900 underline decoration-gold-500 underline-offset-4 hover:text-gold-600">
        {t.notFound.back}
      </Link>
    </>
  )
}
