import { useState } from 'react'
import { Route, Routes } from 'react-router-dom'
import { AppShell } from './components/layout/app-shell'
import { t } from './i18n/id'
import { loadSavedDevice, pickDevice, saveDevice, useDevices, useIdentity } from './lib/session'
import { AnalyticsPage } from './pages/analytics'
import { HomePage } from './pages/home'
import { NotFoundPage } from './pages/not-found'
import { SchedulePage } from './pages/schedule'

export default function App() {
  const identity = useIdentity()
  const devices = useDevices(identity.data)
  const [chosen, setChosen] = useState<string | null>(loadSavedDevice)

  if (identity.isPending || (identity.data && devices.isPending)) {
    return <p className="p-8 text-teal-600">{t.common.loading}</p>
  }
  if (identity.isError || devices.isError || !identity.data) {
    return <p className="p-8 text-danger">{t.common.error}</p>
  }

  const list = devices.data ?? []
  const deviceId = pickDevice(list, chosen)
  const device = list.find((d) => d.id === deviceId)
  const choose = (id: string) => {
    saveDevice(id)
    setChosen(id)
  }

  return (
    <AppShell identity={identity.data} devices={list} deviceId={deviceId} onDeviceChange={choose}>
      <Routes>
        <Route path="/" element={<HomePage device={device} />} />
        <Route path="/analisis" element={<AnalyticsPage deviceId={deviceId} />} />
        <Route path="/jadwal/*" element={<SchedulePage deviceId={deviceId} />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </AppShell>
  )
}
