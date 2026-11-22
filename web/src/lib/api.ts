// The Go server injects <base href="{APP_BASE_PATH}/panel/"> into index.html,
// so the same build works under any base path.

export class ApiError extends Error {
  status: number
  code: string
  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

/** Server root (APP_BASE_PATH) derived from the SPA base URL, without trailing slash. */
export function apiRoot(baseURI: string = document.baseURI): string {
  const path = new URL(baseURI).pathname
  return path.replace(/\/?panel\/?$/, '').replace(/\/$/, '')
}

/** Router basename: the SPA base path without trailing slash, e.g. "/panel" or "/wa/panel". */
export function appBasename(baseURI: string = document.baseURI): string {
  return new URL(baseURI).pathname.replace(/\/$/, '')
}

/** GET a GOWA endpoint and return its `results` field (or the whole body when absent). */
export function apiGet<T>(path: string, deviceId?: string): Promise<T> {
  return request<T>('GET', path, deviceId)
}

/** POST without a body (actions); returns `results`. */
export function apiPost<T>(path: string, deviceId?: string): Promise<T> {
  return request<T>('POST', path, deviceId)
}

/** URL for media/elements that cannot send headers (img src): device as query. */
export function apiUrl(path: string, deviceId?: string): string {
  const sep = path.includes('?') ? '&' : '?'
  return apiRoot() + path + (deviceId ? sep + 'device_id=' + encodeURIComponent(deviceId) : '')
}

async function request<T>(method: string, path: string, deviceId?: string): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (deviceId) headers['X-Device-Id'] = deviceId
  const res = await fetch(apiRoot() + path, { method, credentials: 'same-origin', headers })
  let body: { code?: string; message?: string; results?: unknown } = {}
  try {
    body = await res.json()
  } catch {
    // non-JSON error page
  }
  if (!res.ok) {
    throw new ApiError(res.status, body.code ?? 'HTTP_' + res.status, body.message ?? res.statusText)
  }
  return (body.results !== undefined ? body.results : body) as T
}
