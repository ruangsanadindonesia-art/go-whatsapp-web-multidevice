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
  it('hides admin menu for CS', () => {
    renderShell({ username: 'cs', role: 'cs', devices: ['cs-ai'] })
    expect(screen.queryByText('Kelola device')).toBeNull()
    expect(screen.getByText('Beranda')).toBeTruthy()
  })

  it('shows admin menu for admin', () => {
    renderShell({ username: 'admin', role: 'admin', devices: [] })
    expect(screen.getByText('Kelola device')).toBeTruthy()
  })

  it('shows no device picker for a single device', () => {
    renderShell({ username: 'cs', role: 'cs', devices: ['cs-ai'] })
    expect(screen.queryByRole('combobox')).toBeNull()
    expect(screen.getByText('CS Ruang Sanad')).toBeTruthy()
  })

  it('shows a picker for several devices', () => {
    renderShell({ username: 'admin', role: 'admin', devices: [] }, [
      ...devices,
      { id: 'keuangan', state: 'connected' },
    ])
    expect(screen.getByRole('combobox')).toBeTruthy()
  })
})
