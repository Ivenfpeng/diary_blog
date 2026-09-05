<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { apiRequest } from '../api/client'

type TaxonomyKind = 'categories' | 'tags'
interface Taxonomy { id: number; name: string; slug: string }
interface Editing { kind: TaxonomyKind; id: number; name: string; slug: string }

const categories = ref<Taxonomy[]>([])
const tags = ref<Taxonomy[]>([])
const name = ref('')
const slug = ref('')
const kind = ref<TaxonomyKind>('categories')
const editing = ref<Editing | null>(null)
const deleting = ref<Editing | null>(null)
const errorMessage = ref('')
const editHeading = computed(() => editing.value?.kind === 'categories' ? 'Edit category' : 'Edit tag')

function collection(type: TaxonomyKind) {
  return type === 'categories' ? categories : tags
}
function singular(type: TaxonomyKind) {
  return type === 'categories' ? 'category' : 'tag'
}

async function load(): Promise<void> {
  try {
    const [categoryResponse, tagResponse] = await Promise.all([
      apiRequest<{ categories: Taxonomy[] }>('/api/admin/categories'),
      apiRequest<{ tags: Taxonomy[] }>('/api/admin/tags'),
    ])
    categories.value = categoryResponse.categories
    tags.value = tagResponse.tags
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to load taxonomy.'
  }
}

async function create(): Promise<void> {
  try {
    const response = await apiRequest<Record<string, Taxonomy>>(`/api/admin/${kind.value}`, { method: 'POST', body: { name: name.value, slug: slug.value } })
    collection(kind.value).value.push(response[singular(kind.value)])
    name.value = ''
    slug.value = ''
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to save taxonomy.'
  }
}

function startEdit(type: TaxonomyKind, item: Taxonomy): void {
  editing.value = { kind: type, ...item }
}

async function saveEdit(): Promise<void> {
  if (!editing.value) return
  const current = editing.value
  try {
    const response = await apiRequest<Record<string, Taxonomy>>(`/api/admin/${current.kind}/${current.id}`, { method: 'PUT', body: { name: current.name, slug: current.slug } })
    const values = collection(current.kind).value
    const index = values.findIndex((item) => item.id === current.id)
    if (index >= 0) values[index] = response[singular(current.kind)]
    editing.value = null
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to update taxonomy.'
  }
}

async function confirmDelete(): Promise<void> {
  if (!deleting.value) return
  const current = deleting.value
  try {
    await apiRequest<void>(`/api/admin/${current.kind}/${current.id}`, { method: 'DELETE' })
    collection(current.kind).value = collection(current.kind).value.filter((item) => item.id !== current.id)
    deleting.value = null
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'Unable to delete taxonomy.'
  }
}

onMounted(load)
</script>

<template>
  <section class="taxonomy-view" aria-labelledby="taxonomy-heading">
    <p class="eyebrow">Organization</p>
    <h1 id="taxonomy-heading">Categories &amp; tags</h1>
    <form class="taxonomy-form" @submit.prevent="create">
      <label>Type <select v-model="kind"><option value="categories">Category</option><option value="tags">Tag</option></select></label>
      <label>Name <input v-model="name" required /></label>
      <label>Slug <input v-model="slug" required pattern="[a-z0-9-]+" /></label>
      <button class="primary-button">Add</button>
    </form>
    <p v-if="errorMessage" role="alert" class="form-error">{{ errorMessage }}</p>
    <div class="lists">
      <section v-for="group in [{ type: 'categories' as const, label: 'Categories', values: categories }, { type: 'tags' as const, label: 'Tags', values: tags }]" :key="group.type">
        <h2>{{ group.label }}</h2>
        <div v-for="item in group.values" :key="item.id" class="taxonomy-row">
          <span>{{ item.name }} <small>/{{ item.slug }}</small></span>
          <span><button type="button" class="text-button" :data-testid="`edit-${group.type}-${item.id}`" @click="startEdit(group.type, item)">Edit</button><button type="button" class="text-button danger" :data-testid="`delete-${group.type}-${item.id}`" @click="deleting = { kind: group.type, ...item }">Delete</button></span>
        </div>
      </section>
    </div>
    <section v-if="editing" class="dialog" role="dialog" aria-modal="true" :aria-label="editHeading">
      <h2>{{ editHeading }}</h2><label>Name <input id="taxonomy-edit-name" v-model="editing.name" /></label><label>Slug <input id="taxonomy-edit-slug" v-model="editing.slug" /></label>
      <div><button type="button" class="secondary-button" @click="editing = null">Cancel</button><button type="button" class="primary-button" data-testid="save-taxonomy-edit" @click="saveEdit">Save changes</button></div>
    </section>
    <section v-if="deleting" class="dialog" role="dialog" aria-modal="true" aria-label="Confirm taxonomy deletion">
      <h2>Delete {{ deleting.name }}?</h2><p>This cannot be undone.</p>
      <div><button type="button" class="secondary-button" @click="deleting = null">Cancel</button><button type="button" class="danger-button" data-testid="confirm-taxonomy-delete" @click="confirmDelete">Delete</button></div>
    </section>
  </section>
</template>

<style scoped>
h1 { margin: 0 0 20px; font-size: 28px; }
.taxonomy-form { display: flex; gap: 12px; align-items: end; flex-wrap: wrap; border-bottom: 1px solid #d9dfdb; padding-bottom: 16px; }
label { display: grid; gap: 6px; font-size: 13px; font-weight: 650; }
input, select { min-height: 40px; border: 1px solid #b9c5be; border-radius: 3px; padding: 0 10px; font: inherit; }
.lists { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 32px; padding-top: 18px; }
h2 { font-size: 17px; } small { color: #68756d; }
.taxonomy-row { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 8px 0; border-top: 1px solid #edf0ee; }
.text-button { border: 0; background: transparent; color: #285e48; font: inherit; font-size: 13px; cursor: pointer; }
.danger { color: #a33a32; }
.dialog { position: fixed; inset: 20% max(16px, calc(50% - 220px)) auto; z-index: 2; display: grid; gap: 12px; padding: 20px; border: 1px solid #9eaea5; border-radius: 4px; background: #fff; box-shadow: 0 12px 40px #193d2f33; }
.dialog h2 { margin: 0; }.dialog div { display: flex; gap: 10px; }
.secondary-button, .danger-button { min-height: 36px; border-radius: 3px; padding: 0 10px; font: inherit; font-weight: 650; cursor: pointer; }
.secondary-button { border: 1px solid #9eaea5; background: #fff; }.danger-button { border: 1px solid #a33a32; background: #a33a32; color: #fff; }
@media (max-width: 600px) { .lists { grid-template-columns: 1fr; } }
</style>
