import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it } from 'vitest'
import type { Device, Identity } from '../../lib/session'
import { AppShell } from './app-shell'

const devices: Device[] = [{ id: 'cs-ai', display_name: 'CS Ruang Sanad', state: 'logged_in' }]

function renderShell(identity: Identity, list = devices) {
  return render(
    <MemoryRouter>
      <AppShell identity={identity} devices={list} deviceId={list[0]?.id} onDeviceChange={() => {}}>
        <p>isi</p>
      </AppShell>
    </MemoryRouter>,
  )
}

afterEach(cleanup)

describe('AppShell', () => {
  it('renders a bottom tab bar and a sidebar navigation', () => {
    renderShell({ username: 'cs', role: 'cs', devices: ['cs-ai'] })
    expect(screen.getAllByRole('navigation', { name: 'Menu utama' })).toHaveLength(2)
    expect(screen.getAllByText('Jadwal')).toHaveLength(2)
  })

  it('hides admin menu for CS', () => {
    renderShell({ username: 'cs', role: 'cs', devices: ['cs-ai'] })
    expect(screen.queryAllByText('Kelola device')).toHaveLength(0)
    expect(screen.getAllByText('Beranda')).toHaveLength(2) // bottom tab bar + sidebar
  })

  it('shows admin menu for admin', () => {
    renderShell({ username: 'admin', role: 'admin', devices: [] })
    expect(screen.getAllByText('Kelola device')).toHaveLength(2)
  })

  it('shows no device picker for a single device', () => {
    renderShell({ username: 'cs', role: 'cs', devices: ['cs-ai'] })
    expect(screen.queryAllByRole('combobox')).toHaveLength(0)
    expect(screen.getAllByText('CS Ruang Sanad').length).toBeGreaterThan(0)
  })

  it('shows a picker for several devices', () => {
    renderShell({ username: 'admin', role: 'admin', devices: [] }, [
      ...devices,
      { id: 'keuangan', state: 'connected' },
    ])
    expect(screen.getAllByRole('combobox')).toHaveLength(2) // top bar + sidebar
  })
})
