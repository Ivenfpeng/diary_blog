import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PostEditorView from './PostEditorView.vue'

const mockedClient = vi.hoisted(() => ({ apiRequest: vi.fn() }))
vi.mock('../api/client', () => mockedClient)
vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '7' } }),
  useRouter: () => ({ push: vi.fn() }),
}))

describe('PostEditorView', () => {
  it('keeps publish unavailable until a valid saved article is ready', async () => {
    mockedClient.apiRequest.mockResolvedValue({
      post: {
        id: 7, slug: '', title: '', summary: '', content_md: '', status: 'draft',
        category_id: null, tag_ids: [], revision: 3,
      },
    })

    const wrapper = mount(PostEditorView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          MarkdownEditor: {
            props: ['modelValue'],
            emits: ['update:modelValue'],
            template: '<textarea data-testid="markdown-editor" @input="$emit(\'update:modelValue\', $event.target.value)" />',
          },
          PublishPanel: { props: ['canPublish'], template: '<button name="publish" :disabled="!canPublish">Publish</button>' },
          RevisionPanel: true,
        },
      },
    })
    await vi.waitFor(() => expect(wrapper.find('.editor-form').exists()).toBe(true))

    expect(wrapper.get('button[name="publish"]').attributes('disabled')).toBeDefined()
    await wrapper.get('#title').setValue('Ready to publish')
    await wrapper.get('#slug').setValue('ready-to-publish')
    await wrapper.get('[data-testid="markdown-editor"]').setValue('# Ready')

    expect(wrapper.get('button[name="publish"]').attributes('disabled')).toBeUndefined()
  })
})
