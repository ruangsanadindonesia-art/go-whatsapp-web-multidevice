import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { apiGet, apiPost } from '../../lib/api'

export interface Campaign {
  key: string
  message_type: string
  summary: string
  status: string
  recurrence: string
  weekdays?: number[]
  day_of_month?: number
  timezone: string
  scheduled_at: string
  next_run_at?: string
  end_at?: string
  occurrence_limit?: number
  recipients: number
  status_counts: Record<string, number>
  sent: number
  failed: number
  last_run_at?: string
  created_at: string
}
export interface Recipient {
  schedule_id: string
  phone: string
  name: string
  status: string
  occurrence_count: number
  next_run_at?: string
  last_run_at?: string
  last_error?: string
}
export interface Run {
  schedule_id: string
  phone: string
  campaign_key?: string
  summary?: string
  occurrence_at: string
  executed_at: string
  status: 'sent' | 'failed'
  message_id?: string
  error?: string
}
export interface Media {
  field: string
  filename?: string
  content_type?: string
  url?: string
  schedule_id?: string
}
export interface MessagePreview {
  type: string
  text?: string
  caption?: string
  media?: Media
  view_once?: boolean
  voice_note?: boolean
  link?: string
  latitude?: string
  longitude?: string
  contact_name?: string
  contact_phone?: string
  poll_question?: string
  poll_options?: string[]
  poll_max_answer?: number
  forward_message_id?: string
  raw?: string
}
export interface Detail extends Campaign {
  message: MessagePreview
  recipient_list: Recipient[]
  runs: Run[]
}
export interface DayPoint {
  day: string
  sent: number
  failed: number
  upcoming: number
}
export interface Dashboard {
  from: string
  to: string
  approximate: boolean
  log_since?: string
  totals: {
    sent: number
    failed: number
    success_rate: number
    recipients: number
    campaigns: number
    active_campaigns: number
    upcoming: number
  }
  daily: DayPoint[]
  by_type: { type: string; sent: number; failed: number }[]
  top_campaigns: { key: string; summary: string; message_type: string; status: string; sent: number; failed: number }[]
  recent_failures: Run[]
}
export interface CalendarEvent {
  key: string
  summary: string
  message_type: string
  at: string
  kind: 'sent' | 'failed' | 'partial' | 'upcoming' | 'paused'
  recipients: number
  sent: number
  failed: number
}

const q = (from: string, to: string) => `from=${from}&to=${to}`

export function useCampaignDashboard(deviceId: string | undefined, from: string, to: string) {
  return useQuery({
    queryKey: ['campaign-dashboard', deviceId, from, to],
    enabled: !!deviceId,
    refetchInterval: 60_000,
    queryFn: () => apiGet<Dashboard>(`/api/app/campaigns/dashboard?${q(from, to)}`, deviceId),
  })
}

export function useCampaignCalendar(deviceId: string | undefined, from: string, to: string) {
  return useQuery({
    queryKey: ['campaign-calendar', deviceId, from, to],
    enabled: !!deviceId,
    queryFn: () => apiGet<CalendarEvent[]>(`/api/app/campaigns/calendar?${q(from, to)}`, deviceId),
  })
}

export function useCampaigns(deviceId: string | undefined, status: string, search: string) {
  return useQuery({
    queryKey: ['campaigns', deviceId, status, search],
    enabled: !!deviceId,
    queryFn: () =>
      apiGet<Campaign[]>(`/api/app/campaigns?status=${encodeURIComponent(status)}&q=${encodeURIComponent(search)}`, deviceId),
  })
}

export function useCampaign(deviceId: string | undefined, key: string | undefined) {
  return useQuery({
    queryKey: ['campaign', deviceId, key],
    enabled: !!deviceId && !!key,
    queryFn: () => apiGet<Detail>(`/api/app/campaigns/${encodeURIComponent(key!)}`, deviceId),
  })
}

export function useCampaignAction(deviceId: string | undefined, key: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (action: 'pause' | 'resume' | 'cancel') =>
      apiPost<{ changed: number; skipped: number }>(`/api/app/campaigns/${encodeURIComponent(key)}/${action}`, deviceId),
    onSuccess: () => {
      for (const k of ['campaign', 'campaigns', 'campaign-calendar', 'campaign-dashboard']) qc.invalidateQueries({ queryKey: [k] })
    },
  })
}
