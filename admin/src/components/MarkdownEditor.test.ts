import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import MarkdownEditor from './MarkdownEditor.vue'

describe('MarkdownEditor', () => {
  it('does not emit an update when the parent replaces its Markdown value', async () => {
    const wrapper = mount(MarkdownEditor, { props: { modelValue: '# Local draft' } })

    await wrapper.setProps({ modelValue: '# Restored server draft' })

    expect(wrapper.get('.cm-content').text()).toContain('# Restored server draft')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('emits a string update when the editor document changes', async () => {
    const wrapper = mount(MarkdownEditor, { attachTo: document.body, props: { modelValue: '# Draft' } })
    const content = wrapper.get('.cm-content').element

    content.textContent = '# Changed by editor'
    content.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: '!' }))
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(wrapper.emitted('update:modelValue')).toEqual([['# Changed by editor']])
    wrapper.unmount()
  })
})
