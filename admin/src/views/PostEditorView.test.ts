import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PostEditorView from './PostEditorView.vue'

const mockedClient = vi.hoisted(() => ({ apiRequest: vi.fn() }))
vi.mock('../api/client', () => mockedClient)
vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '7' } }),
  useRouter: () => ({ push: vi.fn() }),
}))

describe('PostEditorView', () => {
  beforeEach(() => mockedClient.apiRequest.mockReset())

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
    wrapper.unmount()
  })

  it('keeps publishing disabled for slugs rejected by the server', async () => {
    mockedClient.apiRequest.mockResolvedValue({
      post: {
        id: 7, slug: 'valid-slug', title: 'Ready to publish', summary: '', content_md: '# Ready', status: 'draft',
        category_id: null, tag_ids: [], revision: 3,
      },
    })
    const PublishPanelStub = {
      name: 'PublishPanel', props: ['canPublish'], emits: ['publish'],
      template: '<button name="publish" :disabled="!canPublish" @click="$emit(\'publish\')">Publish</button>',
    }
    const wrapper = mount(PostEditorView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          MarkdownEditor: { props: ['modelValue'], template: '<textarea data-testid="markdown-editor" />' },
          PublishPanel: PublishPanelStub,
          RevisionPanel: true,
        },
      },
    })
    await vi.waitFor(() => expect(wrapper.find('.editor-form').exists()).toBe(true))

    await wrapper.get('#slug').setValue('Bad Slug')
    expect(wrapper.get('button[name="publish"]').attributes('disabled')).toBeDefined()
    await wrapper.get('#slug').setValue('-bad')
    expect(wrapper.get('button[name="publish"]').attributes('disabled')).toBeDefined()
    wrapper.findComponent(PublishPanelStub).vm.$emit('publish')
    await wrapper.vm.$nextTick()
    expect(mockedClient.apiRequest).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('does not let dirty local edits archive or restore an article', async () => {
    const post = {
      id: 7, slug: 'valid-slug', title: 'Ready to publish', summary: '', content_md: '# Ready', status: 'published',
      category_id: null, tag_ids: [], revision: 3,
    }
    mockedClient.apiRequest.mockImplementation((path = '') => Promise.resolve(path.endsWith('/revisions')
      ? { revisions: [{ id: 11, revision: 2, title: 'Earlier', created_at: '2026-09-03T00:00:00Z' }] }
      : { post }))
    const PublishPanelStub = {
      name: 'PublishPanel', props: ['canPublish', 'saving', 'actionsLocked'], emits: ['archive'],
      template: '<button name="archive" :disabled="saving || actionsLocked" @click="$emit(\'archive\')">Archive</button>',
    }
    const RevisionPanelStub = {
      name: 'RevisionPanel', props: ['revisions', 'restoring', 'actionsLocked'], emits: ['restore'],
      template: '<button name="restore" :disabled="restoring || actionsLocked" @click="$emit(\'restore\', revisions[0])">Restore</button>',
    }
    const wrapper = mount(PostEditorView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          MarkdownEditor: { props: ['modelValue'], template: '<textarea data-testid="markdown-editor" />' },
          PublishPanel: PublishPanelStub,
          RevisionPanel: RevisionPanelStub,
        },
      },
    })
    await vi.waitFor(() => expect(wrapper.find('.editor-form').exists()).toBe(true))
    await wrapper.get('#title').setValue('Unsaved local title')

    expect(wrapper.get('button[name="archive"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('button[name="restore"]').attributes('disabled')).toBeDefined()
    wrapper.findComponent(PublishPanelStub).vm.$emit('archive')
    wrapper.findComponent(RevisionPanelStub).vm.$emit('restore', { id: 11, revision: 2, title: 'Earlier', created_at: '' })
    await wrapper.vm.$nextTick()

    expect(mockedClient.apiRequest).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
})
