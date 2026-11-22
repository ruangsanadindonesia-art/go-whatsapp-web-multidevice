import { describe, expect, it } from 'vitest'
import { rangeFor, wibToday } from './range'

// 2026-11-22 03:00 UTC = Ahad 22 Nov 10.00 WIB
const now = Date.UTC(2026, 10, 22, 3)

describe('rangeFor', () => {
  it('today and yesterday', () => {
    expect(rangeFor({ preset: 'today' }, now)).toEqual({ from: '2026-11-22', to: '2026-11-22' })
    expect(rangeFor({ preset: 'yesterday' }, now)).toEqual({ from: '2026-11-21', to: '2026-11-21' })
  })
  it('last N days include today', () => {
    expect(rangeFor({ preset: '7d' }, now)).toEqual({ from: '2026-11-16', to: '2026-11-22' })
    expect(rangeFor({ preset: '14d' }, now)).toEqual({ from: '2026-11-09', to: '2026-11-22' })
    expect(rangeFor({ preset: '30d' }, now)).toEqual({ from: '2026-10-24', to: '2026-11-22' })
  })
  it('this month runs to its last day', () => {
    expect(rangeFor({ preset: 'month' }, now)).toEqual({ from: '2026-11-01', to: '2026-11-30' })
  })
  it('single date and custom range', () => {
    expect(rangeFor({ preset: 'date', from: '2026-11-05' }, now)).toEqual({ from: '2026-11-05', to: '2026-11-05' })
    expect(rangeFor({ preset: 'custom', from: '2026-11-10', to: '2026-11-03' }, now)).toEqual({ from: '2026-11-03', to: '2026-11-10' })
  })
  it('falls back to 7 days on a bad custom date', () => {
    expect(rangeFor({ preset: 'custom', from: 'x' }, now)).toEqual({ from: '2026-11-16', to: '2026-11-22' })
  })
  it('wibToday crosses midnight in WIB', () => {
    expect(wibToday(Date.UTC(2026, 10, 22, 18))).toBe('2026-11-23')
  })
})
