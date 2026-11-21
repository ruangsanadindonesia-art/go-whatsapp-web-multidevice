import { describe, expect, it } from 'vitest'
import { chatLabel, formatDuration, formatServiceHours, waLink } from './format'

describe('formatDuration', () => {
  it('minutes and seconds', () => expect(formatDuration(65)).toBe('1m 5d'))
  it('hours and minutes', () => expect(formatDuration(3700)).toBe('1j 1m'))
  it('zero', () => expect(formatDuration(0)).toBe('0d'))
  it('seconds only', () => expect(formatDuration(42)).toBe('42d'))
  it('whole minutes', () => expect(formatDuration(120)).toBe('2m'))
})

describe('chatLabel', () => {
  it('prefers the stored name', () => expect(chatLabel('6281234567890@s.whatsapp.net', 'Fulan')).toBe('Fulan'))
  it('formats an Indonesian number', () => expect(chatLabel('6281234567890@s.whatsapp.net', '')).toBe('+62 812-3456-7890'))
  it('formats a name that is just the number', () =>
    expect(chatLabel('6281234567890@s.whatsapp.net', '6281234567890')).toBe('+62 812-3456-7890'))
  it('hides LID identifiers', () => expect(chatLabel('123456789@lid', '')).toBe('Kontak tanpa nomor'))
})

describe('waLink', () => {
  it('links phone JIDs', () => expect(waLink('6281234567890@s.whatsapp.net')).toBe('https://wa.me/6281234567890'))
  it('no link for LID', () => expect(waLink('123@lid')).toBeUndefined())
})

describe('formatServiceHours', () => {
  it('uses dots and en dashes', () =>
    expect(formatServiceHours('08:00-15:00,19:00-21:00')).toBe('08.00–15.00 & 19.00–21.00 WIB'))
})
