import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import LoginView from './LoginView.vue'
import { apiRequest } from '../api/client'

const push = vi.fn()

vi.mock('vue-router', () => ({
  useRouter: () => ({ push }),
}))

vi.mock('../api/client', () => ({
  apiRequest: vi.fn(),
}))

describe('LoginView', () => {
  beforeEach(() => {
    push.mockReset()
    vi.mocked(apiRequest).mockReset()
  })

  it('navigates to the admin dashboard after a successful login', async () => {
    vi.mocked(apiRequest).mockResolvedValue({ authenticated: true, username: 'editor' })
    const wrapper = mount(LoginView)

    await wrapper.get('#username').setValue('editor')
    await wrapper.get('#password').setValue('correct-horse-battery-staple')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(apiRequest).toHaveBeenCalledWith('/api/auth/login', {
      method: 'POST',
      body: { username: 'editor', password: 'correct-horse-battery-staple' },
    })
    expect(push).toHaveBeenCalledWith('/admin')
  })

  it('retains the username and restores password focus when login fails', async () => {
    vi.mocked(apiRequest).mockRejectedValue(new Error('Invalid username or password.'))
    const wrapper = mount(LoginView, { attachTo: document.body })

    await wrapper.get('#username').setValue('editor')
    await wrapper.get('#password').setValue('wrong-password')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()
    await nextTick()

    expect((wrapper.get('#username').element as HTMLInputElement).value).toBe('editor')
    expect((wrapper.get('#password').element as HTMLInputElement).value).toBe('')
    expect(document.activeElement).toBe(wrapper.get('#password').element)
    expect(wrapper.get('[role="alert"]').text()).toBe('Invalid username or password.')

    wrapper.unmount()
  })
})
