import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PostsView from './PostsView.vue'
import { apiRequest } from '../api/client'

vi.mock('../api/client', () => ({ apiRequest: vi.fn() }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))

describe('PostsView', () => {
  beforeEach(() => vi.mocked(apiRequest).mockReset())

  it('wraps pagination in an adaptive horizontally scrollable record bar', async () => {
    vi.mocked(apiRequest).mockResolvedValueOnce({
      posts: [],
      total: 42,
      page: 1,
      page_size: 20,
    })

    const wrapper = mount(PostsView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.get('.pagination-card').attributes('aria-label')).toBe('Posts pagination')
    expect(wrapper.get('.pagination-strip').text()).toContain('Page 1 of 3')
    expect(wrapper.get('.pagination-total').text()).toBe('42 records')
  })
})
