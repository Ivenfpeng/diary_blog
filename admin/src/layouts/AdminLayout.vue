<script setup lang="ts">
import { ref } from 'vue'
import { FileText, LayoutDashboard, Menu, X } from '@lucide/vue'
import IconButton from '../components/IconButton.vue'
import { session } from '../state/session'

const navigationOpen = ref(false)
const closeNavigation = () => { navigationOpen.value = false }
</script>

<template>
  <div class="admin-shell" :class="{ 'navigation-open': navigationOpen }">
    <aside class="admin-nav" aria-label="Administration navigation">
      <div class="brand">Diary Blog</div>
      <nav>
        <RouterLink to="/admin" class="nav-link" @click="closeNavigation">
          <LayoutDashboard :size="18" aria-hidden="true" />
          Overview
        </RouterLink>
        <span class="nav-link nav-link-disabled" aria-disabled="true">
          <FileText :size="18" aria-hidden="true" />
          Posts
        </span>
      </nav>
    </aside>
    <div class="admin-workspace">
      <header class="admin-topbar">
        <IconButton class="menu-toggle" :icon="navigationOpen ? X : Menu" :label="navigationOpen ? 'Close navigation' : 'Open navigation'" @click="navigationOpen = !navigationOpen" />
        <div class="workspace-title">Administration</div>
        <div class="session-user" aria-label="Signed in administrator">{{ session.username }}</div>
      </header>
      <main class="admin-content">
        <RouterView />
      </main>
    </div>
  </div>
</template>
