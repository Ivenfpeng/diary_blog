<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiRequest } from '../api/client'

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

async function load(): Promise<void> {
  try {
    media.value = (await apiRequest<{ media: Media[] }>('/api/admin/media')).media
    altDrafts.value = Object.fromEntries(media.value.map((item) => [item.id, item.alt_text]))
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to load media.'
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
    return
  }
  if (!altText.value.trim()) {
    errorMessage.value = 'Alt text is required.'
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
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Upload failed'
    statusMessage.value = ''
  } finally {
    uploading.value = false
  }
}

async function saveAltText(item: Media): Promise<void> {
  const nextAltText = (altDrafts.value[item.id] ?? '').trim()
  if (!nextAltText) {
    errorMessage.value = 'Alt text is required.'
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
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to save alt text.'
  } finally {
    savingAltID.value = null
  }
}

onMounted(load)
</script>

<template>
  <section class="media-view" aria-labelledby="media-heading">
    <div class="view-heading"><div><p class="eyebrow">Library</p><h1 id="media-heading">Media</h1></div></div>
    <form class="upload-form" @submit.prevent="upload">
      <label>Image <input id="media-file" type="file" accept="image/jpeg,image/png,image/webp,image/gif,image/avif" @change="selectFile" /></label>
      <label>Alt text <input id="alt-text" v-model="altText" required placeholder="Describe the image" /></label>
      <button class="primary-button" :disabled="uploading">{{ uploading ? 'Uploading…' : 'Upload image' }}</button>
    </form>
    <p v-if="errorMessage" class="form-error" role="alert">{{ errorMessage }}</p>
    <p v-if="statusMessage" class="muted">{{ statusMessage }}</p>
    <div class="media-list">
      <article v-for="item in media" :key="item.id" class="media-row">
        <img :src="`/media/${item.path}`" :alt="item.alt_text" />
        <div><strong>{{ item.alt_text || 'Untitled image' }}</strong><small>{{ item.mime_type }} · {{ item.width }}×{{ item.height }}</small></div>
        <label class="alt-editor">Alt text <input :id="`media-alt-${item.id}`" v-model="altDrafts[item.id]" /></label>
        <button type="button" class="secondary-button" :data-testid="`save-alt-${item.id}`" :disabled="savingAltID === item.id" @click="saveAltText(item)">Save alt text</button>
      </article>
      <p v-if="!media.length" class="muted">No media uploaded yet.</p>
    </div>
  </section>
</template>

<style scoped>
.media-view { max-width: 960px; }
h1 { margin: 0; font-size: 28px; letter-spacing: -.025em; }
.view-heading { margin-bottom: 22px; }
.upload-form { display: flex; align-items: end; gap: 12px; flex-wrap: wrap; padding-bottom: 16px; border-bottom: 1px solid #d9dfdb; }
label { display: grid; gap: 6px; color: #39463e; font-size: 13px; font-weight: 650; }
input { min-height: 40px; border: 1px solid #b9c5be; border-radius: 3px; padding: 0 10px; font: inherit; }
.media-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 14px; padding-top: 18px; }
.media-row { display: grid; grid-template-columns: 80px 1fr; gap: 10px; align-items: center; min-width: 0; }
.media-row img { width: 80px; height: 58px; object-fit: cover; background: #edf0ee; }
.media-row small { display: block; color: #68756d; margin-top: 4px; }
.alt-editor, .secondary-button { grid-column: 1 / -1; }
.secondary-button { min-height: 36px; justify-self: start; border: 1px solid #9eaea5; border-radius: 3px; padding: 0 10px; background: #fff; color: #193d2f; font: inherit; font-weight: 650; cursor: pointer; }
.muted { color: #68756d; }
</style>
