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
