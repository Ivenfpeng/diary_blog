<script setup lang="ts">
import { Archive, Eye, Send } from '@lucide/vue'

defineProps<{ canPublish: boolean; saving: boolean; actionsLocked: boolean; status: string }>()
const emit = defineEmits<{ preview: []; publish: []; archive: [] }>()
</script>

<template>
  <section class="publish-actions" aria-label="Publication controls">
    <span class="status-label">{{ status }}</span>
    <button type="button" class="secondary-button" @click="emit('preview')">
      <Eye :size="16" aria-hidden="true" /> Preview
    </button>
    <button name="publish" type="button" class="primary-button" :disabled="!canPublish || saving" @click="emit('publish')">
      <Send :size="16" aria-hidden="true" /> Publish
    </button>
    <button type="button" class="danger-button" :disabled="saving || actionsLocked" @click="emit('archive')">
      <Archive :size="16" aria-hidden="true" /> Archive
    </button>
  </section>
</template>

<style scoped>
.publish-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; width: 100%; }
.status-label { margin-right: auto; color: #5e6b64; font-size: 13px; text-transform: capitalize; }
button { min-height: 42px; display: inline-flex; align-items: center; gap: 7px; padding: 0 14px; border-radius: 13px; font: inherit; font-weight: 750; cursor: pointer; }
.primary-button { margin: 0; }
.secondary-button { border: 1px solid #9eaea5; background: #fff; color: #193d2f; }
.danger-button { border: 1px solid #ba6a61; background: #fff8f6; color: #8d3028; }
button:disabled { opacity: .6; cursor: wait; }
</style>
