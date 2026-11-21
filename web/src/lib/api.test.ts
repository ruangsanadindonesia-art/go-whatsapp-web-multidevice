import { afterEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiRoot, ApiError } from './api'

function setBase(href: string) {
  document.head.querySelectorAll('base').forEach((b) => b.remove())
  const base = document.createElement('base')
  base.href = href
  document.head.appendChild(base)
}

function mockFetch(status: number, body: unknown) {
  const fn = vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }),
  )
  vi.stubGlobal('fetch', fn)
  return fn
}

afterEach(() => vi.unstubAllGlobals())

describe('apiRoot', () => {
  it('strips the trailing app/ segment', () => {
    expect(apiRoot('http://h/app/')).toBe('')
    expect(apiRoot('http://h/wa/app/')).toBe('/wa')
  })
})

describe('apiGet', () => {
  it('calls the server root with the device header', async () => {
    setBase('http://localhost/app/')
    const fn = mockFetch(200, { code: 'SUCCESS', results: { ok: 1 } })
    const res = await apiGet<{ ok: number }>('/chats', 'dev1')
    expect(res).toEqual({ ok: 1 })
    expect(fn).toHaveBeenCalledWith('/chats', expect.objectContaining({ credentials: 'same-origin' }))
    const headers = fn.mock.calls[0][1].headers as Record<string, string>
    expect(headers['X-Device-Id']).toBe('dev1')
  })

  it('honours APP_BASE_PATH', async () => {
    setBase('http://localhost/wa/app/')
    const fn = mockFetch(200, { results: [] })
    await apiGet('/chats')
    expect(fn.mock.calls[0][0]).toBe('/wa/chats')
    const headers = fn.mock.calls[0][1].headers as Record<string, string>
    expect(headers['X-Device-Id']).toBeUndefined()
  })

  it('throws ApiError with the server code', async () => {
    setBase('http://localhost/app/')
    mockFetch(403, { code: 'DEVICE_NOT_ALLOWED', message: 'no' })
    await expect(apiGet('/x')).rejects.toMatchObject({ status: 403, code: 'DEVICE_NOT_ALLOWED' })
    await expect(apiGet('/x')).rejects.toBeInstanceOf(ApiError)
  })
})
