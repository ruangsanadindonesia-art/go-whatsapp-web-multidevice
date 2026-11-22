import { NavLink, Route, Routes, useSearchParams } from 'react-router-dom'
import { PageHeading } from '../components/ornament'
import { useCampaignDashboard } from '../features/schedule/api'
import { CalendarView } from '../features/schedule/calendar-view'
import { CampaignDetailPage } from '../features/schedule/campaign-detail'
import { CampaignList } from '../features/schedule/campaign-list'
import { DashboardView } from '../features/schedule/dashboard-view'
import { DateRangeFilter } from '../features/schedule/date-range-filter'
import { rangeFor, rangeFromParams, type RangeState } from '../features/schedule/range'
import { t } from '../i18n/id'

const tab = 'relative h-10 px-1 text-[14px] font-semibold transition-colors'

function useRange(): [RangeState, (r: RangeState) => void, { from: string; to: string }] {
  const [params, setParams] = useSearchParams()
  const state = rangeFromParams(params)
  const set = (r: RangeState) => {
    const next = new URLSearchParams()
    next.set('range', r.preset)
    if (r.from) next.set('from', r.from)
    if (r.to) next.set('to', r.to)
    setParams(next, { replace: true })
  }
  return [state, set, rangeFor(state)]
}

function Tabs() {
  const [params] = useSearchParams()
  const qs = params.toString() ? '?' + params.toString() : ''
  const items = [
    { to: '/jadwal', label: 'Dashboard', end: true },
    { to: '/jadwal/kalender', label: 'Kalender', end: false },
    { to: '/jadwal/daftar', label: 'Daftar campaign', end: false },
  ]
  return (
    <nav aria-label="Bagian jadwal" className="mb-6 flex gap-6 border-b border-sage-300">
      {items.map((i) => (
        <NavLink
          key={i.to}
          to={i.to + qs}
          end={i.end}
          className={({ isActive }) =>
            tab +
            (isActive
              ? ' text-teal-900 after:absolute after:inset-x-0 after:-bottom-px after:h-0.5 after:rounded-full after:bg-gold-500'
              : ' text-teal-600 hover:text-teal-900')
          }
        >
          {i.label}
        </NavLink>
      ))}
    </nav>
  )
}

function DashboardTab({ deviceId }: { deviceId: string | undefined }) {
  const [range, setRange, { from, to }] = useRange()
  const dash = useCampaignDashboard(deviceId, from, to)
  return (
    <>
      <div className="mb-6">
        <DateRangeFilter value={range} onChange={setRange} />
      </div>
      {dash.isPending && <p className="text-teal-600">{t.common.loading}</p>}
      {dash.isError && <p className="text-danger">{t.common.error}</p>}
      {dash.data && <DashboardView data={dash.data} />}
    </>
  )
}

// The list shows every campaign (upcoming first); a date filter here would
// hide campaigns that are scheduled after the range.
function ListTab({ deviceId }: { deviceId: string | undefined }) {
  return <CampaignList deviceId={deviceId} />
}

export function SchedulePage({ deviceId }: { deviceId: string | undefined }) {
  if (!deviceId) return <p className="text-teal-600">{t.device.none}</p>
  return (
    <Routes>
      <Route path="c/:key" element={<CampaignDetailPage deviceId={deviceId} />} />
      <Route
        path="*"
        element={
          <>
            <PageHeading eyebrow="Jadwal" title="Campaign" accent="terjadwal" />
            <Tabs />
            <Routes>
              <Route index element={<DashboardTab deviceId={deviceId} />} />
              <Route path="kalender" element={<CalendarView deviceId={deviceId} />} />
              <Route path="daftar" element={<ListTab deviceId={deviceId} />} />
            </Routes>
          </>
        }
      />
    </Routes>
  )
}
