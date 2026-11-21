import { describe, expect, it } from 'vitest'
import { pickDevice, type Device } from './session'

const list: Device[] = [
  { id: 'a', state: 'logged_in' },
  { id: 'b', state: 'logged_in' },
]

describe('pickDevice', () => {
  it('keeps a saved device that is still allowed', () => expect(pickDevice(list, 'b')).toBe('b'))
  it('falls back to the first when the saved one is gone', () => expect(pickDevice(list, 'x')).toBe('a'))
  it('returns undefined with no devices', () => expect(pickDevice([], 'a')).toBeUndefined())
})
