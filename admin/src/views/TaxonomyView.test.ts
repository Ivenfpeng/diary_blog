import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TaxonomyView from './TaxonomyView.vue'
import { apiRequest } from '../api/client'

vi.mock('../api/client', () => ({ apiRequest: vi.fn() }))

describe('TaxonomyView', () => {
  beforeEach(() => vi.mocked(apiRequest).mockReset())

  it('uses a browser-compatible slug pattern', async () => {
    vi.mocked(apiRequest)
      .mockResolvedValueOnce({ categories: [] })
      .mockResolvedValueOnce({ tags: [] })
    const wrapper = mount(TaxonomyView)
    await flushPromises()

    const pattern = wrapper.get('input[pattern]').attributes('pattern')
    expect(pattern).toBeTruthy()
    if (!pattern) throw new Error('missing slug pattern')
    expect(() => new RegExp(pattern, 'v')).not.toThrow()
  })

  it('edits and deletes a category', async () => {
    vi.mocked(apiRequest)
      .mockResolvedValueOnce({ categories: [{ id: 2, name: 'Engineering', slug: 'engineering' }] })
      .mockResolvedValueOnce({ tags: [] })
      .mockResolvedValueOnce({ category: { id: 2, name: 'Platform', slug: 'platform' } })
      .mockResolvedValueOnce(undefined)
    const wrapper = mount(TaxonomyView)
    await flushPromises()
    await wrapper.get('[data-testid="edit-categories-2"]').trigger('click')
    await wrapper.get('#taxonomy-edit-name').setValue('Platform')
    await wrapper.get('#taxonomy-edit-slug').setValue('platform')
    await wrapper.get('[data-testid="save-taxonomy-edit"]').trigger('click')
    await flushPromises()
    expect(apiRequest).toHaveBeenCalledWith('/api/admin/categories/2', { method: 'PUT', body: { name: 'Platform', slug: 'platform' } })
    await wrapper.get('[data-testid="delete-categories-2"]').trigger('click')
    await wrapper.get('[data-testid="confirm-taxonomy-delete"]').trigger('click')
    await flushPromises()
    expect(apiRequest).toHaveBeenLastCalledWith('/api/admin/categories/2', { method: 'DELETE' })
    expect(wrapper.text()).not.toContain('Platform')
  })
})
