<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ArrowUpRight, FilePlus, FolderTree, Image, Settings2 } from '@lucide/vue'
import { useRouter } from 'vue-router'
import { apiRequest } from '../api/client'
import type { EditablePost } from '../state/editor'
import { notifyError } from '../state/notifications'

interface DashboardData {
  posts: EditablePost[]
  total: number
  media: Array<{ id: number; path: string }>
  categories: Array<{ id: number; name: string }>
  tags: Array<{ id: number; name: string }>
}

const router = useRouter()
const loading = ref(true)
const errorMessage = ref('')
const data = ref<DashboardData>({ posts: [], total: 0, media: [], categories: [], tags: [] })
const recentPosts = computed(() => data.value.posts.slice(0, 5))

async function load(): Promise<void> {
  try {
    const [posts, media, categories, tags] = await Promise.all([
      apiRequest<{ posts: EditablePost[]; total: number }>('/api/admin/posts?page=1&page_size=5'),
      apiRequest<{ media: DashboardData['media'] }>('/api/admin/media'),
      apiRequest<{ categories: DashboardData['categories'] }>('/api/admin/categories'),
      apiRequest<{ tags: DashboardData['tags'] }>('/api/admin/tags'),
    ])
    data.value = { posts: posts.posts, total: posts.total, media: media.media, categories: categories.categories, tags: tags.tags }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to load workspace metrics.'
    notifyError('Workspace failed to load', errorMessage.value)
  } finally {
    loading.value = false
  }
}

function goTo(name: string): void {
  void router.push({ name })
}

onMounted(load)
</script>

<template>
  <section class="dashboard-view" aria-labelledby="dashboard-heading">
    <div class="dashboard-hero">
      <div>
        <p class="eyebrow">Good to see you / Workspace console</p>
        <h1 id="dashboard-heading">Overview</h1>
        <p>Keep your knowledge base tidy, publish with confidence, and see the latest activity at a glance.</p>
      </div>
      <div class="dashboard-hero-mark" aria-hidden="true">DB</div>
    </div>
    <p v-if="errorMessage" class="form-error" role="alert">{{ errorMessage }}</p>
    <div v-else-if="loading" class="dashboard-loading muted">Loading workspace metrics…</div>
    <template v-else>
      <div class="metric-grid" aria-label="Workspace metrics">
        <article class="metric-card metric-card-primary" data-testid="metric-posts"><span>Articles</span><strong>{{ data.total }}</strong><small>All drafts and published notes</small></article>
        <article class="metric-card" data-testid="metric-media"><span>Media assets</span><strong>{{ data.media.length }}</strong><small>Images ready to reuse</small></article>
        <article class="metric-card" data-testid="metric-taxonomy"><span>Taxonomy</span><strong>{{ data.categories.length + data.tags.length }}</strong><small>{{ data.categories.length }} categories · {{ data.tags.length }} tags</small></article>
      </div>
      <div class="dashboard-grid">
        <section class="dashboard-panel dashboard-actions-panel" aria-labelledby="quick-actions-heading">
          <div class="panel-heading"><div><p class="eyebrow">Shortcuts</p><h2 id="quick-actions-heading">Quick actions</h2></div></div>
          <div class="quick-actions" data-testid="dashboard-actions">
            <button type="button" class="quick-action quick-action-primary" @click="goTo('posts')"><FilePlus :size="18" aria-hidden="true" /><span>New post</span><ArrowUpRight :size="15" aria-hidden="true" /></button>
            <button type="button" class="quick-action" @click="goTo('media')"><Image :size="18" aria-hidden="true" /><span>Upload media</span><ArrowUpRight :size="15" aria-hidden="true" /></button>
            <button type="button" class="quick-action" @click="goTo('taxonomy')"><FolderTree :size="18" aria-hidden="true" /><span>Organize taxonomy</span><ArrowUpRight :size="15" aria-hidden="true" /></button>
            <button type="button" class="quick-action" @click="goTo('settings')"><Settings2 :size="18" aria-hidden="true" /><span>Site settings</span><ArrowUpRight :size="15" aria-hidden="true" /></button>
          </div>
        </section>
        <section class="dashboard-panel" aria-labelledby="recent-heading">
          <div class="panel-heading"><div><p class="eyebrow">Activity</p><h2 id="recent-heading">Recent articles</h2></div><RouterLink :to="{ name: 'posts' }" class="panel-link">View all</RouterLink></div>
          <div v-if="recentPosts.length" class="recent-list">
            <RouterLink v-for="post in recentPosts" :key="post.id" :to="{ name: 'post-edit', params: { id: post.id } }" class="recent-row" data-testid="recent-post">
              <span><strong>{{ post.title || 'Untitled article' }}</strong><small>{{ post.slug || 'Draft without slug' }}</small></span><em>{{ post.status }}</em>
            </RouterLink>
          </div>
          <p v-else class="empty-state">No articles yet. Start with a short note.</p>
        </section>
      </div>
    </template>
  </section>
</template>

<style scoped>
.dashboard-view { display: grid; gap: 22px; min-height: 100%; padding: clamp(24px, 4vw, 48px); border: 1px solid #dfe7e1; border-radius: 24px; background: rgb(255 255 255 / 88%); box-shadow: 0 20px 70px rgb(24 60 42 / 9%); }
.dashboard-hero { display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; padding-bottom: 24px; border-bottom: 1px solid #e8eee9; }
.dashboard-hero h1 { margin: 0 0 10px; font-size: clamp(32px, 5vw, 56px); letter-spacing: -.055em; }
.dashboard-hero p:not(.eyebrow) { max-width: 620px; margin: 0; color: #66766d; font-size: 15px; line-height: 1.7; }
.dashboard-hero-mark { display: grid; width: 72px; height: 72px; place-items: center; border-radius: 22px; background: linear-gradient(135deg, #1677ff, #36cfc9); color: #fff; font-size: 20px; font-weight: 850; letter-spacing: -.05em; box-shadow: 0 14px 30px rgb(22 119 255 / 24%); }
.metric-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; }
.metric-card { display: grid; gap: 7px; padding: 20px; border: 1px solid #e6ece8; border-radius: 18px; background: #fff; }
.metric-card span { color: #6b7b72; font-size: 13px; font-weight: 700; }.metric-card strong { color: #152f25; font-size: 34px; letter-spacing: -.06em; }.metric-card small { color: #89958e; font-size: 12px; }.metric-card-primary { border-color: #b9d8ff; background: linear-gradient(145deg, #f0f7ff, #fff); }.metric-card-primary strong { color: #1677ff; }
.dashboard-grid { display: grid; grid-template-columns: minmax(280px, .88fr) minmax(0, 1.12fr); gap: 14px; }
.dashboard-panel { min-width: 0; padding: 20px; border: 1px solid #e6ece8; border-radius: 18px; background: #fff; }.panel-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 16px; }.panel-heading h2 { margin: 0; color: #183529; font-size: 18px; }.panel-link { color: #1677ff; font-size: 12px; font-weight: 750; text-decoration: none; }
.quick-actions { display: grid; gap: 8px; }.quick-action { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 10px; min-height: 48px; padding: 0 13px; border: 1px solid #e3ebe5; border-radius: 12px; background: #fbfdfc; color: #234236; font: inherit; font-size: 13px; font-weight: 700; text-align: left; cursor: pointer; }.quick-action:hover { border-color: #91bdf2; background: #f1f7ff; color: #1677ff; }.quick-action-primary { border-color: #91bdf2; background: #eff6ff; color: #1668d7; }
.recent-list { display: grid; }.recent-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 0; border-top: 1px solid #eef2ef; color: inherit; text-decoration: none; }.recent-row:first-child { border-top: 0; }.recent-row strong { display: block; color: #213d31; font-size: 13px; }.recent-row small { display: block; margin-top: 3px; color: #87948d; font-size: 11px; }.recent-row em { color: #1677ff; font-size: 11px; font-style: normal; font-weight: 800; text-transform: uppercase; }
.dashboard-loading, .empty-state { color: #738179; }.muted { color: #68756d; }
@media (max-width: 900px) { .dashboard-grid { grid-template-columns: 1fr; } }
@media (max-width: 640px) { .dashboard-hero { flex-direction: column; }.dashboard-hero-mark { width: 56px; height: 56px; border-radius: 16px; }.metric-grid { grid-template-columns: 1fr; }.dashboard-view { padding: 20px; border-radius: 18px; } }
</style>
