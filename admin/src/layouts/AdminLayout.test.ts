import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AdminLayout from './AdminLayout.vue'
import { clearNotifications, notifyError, notifySuccess } from '../state/notifications'

describe('AdminLayout', () => {
  it('renders operation notifications above the workspace', () => {
    clearNotifications()
    notifySuccess('Post published', 'The article is now public.')
    notifyError('Upload failed')

    const wrapper = mount(AdminLayout, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          RouterView: { template: '<section>Active admin view</section>' },
        },
      },
    })

    expect(wrapper.get('[aria-label="Operation notifications"]').text()).toContain('Post published')
    expect(wrapper.get('[aria-label="Operation notifications"]').text()).toContain('The article is now public.')
    expect(wrapper.get('[aria-label="Operation notifications"]').text()).toContain('Upload failed')
  })
})
