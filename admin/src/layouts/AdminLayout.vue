<script setup lang="ts">
import { ref } from 'vue'
import { FileText, FolderTree, Image, LayoutDashboard, Menu, Settings, X } from '@lucide/vue'
import IconButton from '../components/IconButton.vue'
import NotificationTray from '../components/NotificationTray.vue'
import { session } from '../state/session'

const navigationOpen = ref(false)
const closeNavigation = () => { navigationOpen.value = false }
</script>

<template>
  <div class="admin-shell" :class="{ 'navigation-open': navigationOpen }">
    <aside class="admin-nav" aria-label="Administration navigation">
      <div class="brand">Diary Blog</div>
      <nav>
        <RouterLink :to="{ name: 'dashboard' }" class="nav-link" @click="closeNavigation">
          <LayoutDashboard :size="18" aria-hidden="true" />
          Overview
        </RouterLink>
        <RouterLink :to="{ name: 'posts' }" class="nav-link" @click="closeNavigation">
          <FileText :size="18" aria-hidden="true" />
          Posts
        </RouterLink>
        <RouterLink :to="{ name: 'taxonomy' }" class="nav-link" @click="closeNavigation"><FolderTree :size="18" aria-hidden="true" /> Categories &amp; tags</RouterLink>
        <RouterLink :to="{ name: 'media' }" class="nav-link" @click="closeNavigation"><Image :size="18" aria-hidden="true" /> Media</RouterLink>
        <RouterLink :to="{ name: 'settings' }" class="nav-link" @click="closeNavigation"><Settings :size="18" aria-hidden="true" /> Settings</RouterLink>
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
      <NotificationTray />
    </div>
  </div>
</template>
