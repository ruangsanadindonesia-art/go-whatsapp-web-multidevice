import { describe, expect, it } from 'vitest'
import { describeSchedule, monthGrid, typeLabel } from './describe'

describe('describeSchedule', () => {
  const at = '2026-11-24T01:00:00Z' // Selasa 24 Nov 08.00 WIB
  it('once', () => expect(describeSchedule({ recurrence: 'once', scheduled_at: at })).toBe('Sekali · Selasa, 24 Nov · 08.00'))
  it('daily', () => expect(describeSchedule({ recurrence: 'daily', scheduled_at: at })).toBe('Setiap hari · 08.00'))
  it('weekly', () =>
    expect(describeSchedule({ recurrence: 'weekly', scheduled_at: at, weekdays: [4, 1] })).toBe('Setiap Senin & Kamis · 08.00'))
  it('monthly with limit', () =>
    expect(describeSchedule({ recurrence: 'monthly', scheduled_at: at, day_of_month: 5, occurrence_limit: 3 })).toBe(
      'Setiap tanggal 5 · 08.00 · 3 kali',
    ))
})

describe('typeLabel', () => {
  it('maps known types', () => {
    expect(typeLabel('image')).toBe('Gambar')
    expect(typeLabel('weird')).toBe('weird')
  })
})

describe('monthGrid', () => {
  it('starts on Monday and covers whole weeks', () => {
    const weeks = monthGrid(2026, 10) // November 2026: 1 Nov is a Sunday
    expect(weeks[0][0]).toBe('2026-10-26')
    expect(weeks[0][6]).toBe('2026-11-01')
    expect(weeks.at(-1)![6]).toBe('2026-12-06')
    expect(weeks.every((w) => w.length === 7)).toBe(true)
  })
})
