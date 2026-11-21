import { useQuery } from '@tanstack/react-query'
import { apiGet } from './api'

export type Role = 'admin' | 'cs'
export interface Identity {
  username: string
  role: Role
  devices: string[] // allowed devices for CS; empty for admin (all)
}
export interface Device {
  id: string
  display_name?: string
  phone_number?: string
  jid?: string
  state: 'disconnected' | 'connecting' | 'connected' | 'logged_in'
}
export interface DeviceStatus {
  device_id: string
  is_connected: boolean
  is_logged_in: boolean
}

export function useIdentity() {
  return useQuery({ queryKey: ['me'], queryFn: () => apiGet<Identity>('/api/app/me'), staleTime: Infinity })
}

/** Devices the account may use: admins list all, CS accounts get their scope. */
export function useDevices(identity: Identity | undefined) {
  return useQuery({
    queryKey: ['devices', identity?.username],
    enabled: !!identity,
    queryFn: async (): Promise<Device[]> => {
      if (identity!.role === 'admin') return (await apiGet<Device[] | null>('/devices')) ?? []
      return Promise.all(identity!.devices.map((id) => apiGet<Device>('/devices/' + encodeURIComponent(id))))
    },
  })
}

export function useDeviceStatus(deviceId: string | undefined) {
  return useQuery({
    queryKey: ['device-status', deviceId],
    enabled: !!deviceId,
    refetchInterval: 15_000,
    queryFn: () => apiGet<DeviceStatus>('/devices/' + encodeURIComponent(deviceId!) + '/status', deviceId),
  })
}

const KEY = 'rs-panel.device'
export function loadSavedDevice(): string | null {
  try {
    return localStorage.getItem(KEY)
  } catch {
    return null
  }
}
export function saveDevice(id: string) {
  try {
    localStorage.setItem(KEY, id)
  } catch {
    // private mode / blocked storage: selection just isn't remembered
  }
}

/** Pick the saved device when still allowed, else the first one. */
export function pickDevice(devices: Device[], saved: string | null): string | undefined {
  if (saved && devices.some((d) => d.id === saved)) return saved
  return devices[0]?.id
}

export function deviceLabel(d: Device): string {
  return d.display_name || d.phone_number || d.id
}
