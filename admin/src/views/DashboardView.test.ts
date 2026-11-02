import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import DashboardView from './DashboardView.vue'
import { apiRequest } from '../api/client'

vi.mock('../api/client', () => ({ apiRequest: vi.fn() }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))

describe('DashboardView', () => {
  beforeEach(() => vi.mocked(apiRequest).mockReset())

  it('loads workspace metrics and renders quick actions', async () => {
    vi.mocked(apiRequest)
      .mockResolvedValueOnce({ posts: [{ id: 9, title: 'Latest note', slug: 'latest-note', status: 'published', revision: 2 }], total: 9 })
      .mockResolvedValueOnce({ media: [{ id: 1, path: '2026/11/photo.png' }, { id: 2, path: '2026/11/cover.png' }] })
      .mockResolvedValueOnce({ categories: [{ id: 1, name: 'Notes' }] })
      .mockResolvedValueOnce({ tags: [{ id: 1, name: 'Go' }, { id: 2, name: 'Vue' }] })

    const wrapper = mount(DashboardView, {
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="metric-posts"]').text()).toContain('9')
    expect(wrapper.get('[data-testid="metric-media"]').text()).toContain('2')
    expect(wrapper.get('[data-testid="metric-taxonomy"]').text()).toContain('3')
    expect(wrapper.get('[data-testid="recent-post"]').text()).toContain('Latest note')
    expect(wrapper.get('[data-testid="dashboard-actions"]').text()).toContain('New post')
  })
})
