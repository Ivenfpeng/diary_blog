<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiRequest } from '../api/client'

interface Settings {
  site_title: string
  description: string
  author: string
  navigation: unknown[]
  social_links: Record<string, unknown>
  seo_defaults: Record<string, unknown>
}

const settings = ref<Settings>({
  site_title: '', description: '', author: '', navigation: [], social_links: {}, seo_defaults: {},
})
const errorMessage = ref('')
const saved = ref(false)

async function load(): Promise<void> {
  try {
    settings.value = { ...settings.value, ...(await apiRequest<{ settings: Settings }>('/api/admin/settings')).settings }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to load settings.'
  }
}
async function save(): Promise<void> {
  errorMessage.value = ''
  saved.value = false
  try {
    const response = await apiRequest<{ settings: Settings }>('/api/admin/settings', { method: 'PUT', body: settings.value })
    settings.value = { ...settings.value, ...response.settings }
    saved.value = true
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to save settings.'
  }
}
onMounted(load)
</script>
<template>
  <section class="settings-view">
    <p class="eyebrow">Identity</p>
    <h1>Site settings</h1>
    <form @submit.prevent="save">
      <label>Site title <input v-model="settings.site_title" required maxlength="120" /></label>
      <label>Description <textarea v-model="settings.description" maxlength="500" rows="4" /></label>
      <label>Author <input v-model="settings.author" maxlength="120" /></label>
      <button class="primary-button">Save settings</button>
    </form>
    <p v-if="errorMessage" class="form-error" role="alert">{{ errorMessage }}</p>
    <p v-if="saved" class="muted">Settings saved.</p>
  </section>
</template>
<style scoped>
.settings-view { max-width: 680px; }
h1 { margin: 0 0 20px; font-size: 28px; }
form { display: grid; gap: 16px; }
label { display: grid; gap: 6px; font-size: 13px; font-weight: 650; }
input, textarea { border: 1px solid #b9c5be; border-radius: 3px; padding: 9px 10px; font: inherit; }
.muted { color: #68756d; }
</style>
