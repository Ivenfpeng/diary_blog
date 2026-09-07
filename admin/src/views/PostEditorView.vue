<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ArrowLeft, Save } from '@lucide/vue'
import { useRoute, useRouter } from 'vue-router'
import MarkdownEditor from '../components/MarkdownEditor.vue'
import PublishPanel from '../components/PublishPanel.vue'
import RevisionPanel, { type PostRevision } from '../components/RevisionPanel.vue'
import { apiRequest } from '../api/client'
import { createEditorState, type EditablePost } from '../state/editor'

interface TaxonomyOption { id: number; name: string; slug: string }

const route = useRoute()
const router = useRouter()
const revisions = ref<PostRevision[]>([])
const categories = ref<TaxonomyOption[]>([])
const tags = ref<TaxonomyOption[]>([])
const loading = ref(true)
const restoring = ref(false)
const destructiveActionInFlight = ref(false)
const errorMessage = ref('')
const postID = computed(() => Number(route.params.id))
const publishSlugPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/
const maxContentBytes = 2 * 1024 * 1024
const textEncoder = new TextEncoder()

const clientValidationMessage = computed(() => {
  const titleBytes = textEncoder.encode(editor.article.title).length
  const contentBytes = textEncoder.encode(editor.article.content_md).length
  if (editor.article.slug && !publishSlugPattern.test(editor.article.slug)) return 'Slug must use lowercase letters, numbers, and single hyphens.'
  if (/[\r\n]/.test(editor.article.title)) return 'Title must stay on one line.'
  if (titleBytes > 300) return 'Title must be 300 bytes or fewer.'
  if (contentBytes > maxContentBytes) return 'Markdown must be 2 MiB or smaller.'
  return ''
})

const editor = createEditorState(async (article) => {
  if (clientValidationMessage.value) throw new Error(clientValidationMessage.value)
  const response = await apiRequest<{ post: EditablePost }>(`/api/admin/posts/${article.id}`, {
    method: 'PUT',
    body: {
      slug: article.slug,
      title: article.title,
      summary: article.summary,
      content_md: article.content_md,
      category_id: article.category_id,
      cover_media_id: article.cover_media_id ?? null,
      tag_ids: article.tag_ids,
      expected_revision: article.expected_revision,
    },
  })
  return response.post
})

const canPublish = computed(() => Boolean(
  editor.article.title.trim()
  && !/[\r\n]/.test(editor.article.title)
  && textEncoder.encode(editor.article.title).length <= 300
  && publishSlugPattern.test(editor.article.slug)
  && editor.article.content_md.trim()
  && textEncoder.encode(editor.article.content_md).length <= maxContentBytes,
) && !editor.saving.value)
const destructiveActionsLocked = computed(() => editor.dirty.value || editor.saving.value || editor.conflict.value || restoring.value || destructiveActionInFlight.value)
const saveLabel = computed(() => ({ idle: editor.dirty.value ? 'Unsaved changes' : 'Saved', saving: 'Saving…', saved: 'Saved', error: 'Save failed', conflict: 'Conflict detected' }[editor.saveStatus.value]))

function updateField(field: 'title' | 'slug' | 'summary' | 'content_md', value: string): void {
  if (destructiveActionInFlight.value) return
  editor.update({ [field]: value })
}

function updateCategory(value: string): void {
  if (destructiveActionInFlight.value) return
  const id = Number(value)
  editor.update({ category_id: Number.isInteger(id) && id > 0 ? id : null })
}

function updateTag(id: number, checked: boolean): void {
  if (destructiveActionInFlight.value) return
  const selected = new Set(editor.article.tag_ids)
  if (checked) selected.add(id)
  else selected.delete(id)
  editor.update({ tag_ids: tags.value.filter((tag) => selected.has(tag.id)).map((tag) => tag.id) })
}

function articleSnapshot(): string {
  return JSON.stringify({ ...editor.article, tag_ids: editor.article.tag_ids })
}

function hasLocalChangesSince(snapshot: string): boolean {
  return editor.dirty.value || articleSnapshot() !== snapshot
}

async function load(): Promise<void> {
  if (!Number.isInteger(postID.value) || postID.value < 1) {
    errorMessage.value = 'This article could not be found.'
    loading.value = false
    return
  }
  try {
    const [postResponse, revisionResponse, categoryResponse, tagResponse] = await Promise.all([
      apiRequest<{ post: EditablePost }>(`/api/admin/posts/${postID.value}`),
      apiRequest<{ revisions: PostRevision[] }>(`/api/admin/posts/${postID.value}/revisions`),
      apiRequest<{ categories: TaxonomyOption[] }>('/api/admin/categories'),
      apiRequest<{ tags: TaxonomyOption[] }>('/api/admin/tags'),
    ])
    editor.load(postResponse.post)
    revisions.value = revisionResponse.revisions ?? []
    categories.value = categoryResponse.categories ?? []
    tags.value = tagResponse.tags ?? []
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to load this article.'
  } finally {
    loading.value = false
  }
}

async function saveNow(): Promise<void> { await editor.save() }
async function preview(): Promise<void> {
  const response = await apiRequest<{ html: string }>(`/api/admin/posts/${editor.article.id}/preview`, { method: 'POST', body: { content_md: editor.article.content_md } })
  editor.previewHTML.value = response.html
}
async function publish(): Promise<void> {
  if (!canPublish.value || destructiveActionInFlight.value) return
  destructiveActionInFlight.value = true
  try {
    if (editor.dirty.value) await editor.save()
    if (editor.dirty.value || editor.conflict.value) return
    const response = await apiRequest<{ post: EditablePost }>(`/api/admin/posts/${editor.article.id}/publish`, { method: 'POST', body: { expected_revision: editor.revision.value } })
    editor.load(response.post)
  } finally {
    destructiveActionInFlight.value = false
  }
}
async function archive(): Promise<void> {
  if (destructiveActionsLocked.value) return
  const snapshot = articleSnapshot()
  destructiveActionInFlight.value = true
  try {
    const response = await apiRequest<{ post: EditablePost }>(`/api/admin/posts/${editor.article.id}/archive`, { method: 'POST', body: { expected_revision: editor.revision.value } })
    if (!hasLocalChangesSince(snapshot)) editor.load(response.post)
  } finally {
    destructiveActionInFlight.value = false
  }
}
async function restore(revision: PostRevision): Promise<void> {
  if (destructiveActionsLocked.value) return
  const snapshot = articleSnapshot()
  destructiveActionInFlight.value = true
  restoring.value = true
  try {
    const response = await apiRequest<{ post: EditablePost }>(`/api/admin/posts/${editor.article.id}/revisions/${revision.id}/restore`, { method: 'POST', body: { expected_revision: editor.revision.value } })
    if (!hasLocalChangesSince(snapshot)) {
      editor.load(response.post)
      const revisionResponse = await apiRequest<{ revisions: PostRevision[] }>(`/api/admin/posts/${editor.article.id}/revisions`)
      revisions.value = revisionResponse.revisions ?? []
    }
  } finally {
    restoring.value = false
    destructiveActionInFlight.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="editor-view" aria-labelledby="editor-heading">
    <RouterLink :to="{ name: 'posts' }" class="back-link"><ArrowLeft :size="16" aria-hidden="true" /> All posts</RouterLink>
    <div class="editor-heading"><div><p class="eyebrow">Article editor</p><h1 id="editor-heading">{{ editor.article.title || 'Untitled article' }}</h1></div><span class="save-status" :class="editor.saveStatus.value">{{ saveLabel }}</span></div>
    <p v-if="errorMessage" class="form-error" role="alert">{{ errorMessage }}</p>
    <p v-else-if="loading" class="muted">Loading article…</p>
    <template v-else>
      <p v-if="editor.conflict.value" class="form-error" role="alert">This article changed elsewhere. Your local Markdown is preserved; reload before saving again.</p>
      <form class="editor-form" @submit.prevent="saveNow">
        <label>Title <input id="title" :value="editor.article.title" required :disabled="destructiveActionInFlight" @input="updateField('title', ($event.target as HTMLInputElement).value)" /></label>
        <label>Slug <input id="slug" :value="editor.article.slug" required pattern="[a-z0-9\-]+" :disabled="destructiveActionInFlight" @input="updateField('slug', ($event.target as HTMLInputElement).value)" /></label>
        <p v-if="clientValidationMessage" class="form-error wide" role="alert">{{ clientValidationMessage }}</p>
        <label class="wide">Summary <textarea id="summary" :value="editor.article.summary" rows="3" :disabled="destructiveActionInFlight" @input="updateField('summary', ($event.target as HTMLTextAreaElement).value)" /></label>
        <label>Category <select id="category" :value="editor.article.category_id ?? ''" :disabled="destructiveActionInFlight" @change="updateCategory(($event.target as HTMLSelectElement).value)"><option value="">No category</option><option v-for="category in categories" :key="category.id" :value="category.id">{{ category.name }} /{{ category.slug }}</option></select></label>
        <fieldset class="tag-options">
          <legend>Tags</legend>
          <p v-if="tags.length === 0" class="muted">No tags yet. Create tags in Categories &amp; tags first.</p>
          <label v-for="tag in tags" :key="tag.id" class="checkbox-label">
            <input :id="`tag-${tag.id}`" type="checkbox" :checked="editor.article.tag_ids.includes(tag.id)" :disabled="destructiveActionInFlight" @change="updateTag(tag.id, ($event.target as HTMLInputElement).checked)" />
            <span>{{ tag.name }} /{{ tag.slug }}</span>
          </label>
        </fieldset>
        <label class="wide">Markdown <MarkdownEditor :model-value="editor.article.content_md" :disabled="destructiveActionInFlight" @update:model-value="updateField('content_md', $event)" /></label>
        <div class="editor-actions"><button type="submit" class="secondary-button" :disabled="editor.saving.value || destructiveActionInFlight || !editor.dirty.value || Boolean(clientValidationMessage)"><Save :size="16" aria-hidden="true" /> Save now</button><PublishPanel :can-publish="canPublish" :saving="editor.saving.value || destructiveActionInFlight" :actions-locked="destructiveActionsLocked" :status="editor.article.status" @preview="preview" @publish="publish" @archive="archive" /></div>
      </form>
      <section v-if="editor.previewHTML.value" class="preview" aria-labelledby="preview-heading"><h2 id="preview-heading">Preview</h2><div v-html="editor.previewHTML.value" /></section>
      <RevisionPanel :revisions="revisions" :restoring="restoring" :actions-locked="destructiveActionsLocked" @restore="restore" />
    </template>
  </section>
</template>

<style scoped>
.editor-view { max-width: 1060px; }
.back-link { display: inline-flex; align-items: center; gap: 5px; margin-bottom: 20px; color: #285e48; font-size: 14px; font-weight: 650; text-decoration: none; }
.editor-heading { display: flex; justify-content: space-between; align-items: end; gap: 16px; margin-bottom: 22px; }
h1 { margin: 0; font-size: 28px; letter-spacing: -.025em; }
.save-status { color: #637168; font-size: 13px; white-space: nowrap; }.save-status.saving { color: #356b54; }.save-status.conflict, .save-status.error { color: #a33a32; }
.editor-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
label { display: grid; gap: 6px; color: #39463e; font-size: 13px; font-weight: 650; }.wide { grid-column: 1 / -1; }
input, select, textarea { min-width: 0; border: 1px solid #b9c5be; border-radius: 3px; padding: 9px 10px; font: inherit; background: #fff; } textarea { resize: vertical; }
.tag-options { min-width: 0; border: 0; margin: 0; padding: 0; display: grid; gap: 8px; color: #39463e; font-size: 13px; font-weight: 650; }
.tag-options legend { padding: 0; margin-bottom: 2px; }
.checkbox-label { display: flex; align-items: center; gap: 8px; font-weight: 500; }
.checkbox-label input { min-width: auto; padding: 0; }
.editor-actions { grid-column: 1 / -1; display: flex; flex-wrap: wrap; gap: 10px; align-items: center; border-top: 1px solid #d9dfdb; padding-top: 16px; }.secondary-button { min-height: 38px; display: inline-flex; align-items: center; gap: 6px; padding: 0 12px; border: 1px solid #9eaea5; border-radius: 3px; background: #fff; color: #193d2f; font: inherit; font-weight: 650; cursor: pointer; }
.preview { border-top: 1px solid #d9dfdb; margin-top: 26px; padding-top: 20px; }.preview h2 { margin: 0 0 12px; font-size: 18px; }.muted { color: #68756d; }
@media (max-width: 640px) { .editor-form { grid-template-columns: 1fr; }.wide, .editor-actions { grid-column: auto; }.editor-heading { align-items: flex-start; flex-direction: column; } }
</style>
