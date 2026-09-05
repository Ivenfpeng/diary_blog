import type { APIErrorResponse } from './types'

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields: Record<string, string>
  readonly requestID: string

  constructor(status: number, error: APIErrorResponse['error']) {
    super(error.message)
    this.name = 'ApiError'
    this.status = status
    this.code = error.code
    this.fields = error.fields
    this.requestID = error.request_id
  }
}

export interface APIRequestOptions extends Omit<RequestInit, 'body'> {
  body?: unknown
}

const unsafeMethods = new Set(['POST', 'PUT', 'PATCH', 'DELETE'])

function readCookie(name: string): string | undefined {
  return document.cookie
    .split('; ')
    .find((cookie) => cookie.startsWith(`${name}=`))
    ?.slice(name.length + 1)
}

export async function apiRequest<T>(path: string, options: APIRequestOptions = {}): Promise<T> {
  const method = (options.method ?? 'GET').toUpperCase()
  const headers = new Headers(options.headers)

  const formData = options.body instanceof FormData
  if (options.body !== undefined && !formData) {
    headers.set('Content-Type', 'application/json')
  }
  if (unsafeMethods.has(method)) {
    const csrfToken = readCookie('diary_csrf')
    if (csrfToken) headers.set('X-CSRF-Token', decodeURIComponent(csrfToken))
  }

  const response = await fetch(path, {
    ...options,
    method,
    headers,
    body: options.body === undefined ? undefined : formData ? options.body as FormData : JSON.stringify(options.body),
    credentials: 'include',
  })

  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as APIErrorResponse | null
    if (payload?.error) throw new ApiError(response.status, payload.error)
    throw new ApiError(response.status, {
      code: 'request_failed',
      message: 'The request could not be completed.',
      fields: {},
      request_id: '',
    })
  }

  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}
