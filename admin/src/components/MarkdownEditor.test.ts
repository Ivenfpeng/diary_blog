import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import MarkdownEditor from './MarkdownEditor.vue'

async function settle(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, 0))
}

describe('MarkdownEditor', () => {
  it('does not emit an update when the parent replaces its Markdown value', async () => {
    const wrapper = mount(MarkdownEditor, { props: { modelValue: '# Local draft' } })

    await wrapper.setProps({ modelValue: '# Restored server draft' })

    expect(wrapper.get('.rich-editor-surface').text()).toContain('Restored server draft')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('emits Markdown when the rich text document changes', async () => {
    const wrapper = mount(MarkdownEditor, { attachTo: document.body, props: { modelValue: '# Draft' } })
    const surface = wrapper.get('.rich-editor-surface').element as HTMLElement

    surface.innerHTML = '<h1>Changed by editor</h1><p>A <strong>bold</strong> paragraph</p>'
    surface.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: '!' }))
    await settle()

    expect(wrapper.emitted('update:modelValue')).toEqual([['# Changed by editor\n\nA **bold** paragraph']])
    wrapper.unmount()
  })

  it('focuses the editable document when the blank editor shell is clicked', async () => {
    const wrapper = mount(MarkdownEditor, { attachTo: document.body, props: { modelValue: '' } })

    await wrapper.get('[data-testid="markdown-editor"]').trigger('click')

    expect(document.activeElement).toBe(wrapper.get('.rich-editor-surface').element)
    wrapper.unmount()
  })

  it('uploads an image and inserts it as Markdown content', async () => {
    const uploadImage = vi.fn().mockResolvedValue({ src: '/media/2026/09/photo.png', alt: 'Photo' })
    const wrapper = mount(MarkdownEditor, {
      attachTo: document.body,
      props: { modelValue: 'Intro', uploadImage },
    })
    const file = new File(['image'], 'photo.png', { type: 'image/png' })

    Object.defineProperty(wrapper.get('#rich-editor-image-upload').element, 'files', { value: [file] })
    await wrapper.get('#rich-editor-image-upload').trigger('change')
    await settle()

    expect(uploadImage).toHaveBeenCalledWith(file)
    expect(wrapper.get('.rich-editor-surface img').attributes('src')).toBe('/media/2026/09/photo.png')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([expect.stringContaining('Intro')])
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([expect.stringContaining('![Photo](/media/2026/09/photo.png)')])
    wrapper.unmount()
  })
})
