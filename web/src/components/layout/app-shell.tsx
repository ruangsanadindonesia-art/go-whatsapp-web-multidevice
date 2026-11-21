import { BarChart3, ExternalLink, Home } from 'lucide-react'
import type { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'
import logo from '../../assets/logo-ruang-sanad.webp'
import { t } from '../../i18n/id'
import { apiRoot } from '../../lib/api'
import type { Device, Identity } from '../../lib/session'
import { DevicePicker } from './device-picker'

const navItem =
  'flex h-10 items-center gap-2.5 rounded-md px-3 text-[14px] font-medium transition-colors duration-150 ' +
  'text-teal-600 hover:bg-gold-100 hover:text-teal-900'
const navActive = 'bg-cream-50 text-teal-900 shadow-[0_0_0_1px_var(--color-sage-300)]'

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
  return (
    <div className="min-h-dvh md:grid md:grid-cols-[248px_1fr]">
      <aside className="flex flex-col gap-6 border-b border-sage-300 px-4 py-5 md:sticky md:top-0 md:h-dvh md:border-r md:border-b-0">
        <div className="flex items-center justify-between gap-3 md:block">
          <img src={logo} alt="Ruang Sanad" className="h-12 w-auto" />
          <p className="eyebrow md:mt-3">{t.brand}</p>
        </div>

        <DevicePicker devices={devices} value={deviceId} onChange={onDeviceChange} />

        <nav className="flex gap-1 md:flex-col" aria-label="Menu utama">
          <NavLink to="/" end className={({ isActive }) => navItem + (isActive ? ' ' + navActive : '')}>
            <Home size={16} strokeWidth={2} aria-hidden /> {t.nav.home}
          </NavLink>
          <NavLink to="/analisis" className={({ isActive }) => navItem + (isActive ? ' ' + navActive : '')}>
            <BarChart3 size={16} strokeWidth={2} aria-hidden /> {t.nav.analytics}
          </NavLink>
          {identity.role === 'admin' && (
            <a href={apiRoot() + '/'} className={navItem}>
              <ExternalLink size={16} strokeWidth={2} aria-hidden /> {t.nav.manageDevices}
            </a>
          )}
        </nav>

        <div className="mt-auto hidden border-t border-sage-300 pt-4 md:block">
          <p className="truncate text-[14px] font-medium text-teal-900">{identity.username}</p>
          <p className="text-[12px] text-teal-600">{t.role[identity.role]}</p>
        </div>
      </aside>
      <main className="mx-auto w-full max-w-[1140px] px-4 py-8 md:px-10 md:py-10">{children}</main>
    </div>
  )
}
