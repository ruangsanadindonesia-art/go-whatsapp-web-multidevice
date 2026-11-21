import { describe, expect, it } from 'vitest'
import { formatWIBTime } from './format'

describe('formatWIBTime', () => {
  it('converts UTC to WIB with a dot', () => {
    expect(formatWIBTime(Date.UTC(2026, 10, 21, 10, 5))).toBe('17.05')
  })
  it('wraps past midnight', () => {
    expect(formatWIBTime(Date.UTC(2026, 10, 21, 18, 30))).toBe('01.30')
  })
})

import { formatWIBDateTime } from './format'

describe('formatWIBDateTime', () => {
  it('uses Ahad and WIB', () => {
    // 2026-11-22 is a Sunday; 07:05 UTC = 14.05 WIB
    expect(formatWIBDateTime('2026-11-22T07:05:00Z')).toBe('Ahad, 22 Nov · 14.05')
  })
  it('rolls the day over in WIB', () => {
    expect(formatWIBDateTime('2026-11-22T18:30:00Z')).toBe('Senin, 23 Nov · 01.30')
  })
})
