<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiRequest } from '../api/client'
import { notifyError, notifySuccess } from '../state/notifications'

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
    notifyError('Settings failed to load', errorMessage.value)
  }
}
async function save(): Promise<void> {
  errorMessage.value = ''
  saved.value = false
  try {
    const response = await apiRequest<{ settings: Settings }>('/api/admin/settings', { method: 'PUT', body: settings.value })
    settings.value = { ...settings.value, ...response.settings }
    saved.value = true
    notifySuccess('Settings saved', 'Site settings are ready for visitors.')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to save settings.'
    notifyError('Settings save failed', errorMessage.value)
  }
}
onMounted(load)
</script>
<template>
  <section class="settings-view" aria-labelledby="settings-heading">
    <header class="settings-heading">
      <div>
        <p class="eyebrow">Workspace / Identity</p>
        <h1 id="settings-heading">Site settings</h1>
        <p class="settings-lede">Shape the voice of your public journal. These details appear in the header, feeds and search previews.</p>
      </div>
      <span class="save-state" :class="{ saved }"><span class="state-dot" />{{ saved ? 'Saved just now' : 'Unsaved changes' }}</span>
    </header>
    <div class="settings-grid">
      <form class="settings-card" @submit.prevent="save">
        <div class="card-heading"><div><p class="eyebrow">Public identity</p><h2>How readers meet you</h2></div><span class="card-index">01</span></div>
        <label><span>Site title <small>Shown in the browser title and wordmark.</small></span><input v-model="settings.site_title" required maxlength="120" /></label>
        <label><span>Description <small>A short sentence for your homepage and SEO.</small></span><textarea v-model="settings.description" maxlength="500" rows="5" /></label>
        <label><span>Author <small>Your name on the public journal.</small></span><input v-model="settings.author" maxlength="120" /></label>
        <div class="form-footer"><span>{{ settings.description.length }}/500 characters</span><button class="primary-button">Save settings <span aria-hidden="true">→</span></button></div>
      </form>
      <aside class="settings-preview" aria-label="Site preview">
        <div class="preview-top"><p class="eyebrow">Live preview</p><span class="preview-dot">● Online</span></div>
        <div class="preview-browser"><span /><span /><span /></div>
        <div class="preview-body"><div class="preview-mark">D</div><p class="preview-kicker">Personal knowledge stream</p><h2>{{ settings.site_title || 'Your site title' }}</h2><p>{{ settings.description || 'Your site description will appear here.' }}</p><div class="preview-author">Written by <strong>{{ settings.author || 'Your name' }}</strong></div></div>
      </aside>
    </div>
    <p v-if="errorMessage" class="form-error" role="alert">{{ errorMessage }}</p>
  </section>
</template>
<style scoped>
.settings-view { width: min(100%, 1180px); display: grid; gap: 28px; }
.settings-heading { display: flex; align-items: end; justify-content: space-between; gap: 24px; }
h1 { margin: 0; color: #12233f; font-size: clamp(32px, 4vw, 54px); letter-spacing: -.06em; }
.settings-lede { max-width: 620px; margin: 12px 0 0; color: #667085; font-size: 14px; line-height: 1.7; }
.save-state { display: inline-flex; flex: 0 0 auto; align-items: center; gap: 8px; padding: 8px 12px; border: 1px solid #dbe3ed; border-radius: 999px; color: #667085; background: #fff; font-size: 12px; font-weight: 700; }
.state-dot { width: 7px; height: 7px; border-radius: 50%; background: #faad14; box-shadow: 0 0 0 4px #fff7e6; }
.save-state.saved { color: #389e0d; border-color: #b7eb8f; background: #f6ffed; }
.save-state.saved .state-dot { background: #52c41a; box-shadow: 0 0 0 4px #f6ffed; }
.settings-grid { display: grid; grid-template-columns: minmax(0, 1.35fr) minmax(280px, .65fr); gap: 18px; align-items: start; }
.settings-card, .settings-preview { border: 1px solid #e2e8f0; border-radius: 20px; background: #fff; box-shadow: 0 16px 45px rgb(15 35 68 / 7%); }
.settings-card { display: grid; gap: 20px; padding: clamp(22px, 3vw, 34px); }
.card-heading { display: flex; align-items: start; justify-content: space-between; gap: 12px; padding-bottom: 18px; border-bottom: 1px solid #eef2f7; }
.card-heading h2 { margin: 0; color: #172b4d; font-size: 20px; letter-spacing: -.03em; }
.card-index { color: #91a0b5; font-family: ui-monospace, monospace; font-size: 12px; font-weight: 800; }
label { display: grid; gap: 8px; color: #344054; font-size: 13px; font-weight: 750; }
label span { display: grid; gap: 3px; }
label small { color: #98a2b3; font-size: 11px; font-weight: 500; }
input, textarea { width: 100%; padding: 12px 13px; color: #172b4d; }
textarea { resize: vertical; line-height: 1.6; }
.form-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding-top: 4px; color: #98a2b3; font-size: 11px; }
.primary-button { margin-top: 0; padding: 0 18px; }
.primary-button span { margin-left: 6px; font-size: 16px; }
.settings-preview { position: sticky; top: 22px; overflow: hidden; }
.preview-top { display: flex; align-items: center; justify-content: space-between; padding: 18px 18px 12px; }
.preview-dot { color: #389e0d; font-size: 11px; font-weight: 700; }
.preview-browser { display: flex; gap: 5px; padding: 0 18px 14px; border-bottom: 1px solid #eef2f7; }
.preview-browser span { width: 7px; height: 7px; border-radius: 50%; background: #d0d5dd; }
.preview-body { min-height: 300px; padding: 28px 24px; background: linear-gradient(145deg, #f5f9ff, #fff 55%, #f0fffe); }
.preview-mark { display: grid; width: 38px; height: 38px; place-items: center; margin-bottom: 28px; border-radius: 12px; color: #fff; background: linear-gradient(135deg, #1677ff, #13c2c2); font-weight: 800; box-shadow: 0 10px 18px #1677ff2b; }
.preview-kicker { margin: 0 0 8px; color: #1677ff; font-family: ui-monospace, monospace; font-size: 10px; font-weight: 800; letter-spacing: .1em; text-transform: uppercase; }
.preview-body h2 { margin: 0 0 10px; color: #12233f; font-size: 28px; letter-spacing: -.05em; }
.preview-body > p:not(.preview-kicker) { margin: 0; color: #667085; font-size: 13px; line-height: 1.6; }
.preview-author { margin-top: 34px; padding-top: 14px; border-top: 1px solid #dbe3ed; color: #98a2b3; font-size: 11px; }
.preview-author strong { color: #344054; }
.form-error { margin: 0; }
@media (max-width: 860px) { .settings-grid { grid-template-columns: 1fr; }.settings-preview { position: static; }.preview-body { min-height: 220px; } }
@media (max-width: 560px) { .settings-heading { align-items: start; flex-direction: column; }.form-footer { align-items: stretch; flex-direction: column; }.form-footer .primary-button { width: 100%; } }
</style>
