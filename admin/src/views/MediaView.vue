<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Copy, Search, Trash2 } from '@lucide/vue'
import { apiRequest } from '../api/client'
import { notifyError, notifySuccess } from '../state/notifications'

interface Media {
  id: number
  path: string
  mime_type: string
  width: number
  height: number
  size: number
  alt_text: string
}

const media = ref<Media[]>([])
const selectedFile = ref<File | null>(null)
const altText = ref('')
const altDrafts = ref<Record<number, string>>({})
const errorMessage = ref('')
const statusMessage = ref('')
const uploading = ref(false)
const savingAltID = ref<number | null>(null)
const query = ref('')
const filteredMedia = computed(() => {
  const term = query.value.trim().toLowerCase()
  if (!term) return media.value
  return media.value.filter((item) => `${item.alt_text} ${item.path} ${item.mime_type}`.toLowerCase().includes(term))
})

async function load(): Promise<void> {
  try {
    media.value = (await apiRequest<{ media: Media[] }>('/api/admin/media')).media
    altDrafts.value = Object.fromEntries(media.value.map((item) => [item.id, item.alt_text]))
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to load media.'
    notifyError('Media failed to load', errorMessage.value)
  }
}

function selectFile(event: Event): void {
  selectedFile.value = (event.target as HTMLInputElement).files?.[0] ?? null
  errorMessage.value = ''
  statusMessage.value = ''
}

async function upload(): Promise<void> {
  errorMessage.value = ''
  statusMessage.value = ''
  if (!selectedFile.value) {
    errorMessage.value = 'Choose an image to upload.'
    notifyError('Image upload failed', errorMessage.value)
    return
  }
  if (!altText.value.trim()) {
    errorMessage.value = 'Alt text is required.'
    notifyError('Image upload failed', errorMessage.value)
    return
  }
  const body = new FormData()
  body.set('file', selectedFile.value)
  body.set('alt_text', altText.value.trim())
  uploading.value = true
  statusMessage.value = 'Uploading…'
  try {
    const response = await apiRequest<{ media: Media }>('/api/admin/media', { method: 'POST', body })
    media.value.unshift(response.media)
    altDrafts.value[response.media.id] = response.media.alt_text
    altText.value = ''
    statusMessage.value = 'Upload complete.'
    notifySuccess('Image uploaded', `${response.media.alt_text} is now available in the media library.`)
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Upload failed'
    statusMessage.value = ''
    notifyError('Image upload failed', errorMessage.value)
  } finally {
    uploading.value = false
  }
}

async function saveAltText(item: Media): Promise<void> {
  const nextAltText = (altDrafts.value[item.id] ?? '').trim()
  if (!nextAltText) {
    errorMessage.value = 'Alt text is required.'
    notifyError('Alt text save failed', errorMessage.value)
    return
  }
  savingAltID.value = item.id
  errorMessage.value = ''
  try {
    const response = await apiRequest<{ media: Media }>(`/api/admin/media/${item.id}`, {
      method: 'PATCH',
      body: { alt_text: nextAltText },
    })
    const index = media.value.findIndex((entry) => entry.id === item.id)
    if (index >= 0) media.value[index] = response.media
    altDrafts.value[item.id] = response.media.alt_text
    notifySuccess('Alt text saved', 'The media description was updated.')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to save alt text.'
    notifyError('Alt text save failed', errorMessage.value)
  } finally {
    savingAltID.value = null
  }
}

async function copyURL(item: Media): Promise<void> {
  const url = new URL(`/media/${item.path}`, window.location.origin).toString()
  try {
    await navigator.clipboard.writeText(url)
    notifySuccess('URL copied', 'The public media URL is ready to paste.')
  } catch (error) {
    notifyError('Copy failed', error instanceof Error ? error.message : 'Clipboard access is unavailable.')
  }
}

async function deleteMedia(item: Media): Promise<void> {
  if (!window.confirm(`Delete ${item.alt_text || 'this image'} permanently?`)) return
  try {
    await apiRequest(`/api/admin/media/${item.id}`, { method: 'DELETE' })
    media.value = media.value.filter((entry) => entry.id !== item.id)
    notifySuccess('Image deleted', 'The media file was removed from the library.')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to delete image.'
    notifyError('Image delete failed', errorMessage.value)
  }
}

onMounted(load)
</script>

<template>
  <section class="media-view" aria-labelledby="media-heading">
    <div class="view-heading"><div><p class="eyebrow">Library / Assets</p><h1 id="media-heading">Media</h1><p class="view-lede">Upload, review and reuse the images that give your articles a visual rhythm.</p></div><span class="media-count">{{ media.length }} assets</span></div>
    <form class="upload-form" @submit.prevent="upload">
      <label>Image <input id="media-file" type="file" accept="image/jpeg,image/png,image/webp,image/gif,image/avif" @change="selectFile" /></label>
      <label>Alt text <input id="alt-text" v-model="altText" required placeholder="Describe the image" /></label>
      <button class="primary-button" :disabled="uploading">{{ uploading ? 'Uploading…' : 'Upload image' }}</button>
    </form>
    <p v-if="errorMessage" class="form-error" role="alert">{{ errorMessage }}</p>
    <p v-if="statusMessage" class="muted">{{ statusMessage }}</p>
    <div class="media-toolbar"><label class="media-search"><Search :size="16" aria-hidden="true" /><span class="sr-only">Search media</span><input v-model="query" type="search" placeholder="Search alt text, filename or type" /></label><span class="muted">Showing {{ filteredMedia.length }} of {{ media.length }}</span></div>
    <div class="media-list">
      <article v-for="item in filteredMedia" :key="item.id" class="media-row">
        <img :src="`/media/${item.path}`" :alt="item.alt_text" />
        <div><strong>{{ item.alt_text || 'Untitled image' }}</strong><small>{{ item.mime_type }} · {{ item.width }}×{{ item.height }}</small></div>
        <label class="alt-editor">Alt text <input :id="`media-alt-${item.id}`" v-model="altDrafts[item.id]" /></label>
        <div class="media-actions"><button type="button" class="secondary-button" :data-testid="`save-alt-${item.id}`" :disabled="savingAltID === item.id" @click="saveAltText(item)">Save alt text</button><button type="button" class="icon-action" :aria-label="`Copy URL for ${item.alt_text}`" @click="copyURL(item)"><Copy :size="15" aria-hidden="true" /> Copy URL</button><button type="button" class="icon-action danger-action" :aria-label="`Delete ${item.alt_text}`" @click="deleteMedia(item)"><Trash2 :size="15" aria-hidden="true" /> Delete</button></div>
      </article>
      <p v-if="!filteredMedia.length" class="muted">{{ media.length ? 'No media matches this search.' : 'No media uploaded yet.' }}</p>
    </div>
  </section>
</template>

<style scoped>
.media-view { width: 100%; display: grid; gap: 18px; }
h1 { margin: 0; font-size: clamp(30px, 3vw, 44px); letter-spacing: -.045em; }
.view-lede { max-width: 560px; margin: 8px 0 0; color: #667085; font-size: 14px; }
.media-count { align-self: center; padding: 8px 11px; border: 1px solid #dbe3ed; border-radius: 999px; color: #667085; background: #fff; font-size: 12px; font-weight: 700; }
.view-heading { margin-bottom: 0; }
.upload-form { display: flex; align-items: end; gap: 12px; flex-wrap: wrap; padding: 18px; border: 1px solid rgb(202 215 205 / 80%); border-radius: 22px; background: var(--admin-panel); box-shadow: var(--admin-shadow); backdrop-filter: blur(18px); }
label { display: grid; gap: 6px; color: #39463e; font-size: 13px; font-weight: 650; }
input { min-height: 42px; padding: 0 12px; }
.media-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 16px; }
.media-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.media-search { display: flex; flex: 1; align-items: center; gap: 8px; max-width: 520px; padding: 0 12px; border: 1px solid #d0d5dd; border-radius: 10px; color: #98a2b3; background: #fff; }
.media-search input { min-height: 40px; border: 0; padding-left: 0; outline: 0; box-shadow: none; }
.media-row { display: grid; grid-template-columns: 92px 1fr; gap: 12px; align-items: center; min-width: 0; padding: 16px; border: 1px solid #e2e8f0; border-radius: 16px; background: var(--admin-panel-strong); box-shadow: 0 12px 28px rgb(15 35 68 / 6%); }
.media-row img { width: 92px; height: 68px; object-fit: cover; border-radius: 14px; background: #edf0ee; }
.media-row small { display: block; color: #68756d; margin-top: 4px; }
.alt-editor, .media-actions { grid-column: 1 / -1; }
.secondary-button { min-height: 38px; justify-self: start; border: 1px solid #9eaea5; border-radius: 12px; padding: 0 12px; background: #fff; color: #193d2f; font: inherit; font-weight: 650; cursor: pointer; }
.media-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.icon-action { display: inline-flex; align-items: center; gap: 6px; min-height: 38px; border: 0; color: #1677ff; background: transparent; font: inherit; font-size: 12px; font-weight: 750; cursor: pointer; }
.icon-action:hover { color: #0958d9; }
.danger-action { color: #cf1322; }.danger-action:hover { color: #a8071a; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }
.muted { color: #68756d; }
@media (max-width: 600px) { .media-toolbar { align-items: stretch; flex-direction: column; }.media-search { max-width: none; }.media-count { align-self: start; } }
</style>
