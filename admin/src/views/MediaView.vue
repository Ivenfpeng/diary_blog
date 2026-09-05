<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiRequest } from '../api/client'

interface Media { id: number; path: string; mime_type: string; width: number; height: number; size: number; alt_text: string }
const media = ref<Media[]>([])
const selectedFile = ref<File | null>(null)
const altText = ref('')
const errorMessage = ref('')
const statusMessage = ref('')
const uploading = ref(false)
async function load(): Promise<void> { try { media.value = (await apiRequest<{ media: Media[] }>('/api/admin/media')).media } catch (error) { errorMessage.value = error instanceof Error ? error.message : 'Unable to load media.' } }
function selectFile(event: Event): void { selectedFile.value = (event.target as HTMLInputElement).files?.[0] ?? null; errorMessage.value = ''; statusMessage.value = '' }
async function upload(): Promise<void> { errorMessage.value = ''; statusMessage.value = ''; if (!selectedFile.value) { errorMessage.value = 'Choose an image to upload.'; return }; if (!altText.value.trim()) { errorMessage.value = 'Alt text is required.'; return }; const body = new FormData(); body.set('file', selectedFile.value); body.set('alt_text', altText.value.trim()); uploading.value = true; statusMessage.value = 'Uploading…'; try { const response = await apiRequest<{ media: Media }>('/api/admin/media', { method: 'POST', body }); media.value.unshift(response.media); altText.value = ''; statusMessage.value = 'Upload complete.' } catch (error) { errorMessage.value = error instanceof Error ? error.message : 'Upload failed'; statusMessage.value = '' } finally { uploading.value = false } }
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
    <p v-if="errorMessage" class="form-error" role="alert">{{ errorMessage }}</p><p v-if="statusMessage" class="muted">{{ statusMessage }}</p>
    <div class="media-list"><article v-for="item in media" :key="item.id" class="media-row"><img :src="`/media/${item.path}`" :alt="item.alt_text" /><span><strong>{{ item.alt_text || 'Untitled image' }}</strong><small>{{ item.mime_type }} · {{ item.width }}×{{ item.height }}</small></span></article><p v-if="!media.length" class="muted">No media uploaded yet.</p></div>
  </section>
</template>

<style scoped>
.media-view { max-width: 960px; } h1 { margin: 0; font-size: 28px; letter-spacing: -.025em; }.view-heading { margin-bottom: 22px; }.upload-form { display: flex; align-items: end; gap: 12px; flex-wrap: wrap; padding-bottom: 16px; border-bottom: 1px solid #d9dfdb; } label { display: grid; gap: 6px; color: #39463e; font-size: 13px; font-weight: 650; } input { min-height: 40px; border: 1px solid #b9c5be; border-radius: 3px; padding: 0 10px; font: inherit; }.media-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 14px; padding-top: 18px; }.media-row { display: grid; grid-template-columns: 80px 1fr; gap: 10px; align-items: center; min-width: 0; }.media-row img { width: 80px; height: 58px; object-fit: cover; background: #edf0ee; }.media-row small { display:block; color:#68756d; margin-top:4px; }.muted { color:#68756d; }
</style>
