import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TaxonomyView from './TaxonomyView.vue'
import { apiRequest } from '../api/client'
import { clearNotifications, notifications } from '../state/notifications'

vi.mock('../api/client', () => ({ apiRequest: vi.fn() }))

describe('TaxonomyView', () => {
  beforeEach(() => {
    vi.mocked(apiRequest).mockReset()
    clearNotifications()
  })

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
    const slugRegExp = new RegExp(`^(?:${pattern})$`, 'v')
    expect(slugRegExp.test('事实上')).toBe(true)
    expect(slugRegExp.test('事实-2026')).toBe(true)
    expect(slugRegExp.test('bad slug')).toBe(false)
    expect(slugRegExp.test('bad/slug')).toBe(false)
    expect(slugRegExp.test('bad--slug')).toBe(false)
  })

  it('creates taxonomy entries with Chinese slugs', async () => {
    vi.mocked(apiRequest)
      .mockResolvedValueOnce({ categories: [] })
      .mockResolvedValueOnce({ tags: [] })
      .mockResolvedValueOnce({ category: { id: 3, name: '事实上', slug: '事实-2026' } })
    const wrapper = mount(TaxonomyView)
    await flushPromises()

    await wrapper.get('input[required]:not([pattern])').setValue('事实上')
    await wrapper.get('input[pattern]').setValue('事实-2026')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(apiRequest).toHaveBeenCalledWith('/api/admin/categories', {
      method: 'POST',
      body: { name: '事实上', slug: '事实-2026' },
    })
    expect(wrapper.text()).toContain('/事实-2026')
    expect(notifications.value.at(-1)).toMatchObject({ type: 'success', title: 'Category added' })
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
    expect(notifications.value.at(-1)).toMatchObject({ type: 'success', title: 'Category deleted' })
  })
})
