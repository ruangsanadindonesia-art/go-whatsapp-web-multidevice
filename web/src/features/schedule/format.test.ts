import { describe, expect, it } from 'vitest'
import { formatDayRange, shortDay } from './format'

describe('formatDayRange', () => {
  it('same month', () => expect(formatDayRange('2026-11-16', '2026-11-22')).toBe('16–22 Nov 2026'))
  it('across months', () => expect(formatDayRange('2026-10-28', '2026-11-03')).toBe('28 Okt – 3 Nov 2026'))
  it('single day', () => expect(formatDayRange('2026-11-22', '2026-11-22')).toBe('22 Nov 2026'))
  it('shortDay', () => expect(shortDay('2026-11-05')).toBe('5 Nov'))
})
