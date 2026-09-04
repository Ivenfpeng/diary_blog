import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createEditorState, type EditablePost } from './editor'

const article: EditablePost = {
  id: 7,
  slug: 'first-draft',
  title: 'First draft',
  summary: 'A short note.',
  content_md: '# First draft',
  status: 'draft',
  category_id: null,
  tag_ids: [],
  revision: 3,
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

describe('article editor state', () => {
  beforeEach(() => vi.useFakeTimers())

  it('autosaves a changed article after 1.5 seconds', async () => {
    const save = vi.fn().mockResolvedValue({ ...article, revision: 4, content_md: '# Changed' })
    const editor = createEditorState(save)
    editor.load(article)

    editor.update({ content_md: '# Changed' })
    await vi.advanceTimersByTimeAsync(1499)
    expect(save).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(1)
    expect(save).toHaveBeenCalledWith(expect.objectContaining({ content_md: '# Changed', expected_revision: 3 }))
    expect(editor.revision.value).toBe(4)
    expect(editor.dirty.value).toBe(false)
  })

  it('serializes saves and queues the newest local change', async () => {
    const first = deferred<EditablePost>()
    const save = vi.fn().mockReturnValueOnce(first.promise).mockResolvedValueOnce({ ...article, revision: 5, content_md: 'newest' })
    const editor = createEditorState(save)
    editor.load(article)

    editor.update({ content_md: 'first' })
    await vi.advanceTimersByTimeAsync(1500)
    expect(save).toHaveBeenCalledTimes(1)

    editor.update({ content_md: 'newest' })
    await vi.advanceTimersByTimeAsync(1500)
    expect(save).toHaveBeenCalledTimes(1)

    first.resolve({ ...article, revision: 4, content_md: 'first' })
    await vi.runAllTimersAsync()
    expect(save).toHaveBeenCalledTimes(2)
    expect(save).toHaveBeenLastCalledWith(expect.objectContaining({ content_md: 'newest', expected_revision: 4 }))
    expect(editor.revision.value).toBe(5)
    expect(editor.article.content_md).toBe('newest')
  })

  it('keeps local Markdown and enters conflict state on a 409 save', async () => {
    const save = vi.fn().mockRejectedValue({ status: 409 })
    const editor = createEditorState(save)
    editor.load(article)

    editor.update({ content_md: '# My local work' })
    await vi.advanceTimersByTimeAsync(1500)

    expect(editor.article.content_md).toBe('# My local work')
    expect(editor.conflict.value).toBe(true)
    expect(editor.dirty.value).toBe(true)
    expect(editor.saveStatus.value).toBe('conflict')
  })
})
