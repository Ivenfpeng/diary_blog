import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SettingsView from './SettingsView.vue'
import { apiRequest } from '../api/client'
import { clearNotifications, notifications } from '../state/notifications'

vi.mock('../api/client', () => ({ apiRequest: vi.fn() }))

const settings = {
  site_title: 'Diary Blog',
  description: 'Notes',
  author: 'Iven',
  navigation: [],
  social_links: {},
  seo_defaults: {},
}

describe('SettingsView', () => {
  beforeEach(() => {
    vi.mocked(apiRequest).mockReset()
    clearNotifications()
  })

  it('adds operation notifications for save success and failure', async () => {
    vi.mocked(apiRequest)
      .mockResolvedValueOnce({ settings })
      .mockResolvedValueOnce({ settings: { ...settings, site_title: 'Updated Diary' } })
      .mockRejectedValueOnce(new Error('Unable to save settings.'))

    const wrapper = mount(SettingsView)
    await flushPromises()

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(notifications.value.at(-1)).toMatchObject({
      type: 'success',
      title: 'Settings saved',
      message: 'Site settings are ready for visitors.',
    })

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(notifications.value.at(-1)).toMatchObject({
      type: 'error',
      title: 'Settings save failed',
      message: 'Unable to save settings.',
    })
  })
})
