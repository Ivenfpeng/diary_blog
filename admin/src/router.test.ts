import { beforeEach, describe, expect, it, vi } from 'vitest'

const mockedClient = vi.hoisted(() => {
  class ApiError extends Error {
    constructor(readonly status: number) {
      super('The session is unavailable.')
    }
  }

  return { apiRequest: vi.fn(), ApiError }
})

vi.mock('./api/client', () => mockedClient)

import router from './router'
import { resetSessionForTests, setSession } from './state/session'

describe('admin router', () => {
  beforeEach(() => {
    mockedClient.apiRequest.mockReset()
    resetSessionForTests()
  })

  it('resolves named administration routes beneath the /admin base once', () => {
    expect(router.resolve({ name: 'dashboard' }).href).toBe('/admin/')
    expect(router.resolve({ name: 'login' }).href).toBe('/admin/login')
  })

  it('allows the dashboard after login when an anonymous restore preceded it', async () => {
    mockedClient.apiRequest.mockRejectedValueOnce(new mockedClient.ApiError(401))

    await router.push({ name: 'login' })
    expect(router.currentRoute.value.name).toBe('login')
    expect(mockedClient.apiRequest).toHaveBeenCalledTimes(1)
    expect(mockedClient.apiRequest).toHaveBeenCalledWith('/api/auth/session')

    setSession({ authenticated: true, username: 'editor' })
    await router.push({ name: 'dashboard' })

    expect(router.currentRoute.value.name).toBe('dashboard')
    expect(mockedClient.apiRequest).toHaveBeenCalledTimes(1)
  })
})
