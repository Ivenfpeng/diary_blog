import { describe, expect, it } from 'vitest'
import router from './router'

describe('admin router', () => {
  it('resolves named administration routes beneath the /admin base once', () => {
    expect(router.resolve({ name: 'dashboard' }).href).toBe('/admin/')
    expect(router.resolve({ name: 'login' }).href).toBe('/admin/login')
  })
})
