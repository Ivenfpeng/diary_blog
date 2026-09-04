import { reactive, ref } from 'vue'

export type SaveStatus = 'idle' | 'saving' | 'saved' | 'error' | 'conflict'

export interface EditablePost {
  id: number
  slug: string
  title: string
  summary: string
  content_md: string
  status: string
  category_id: number | null
  tag_ids: number[]
  revision: number
  cover_media_id?: number | null
}

export type SaveArticle = (article: EditablePost & { expected_revision: number }) => Promise<EditablePost>

const emptyArticle = (): EditablePost => ({
  id: 0,
  slug: '',
  title: '',
  summary: '',
  content_md: '',
  status: 'draft',
  category_id: null,
  tag_ids: [],
  revision: 0,
  cover_media_id: null,
})

export function createEditorState(saveArticle: SaveArticle) {
  const article = reactive<EditablePost>(emptyArticle())
  const revision = ref(0)
  const dirty = ref(false)
  const saveStatus = ref<SaveStatus>('idle')
  const conflict = ref(false)
  const previewHTML = ref('')
  const saving = ref(false)
  let timer: ReturnType<typeof setTimeout> | undefined
  let changes = 0
  let queued = false

  function load(post: EditablePost): void {
    clearTimer()
    Object.assign(article, { ...post, tag_ids: [...post.tag_ids] })
    revision.value = post.revision
    dirty.value = false
    conflict.value = false
    saveStatus.value = 'idle'
    changes = 0
    queued = false
  }

  function update(patch: Partial<EditablePost>): void {
    Object.assign(article, patch)
    if (patch.tag_ids) article.tag_ids = [...patch.tag_ids]
    changes += 1
    dirty.value = true
    conflict.value = false
    saveStatus.value = 'idle'
    scheduleSave()
  }

  function scheduleSave(): void {
    clearTimer()
    timer = setTimeout(() => { void save() }, 1500)
  }

  function clearTimer(): void {
    if (timer !== undefined) clearTimeout(timer)
    timer = undefined
  }

  async function save(): Promise<void> {
    clearTimer()
    if (!dirty.value || conflict.value) return
    if (saving.value) {
      queued = true
      return
    }

    const versionAtSave = changes
    const payload = { ...article, tag_ids: [...article.tag_ids], expected_revision: revision.value }
    saving.value = true
    saveStatus.value = 'saving'
    try {
      const saved = await saveArticle(payload)
      revision.value = saved.revision
      article.revision = saved.revision
      if (changes === versionAtSave) {
        Object.assign(article, { ...saved, tag_ids: [...saved.tag_ids] })
        dirty.value = false
        saveStatus.value = 'saved'
      } else {
        dirty.value = true
        saveStatus.value = 'idle'
      }
    } catch (error) {
      if (typeof error === 'object' && error !== null && 'status' in error && error.status === 409) {
        conflict.value = true
        saveStatus.value = 'conflict'
      } else {
        saveStatus.value = 'error'
      }
      dirty.value = true
    } finally {
      saving.value = false
      if (queued && !conflict.value) {
        queued = false
        if (dirty.value) void save()
      }
    }
  }

  return { article, revision, dirty, saveStatus, conflict, previewHTML, saving, load, update, save }
}
