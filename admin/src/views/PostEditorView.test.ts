import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import PostEditorView from './PostEditorView.vue'

const mockedClient = vi.hoisted(() => ({ apiRequest: vi.fn() }))
vi.mock('../api/client', () => mockedClient)
vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: '7' } }),
  useRouter: () => ({ push: vi.fn() }),
}))

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => { resolve = res })
  return { promise, resolve }
}

describe('PostEditorView', () => {
  beforeEach(() => mockedClient.apiRequest.mockReset())
  afterEach(() => vi.useRealTimers())

  it('uses a browser-compatible slug pattern', async () => {
    mockedClient.apiRequest.mockImplementation((path = '') => Promise.resolve(path.endsWith('/revisions')
      ? { revisions: [] }
      : {
          post: {
            id: 7, slug: 'valid-slug', title: 'Ready to publish', summary: '', content_md: '# Ready', status: 'draft',
            category_id: null, tag_ids: [], revision: 3,
          },
        }))
    const wrapper = mount(PostEditorView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          MarkdownEditor: { props: ['modelValue'], template: '<textarea />' },
          PublishPanel: true,
          RevisionPanel: true,
        },
      },
    })
    await vi.waitFor(() => expect(wrapper.find('.editor-form').exists()).toBe(true))

    const pattern = wrapper.get('#slug').attributes('pattern')
    expect(pattern).toBeTruthy()
    if (!pattern) throw new Error('missing slug pattern')
    expect(() => new RegExp(pattern, 'v')).not.toThrow()
    const slugRegExp = new RegExp(`^(?:${pattern})$`, 'v')
    expect(slugRegExp.test('数据库-笔记-2026')).toBe(true)
    expect(slugRegExp.test('bad slug')).toBe(false)
    expect(slugRegExp.test('bad/slug')).toBe(false)
    expect(slugRegExp.test('bad--slug')).toBe(false)
    wrapper.unmount()
  })

  it('saves existing taxonomy selections instead of raw typed ids', async () => {
    const post = {
      id: 7, slug: 'valid-slug', title: 'Ready to publish', summary: '', content_md: '# Ready', status: 'draft',
      category_id: null, tag_ids: [], revision: 3,
    }
    mockedClient.apiRequest.mockImplementation((path = '', options = {}) => {
      if (path === '/api/admin/posts/7' && options.method === 'PUT') {
        return Promise.resolve({ post: { ...post, ...options.body, revision: 4 } })
      }
      if (path === '/api/admin/posts/7') return Promise.resolve({ post })
      if (path.endsWith('/revisions')) return Promise.resolve({ revisions: [] })
      if (path === '/api/admin/categories') return Promise.resolve({ categories: [{ id: 2, name: 'Engineering', slug: 'engineering' }] })
      if (path === '/api/admin/tags') return Promise.resolve({ tags: [{ id: 3, name: 'Go', slug: 'go' }] })
      return Promise.resolve({})
    })
    const wrapper = mount(PostEditorView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          MarkdownEditor: { props: ['modelValue'], template: '<textarea />' },
          PublishPanel: true,
          RevisionPanel: true,
        },
      },
    })
    await vi.waitFor(() => expect(wrapper.find('.editor-form').exists()).toBe(true))

    expect(wrapper.get('#category').element.tagName).toBe('SELECT')
    await wrapper.get('#category').setValue('2')
    await wrapper.get('#tag-3').setValue(true)
    await wrapper.get('form').trigger('submit')

    await vi.waitFor(() => {
      expect(mockedClient.apiRequest).toHaveBeenCalledWith('/api/admin/posts/7', expect.objectContaining({
        method: 'PUT',
        body: expect.objectContaining({ category_id: 2, tag_ids: [3] }),
      }))
    })
    wrapper.unmount()
  })

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

  it('allows Chinese characters in article slugs', async () => {
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

    await wrapper.get('#title').setValue('数据库笔记')
    await wrapper.get('#slug').setValue('数据库-笔记-2026')
    await wrapper.get('[data-testid="markdown-editor"]').setValue('# 数据库笔记')

    expect(wrapper.text()).not.toContain('Slug must')
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
          MarkdownEditor: { props: ['modelValue'], emits: ['update:modelValue'], template: '<textarea data-testid="markdown-editor" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
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
    await wrapper.get('#slug').setValue('valid-slug')
    await wrapper.get('#title').setValue('x'.repeat(301))
    expect(wrapper.get('button[name="publish"]').attributes('disabled')).toBeDefined()
    await wrapper.get('#title').setValue('Ready to publish')
    await wrapper.get('[data-testid="markdown-editor"]').setValue('x'.repeat(2 * 1024 * 1024 + 1))
    expect(wrapper.get('button[name="publish"]').attributes('disabled')).toBeDefined()
    wrapper.findComponent(PublishPanelStub).vm.$emit('publish')
    await wrapper.vm.$nextTick()
    expect(mockedClient.apiRequest.mock.calls.some(([path, options]) => path === '/api/admin/posts/7/publish' || options?.method === 'PUT')).toBe(false)
    wrapper.unmount()
  })

  it('pauses autosave and explains the field error while the slug is invalid', async () => {
    const post = {
      id: 7, slug: 'valid-slug', title: 'Ready to publish', summary: '', content_md: '# Ready', status: 'draft',
      category_id: null, tag_ids: [], revision: 3,
    }
    mockedClient.apiRequest.mockImplementation((path = '') => Promise.resolve(path.endsWith('/revisions') ? { revisions: [] } : { post }))
    const wrapper = mount(PostEditorView, {
      global: {
        stubs: {
          RouterLink: { template: '<a><slot /></a>' },
          MarkdownEditor: { props: ['modelValue'], emits: ['update:modelValue'], template: '<textarea data-testid="markdown-editor" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
          PublishPanel: { props: ['canPublish'], template: '<button name="publish" :disabled="!canPublish">Publish</button>' },
          RevisionPanel: true,
        },
      },
    })
    await vi.waitFor(() => expect(wrapper.find('.editor-form').exists()).toBe(true))

    vi.useFakeTimers()
    await wrapper.get('#slug').setValue('中文 slug')
    await vi.advanceTimersByTimeAsync(1600)

    expect(wrapper.text()).toContain('Slug must use Chinese or other letters, numbers, and single hyphens.')
    expect(mockedClient.apiRequest.mock.calls.some(([path, options]) => path === '/api/admin/posts/7' && options?.method === 'PUT')).toBe(false)
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

    expect(mockedClient.apiRequest.mock.calls.some(([path]) => path === '/api/admin/posts/7/archive' || path === '/api/admin/posts/7/revisions/11/restore')).toBe(false)
    wrapper.unmount()
  })

  it.each([
    ['archive', '/api/admin/posts/7/archive'],
    ['restore', '/api/admin/posts/7/revisions/11/restore'],
  ])('blocks typing while a %s request is pending', async (action, actionPath) => {
    const post = {
      id: 7, slug: 'valid-slug', title: 'Ready to publish', summary: '', content_md: '# Ready', status: 'published',
      category_id: null, tag_ids: [], revision: 3,
    }
    const pending = deferred<{ post: typeof post }>()
    mockedClient.apiRequest.mockImplementation((path = '') => {
      if (path === actionPath) return pending.promise
      if (path.endsWith('/revisions')) return Promise.resolve({ revisions: [{ id: 11, revision: 2, title: 'Earlier', created_at: '' }] })
      return Promise.resolve({ post })
    })
    const MarkdownEditorStub = { name: 'MarkdownEditor', props: ['modelValue', 'disabled'], emits: ['update:modelValue'], template: '<textarea data-testid="markdown-editor" :disabled="disabled" />' }
    const PublishPanelStub = { name: 'PublishPanel', emits: ['archive'], template: '<button name="archive" @click="$emit(\'archive\')">Archive</button>' }
    const RevisionPanelStub = { name: 'RevisionPanel', props: ['revisions'], emits: ['restore'], template: '<button name="restore" @click="$emit(\'restore\', revisions[0])">Restore</button>' }
    const wrapper = mount(PostEditorView, {
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' }, MarkdownEditor: MarkdownEditorStub, PublishPanel: PublishPanelStub, RevisionPanel: RevisionPanelStub } },
    })
    await vi.waitFor(() => expect(wrapper.find('.editor-form').exists()).toBe(true))

    if (action === 'archive') wrapper.findComponent(PublishPanelStub).vm.$emit('archive')
    else wrapper.findComponent(RevisionPanelStub).vm.$emit('restore', { id: 11, revision: 2, title: 'Earlier', created_at: '' })
    await wrapper.vm.$nextTick()

    expect(wrapper.get('#title').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="markdown-editor"]').attributes('disabled')).toBeDefined()
    await wrapper.get('#title').setValue('Attempted local edit')
    wrapper.findComponent(MarkdownEditorStub).vm.$emit('update:modelValue', '# Attempted local Markdown')
    await wrapper.vm.$nextTick()
    expect(wrapper.get('#editor-heading').text()).toBe('Ready to publish')
    expect(mockedClient.apiRequest.mock.calls.filter(([path]) => path === actionPath)).toHaveLength(1)

    pending.resolve({ post: { ...post, status: action === 'archive' ? 'archived' : 'published', revision: 4 } })
    await vi.waitFor(() => expect(wrapper.get('#title').attributes('disabled')).toBeUndefined())
    wrapper.unmount()
  })

  it('blocks edits while publication is pending so the response cannot discard newer input', async () => {
    const post = {
      id: 7, slug: 'valid-slug', title: 'Ready to publish', summary: '', content_md: '# Ready', status: 'draft',
      category_id: null, tag_ids: [], revision: 3,
    }
    const pending = deferred<{ post: typeof post }>()
    mockedClient.apiRequest.mockImplementation((path = '') => {
      if (path === '/api/admin/posts/7/publish') return pending.promise
      if (path.endsWith('/revisions')) return Promise.resolve({ revisions: [] })
      return Promise.resolve({ post })
    })
    const MarkdownEditorStub = { name: 'MarkdownEditor', props: ['modelValue', 'disabled'], emits: ['update:modelValue'], template: '<textarea data-testid="markdown-editor" :disabled="disabled" />' }
    const PublishPanelStub = { name: 'PublishPanel', props: ['canPublish', 'saving'], emits: ['publish'], template: '<button name="publish" :disabled="!canPublish || saving" @click="$emit(\'publish\')">Publish</button>' }
    const wrapper = mount(PostEditorView, {
      global: { stubs: { RouterLink: { template: '<a><slot /></a>' }, MarkdownEditor: MarkdownEditorStub, PublishPanel: PublishPanelStub, RevisionPanel: true } },
    })
    await vi.waitFor(() => expect(wrapper.find('.editor-form').exists()).toBe(true))

    wrapper.findComponent(PublishPanelStub).vm.$emit('publish')
    await wrapper.vm.$nextTick()
    expect(wrapper.get('#title').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="markdown-editor"]').attributes('disabled')).toBeDefined()
    await wrapper.get('#title').setValue('Typed during publish')
    wrapper.findComponent(MarkdownEditorStub).vm.$emit('update:modelValue', '# Typed during publish')
    await wrapper.vm.$nextTick()
    expect(wrapper.get('#editor-heading').text()).toBe('Ready to publish')

    pending.resolve({ post: { ...post, status: 'published', revision: 4 } })
    await vi.waitFor(() => expect(wrapper.get('#title').attributes('disabled')).toBeUndefined())
    expect(wrapper.get('#editor-heading').text()).toBe('Ready to publish')
    wrapper.unmount()
  })
})
