import { BarChart3, CalendarClock, ExternalLink, Home, type LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'
import logo from '../../assets/logo-ruang-sanad.webp'
import { t } from '../../i18n/id'
import { apiRoot } from '../../lib/api'
import type { Device, Identity } from '../../lib/session'
import { DevicePicker } from './device-picker'

interface Item {
  to: string
  label: string
  Icon: LucideIcon
  end?: boolean
  external?: boolean
}

function navItems(identity: Identity): Item[] {
  const items: Item[] = [
    { to: '/', label: t.nav.home, Icon: Home, end: true },
    { to: '/analisis', label: t.nav.analytics, Icon: BarChart3 },
    { to: '/jadwal', label: t.nav.schedule, Icon: CalendarClock },
  ]
  if (identity.role === 'admin') items.push({ to: apiRoot() + '/', label: t.nav.manageDevices, Icon: ExternalLink, external: true })
  return items
}

const sideItem =
  'flex h-10 items-center gap-2.5 rounded-md px-3 text-[14px] font-medium transition-colors duration-150 text-teal-600 hover:bg-gold-100 hover:text-teal-900'
const sideActive = 'bg-cream-50 text-teal-900 shadow-[0_0_0_1px_var(--color-sage-300)]'

const tabItem = 'flex min-h-14 flex-col items-center justify-center gap-1 text-[11px] font-semibold transition-colors'

/**
 * Mobile first: a compact top bar plus a bottom tab bar (thumb reach). From
 * md up, a sticky sidebar.
 */
export function AppShell({
  identity,
  devices,
  deviceId,
  onDeviceChange,
  children,
}: {
  identity: Identity
  devices: Device[]
  deviceId: string | undefined
  onDeviceChange: (id: string) => void
  children: ReactNode
}) {
  const items = navItems(identity)
  return (
    <div className="min-h-dvh md:grid md:grid-cols-[248px_minmax(0,1fr)]">
      {/* Mobile top bar */}
      <header className="sticky top-0 z-30 flex h-14 items-center justify-between gap-3 border-b border-sage-300 bg-cream-100/90 px-4 backdrop-blur-md md:hidden">
        <img src={logo} alt="Ruang Sanad" className="h-8 w-auto" />
        <DevicePicker devices={devices} value={deviceId} onChange={onDeviceChange} compact />
      </header>

      {/* Desktop sidebar */}
      <aside className="hidden flex-col gap-6 border-r border-sage-300 px-4 py-5 md:sticky md:top-0 md:flex md:h-dvh">
        <div>
          <img src={logo} alt="Ruang Sanad" className="h-12 w-auto" />
          <p className="eyebrow mt-3">{t.brand}</p>
        </div>
        <DevicePicker devices={devices} value={deviceId} onChange={onDeviceChange} />
        <nav className="flex flex-col gap-1" aria-label="Menu utama">
          {items.map((i) =>
            i.external ? (
              <a key={i.to} href={i.to} className={sideItem}>
                <i.Icon size={16} strokeWidth={2} aria-hidden /> {i.label}
              </a>
            ) : (
              <NavLink key={i.to} to={i.to} end={i.end} className={({ isActive }) => sideItem + (isActive ? ' ' + sideActive : '')}>
                <i.Icon size={16} strokeWidth={2} aria-hidden /> {i.label}
              </NavLink>
            ),
          )}
        </nav>
        <div className="mt-auto border-t border-sage-300 pt-4">
          <p className="truncate text-[14px] font-medium text-teal-900">{identity.username}</p>
          <p className="text-[12px] text-teal-600">{t.role[identity.role]}</p>
        </div>
      </aside>

      <main className="mx-auto w-full max-w-[1140px] min-w-0 px-4 pt-5 pb-28 md:px-10 md:py-10">{children}</main>

      {/* Mobile bottom tab bar */}
      <nav
        aria-label="Menu utama"
        className="fixed inset-x-0 bottom-0 z-30 border-t border-sage-300 bg-cream-50/95 pb-[env(safe-area-inset-bottom)] backdrop-blur-md md:hidden"
      >
        <div className="mx-auto grid max-w-[520px]" style={{ gridTemplateColumns: `repeat(${items.length}, minmax(0, 1fr))` }}>
          {items.map((i) =>
            i.external ? (
              <a key={i.to} href={i.to} className={tabItem + ' text-teal-600'}>
                <i.Icon size={20} strokeWidth={2} aria-hidden />
                <span className="max-w-full truncate px-1">{i.label}</span>
              </a>
            ) : (
              <NavLink
                key={i.to}
                to={i.to}
                end={i.end}
                className={({ isActive }) =>
                  tabItem +
                  (isActive
                    ? ' text-teal-900 before:absolute before:top-0 before:h-0.5 before:w-8 before:rounded-full before:bg-gold-500 relative'
                    : ' text-teal-600')
                }
              >
                <i.Icon size={20} strokeWidth={2} aria-hidden />
                <span className="max-w-full truncate px-1">{i.label}</span>
              </NavLink>
            ),
          )}
        </div>
      </nav>
    </div>
  )
}
