import { describe, expect, it } from 'vitest'
import { rangeFor, serviceHourSet, weekdayGrid } from './api'

describe('rangeFor', () => {
  it('is inclusive and in WIB', () => {
    // 2026-11-23 18:00 UTC = 24 Nov 01:00 WIB
    expect(rangeFor(7, Date.UTC(2026, 10, 23, 18))).toEqual({ from: '2026-11-18', to: '2026-11-24' })
  })
})

describe('weekdayGrid', () => {
  it('sums inbound by weekday and hour', () => {
    const g = weekdayGrid([
      { day: '2026-11-22', hour: 9, inbound: 2, outbound: 5 }, // Ahad
      { day: '2026-11-29', hour: 9, inbound: 3, outbound: 0 }, // Ahad
      { day: '2026-11-23', hour: 20, inbound: 1, outbound: 0 }, // Senin
    ])
    expect(g[0][9]).toBe(5)
    expect(g[1][20]).toBe(1)
  })
})

describe('serviceHourSet', () => {
  it('marks hours inside the windows', () => {
    const s = serviceHourSet('08:00-15:00,19:00-21:00')
    expect([...s].sort((a, b) => a - b)).toEqual([8, 9, 10, 11, 12, 13, 14, 19, 20])
  })
})
