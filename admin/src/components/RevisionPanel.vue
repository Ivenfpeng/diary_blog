<script setup lang="ts">
export interface PostRevision {
  id: number
  revision: number
  title: string
  created_at: string
}

defineProps<{ revisions: PostRevision[]; restoring: boolean; actionsLocked: boolean }>()
const emit = defineEmits<{ restore: [revision: PostRevision] }>()
</script>

<template>
  <section class="revision-panel" aria-labelledby="revisions-heading">
    <h2 id="revisions-heading">Revisions</h2>
    <p v-if="!revisions.length" class="muted">No earlier revisions yet.</p>
    <ol v-else>
      <li v-for="revision in revisions" :key="revision.id">
        <span><strong>v{{ revision.revision }}</strong> {{ revision.title || 'Untitled article' }}</span>
        <button type="button" :disabled="restoring || actionsLocked" @click="emit('restore', revision)">Restore</button>
      </li>
    </ol>
  </section>
</template>

<style scoped>
.revision-panel { padding: 20px; border: 1px solid rgb(202 215 205 / 80%); border-radius: 24px; background: var(--admin-panel-strong); box-shadow: var(--admin-shadow); }
h2 { margin: 0 0 10px; font-size: 18px; }
ol { margin: 0; padding: 0; list-style: none; }
li { display: flex; justify-content: space-between; gap: 12px; padding: 11px 0; border-top: 1px solid #edf0ee; font-size: 14px; }
button { min-height: 34px; border: 1px solid #9eaea5; border-radius: 11px; background: #fff; color: #193d2f; font: inherit; cursor: pointer; }
.muted { color: #6b766f; }
</style>
