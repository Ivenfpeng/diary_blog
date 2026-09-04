<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { FilePlus, Search } from '@lucide/vue'
import { useRouter } from 'vue-router'
import { apiRequest } from '../api/client'
import type { EditablePost } from '../state/editor'

interface PostListResponse { posts: EditablePost[]; total: number; page: number; page_size: number }

const router = useRouter()
const status = ref('')
const query = ref('')
const page = ref(1)
const pageSize = 20
const total = ref(0)
const posts = ref<EditablePost[]>([])
const loading = ref(false)
const errorMessage = ref('')
const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

async function loadPosts(): Promise<void> {
  loading.value = true
  errorMessage.value = ''
  const params = new URLSearchParams({ page: String(page.value), page_size: String(pageSize) })
  if (status.value) params.set('status', status.value)
  if (query.value.trim()) params.set('q', query.value.trim())
  try {
    const response = await apiRequest<PostListResponse>(`/api/admin/posts?${params}`)
    posts.value = response.posts
    total.value = response.total
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to load articles.'
  } finally {
    loading.value = false
  }
}

async function createPost(): Promise<void> {
  const response = await apiRequest<{ post: EditablePost }>('/api/admin/posts', {
    method: 'POST',
    body: { slug: '', title: '', summary: '', content_md: '', category_id: null, tag_ids: [] },
  })
  await router.push({ name: 'post-edit', params: { id: response.post.id } })
}

function search(): void {
  page.value = 1
  void loadPosts()
}

function goToPage(next: number): void {
  page.value = Math.min(Math.max(1, next), pageCount.value)
  void loadPosts()
}

onMounted(loadPosts)
</script>

<template>
  <section class="posts-view" aria-labelledby="posts-heading">
    <div class="view-heading">
      <div><p class="eyebrow">Articles</p><h1 id="posts-heading">Posts</h1></div>
      <button type="button" class="primary-button create-button" @click="createPost"><FilePlus :size="17" aria-hidden="true" /> New post</button>
    </div>
    <form class="post-filters" @submit.prevent="search">
      <label>Status <select v-model="status" @change="search"><option value="">All statuses</option><option value="draft">Draft</option><option value="published">Published</option><option value="archived">Archived</option></select></label>
      <label class="search-field">Search <input v-model="query" type="search" placeholder="Title or content" /></label>
      <button type="submit" class="secondary-button"><Search :size="16" aria-hidden="true" /> Search</button>
    </form>
    <p v-if="errorMessage" class="form-error" role="alert">{{ errorMessage }}</p>
    <p v-else-if="loading" class="muted">Loading articles…</p>
    <div v-else class="post-list">
      <RouterLink v-for="post in posts" :key="post.id" :to="{ name: 'post-edit', params: { id: post.id } }" class="post-row">
        <span><strong>{{ post.title || 'Untitled article' }}</strong><small>{{ post.summary || post.slug || 'No summary' }}</small></span>
        <span class="post-meta">{{ post.status }} · v{{ post.revision }}</span>
      </RouterLink>
      <p v-if="!posts.length" class="muted">No articles match these filters.</p>
    </div>
    <nav class="pagination" aria-label="Posts pagination">
      <button type="button" class="secondary-button" :disabled="page === 1 || loading" @click="goToPage(page - 1)">Previous</button>
      <span>Page {{ page }} of {{ pageCount }}</span>
      <button type="button" class="secondary-button" :disabled="page === pageCount || loading" @click="goToPage(page + 1)">Next</button>
    </nav>
  </section>
</template>

<style scoped>
.posts-view { max-width: 960px; }
.view-heading, .post-filters, .pagination { display: flex; align-items: end; gap: 12px; flex-wrap: wrap; }
.view-heading { justify-content: space-between; margin-bottom: 24px; }
h1 { margin: 0; font-size: 28px; letter-spacing: -.025em; }
.create-button, .secondary-button { min-height: 40px; display: inline-flex; align-items: center; gap: 6px; padding: 0 12px; border-radius: 3px; font: inherit; font-weight: 650; cursor: pointer; }
.create-button { margin: 0; }
.secondary-button { border: 1px solid #9eaea5; background: #fff; color: #193d2f; }
.post-filters { padding-bottom: 16px; border-bottom: 1px solid #d9dfdb; }
label { display: grid; gap: 5px; color: #39463e; font-size: 13px; font-weight: 650; }
input, select { min-height: 40px; border: 1px solid #b9c5be; border-radius: 3px; padding: 0 10px; font: inherit; background: #fff; }
.search-field { min-width: min(100%, 300px); flex: 1; }
.post-list { border-bottom: 1px solid #d9dfdb; }
.post-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 14px 4px; border-top: 1px solid #edf0ee; color: inherit; text-decoration: none; }
.post-row:first-child { border-top: 0; }
.post-row:hover { background: #f4f7f5; }
small { display: block; margin-top: 4px; color: #68756d; }
.post-meta, .muted { color: #68756d; font-size: 13px; }
.pagination { justify-content: flex-end; padding-top: 16px; }
button:disabled { opacity: .55; cursor: not-allowed; }
@media (max-width: 600px) { .post-row { align-items: flex-start; flex-direction: column; gap: 5px; } .pagination { justify-content: space-between; } }
</style>
