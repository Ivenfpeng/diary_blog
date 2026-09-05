import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import MediaView from './MediaView.vue'
import { apiRequest } from '../api/client'

vi.mock('../api/client', () => ({ apiRequest: vi.fn() }))

describe('MediaView', () => {
  beforeEach(() => vi.mocked(apiRequest).mockReset())

  it('requires alt text before uploading and reports upload progress', async () => {
    vi.mocked(apiRequest).mockResolvedValueOnce({ media: [] }).mockResolvedValueOnce({ media: { id: 1, path: '2026/09/photo.png', alt_text: 'Photo' } })
    const wrapper = mount(MediaView)
    await flushPromises()
    const file = new File(['image'], 'photo.png', { type: 'image/png' })
    Object.defineProperty(wrapper.get('#media-file').element, 'files', { value: [file] })
    await wrapper.get('#media-file').trigger('change')
    await wrapper.get('form').trigger('submit.prevent')
    expect(wrapper.get('[role="alert"]').text()).toContain('Alt text is required')
    await wrapper.get('#alt-text').setValue('Photo')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()
    expect(apiRequest).toHaveBeenCalledWith('/api/admin/media', expect.objectContaining({ method: 'POST' }))
    expect(wrapper.text()).toContain('Upload complete')
  })

  it('keeps the selected file so a failed upload can be retried', async () => {
    vi.mocked(apiRequest).mockResolvedValueOnce({ media: [] }).mockRejectedValueOnce(new Error('Upload failed'))
    const wrapper = mount(MediaView)
    await flushPromises()
    const file = new File(['image'], 'photo.png', { type: 'image/png' })
    Object.defineProperty(wrapper.get('#media-file').element, 'files', { value: [file] })
    await wrapper.get('#media-file').trigger('change')
    await wrapper.get('#alt-text').setValue('Photo')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('Upload failed')
    expect((wrapper.get('#media-file').element as HTMLInputElement).files?.[0]?.name).toBe('photo.png')
  })

  it('updates alt text for an existing image', async () => {
    vi.mocked(apiRequest)
      .mockResolvedValueOnce({ media: [{ id: 7, path: '2026/09/photo.png', mime_type: 'image/png', width: 1, height: 1, size: 1, alt_text: 'Old text' }] })
      .mockResolvedValueOnce({ media: { id: 7, path: '2026/09/photo.png', mime_type: 'image/png', width: 1, height: 1, size: 1, alt_text: 'New text' } })
    const wrapper = mount(MediaView)
    await flushPromises()
    await wrapper.get('#media-alt-7').setValue('New text')
    await wrapper.get('[data-testid="save-alt-7"]').trigger('click')
    await flushPromises()
    expect(apiRequest).toHaveBeenLastCalledWith('/api/admin/media/7', { method: 'PATCH', body: { alt_text: 'New text' } })
    expect(wrapper.text()).toContain('New text')
  })
})
