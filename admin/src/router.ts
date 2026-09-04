import { createRouter, createWebHistory } from 'vue-router'
import AdminLayout from './layouts/AdminLayout.vue'
import DashboardView from './views/DashboardView.vue'
import LoginView from './views/LoginView.vue'
import PostsView from './views/PostsView.vue'
import PostEditorView from './views/PostEditorView.vue'
import { restoreSession, session } from './state/session'

const router = createRouter({
  history: createWebHistory('/admin/'),
  routes: [
    { path: '/login', name: 'login', component: LoginView, meta: { public: true } },
    {
      path: '/',
      component: AdminLayout,
      meta: { requiresAuth: true },
      children: [
        { path: '', name: 'dashboard', component: DashboardView },
        { path: 'posts', name: 'posts', component: PostsView },
        { path: 'posts/:id', name: 'post-edit', component: PostEditorView },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: { name: 'dashboard' } },
  ],
})

router.beforeEach(async (to) => {
  const authenticated = await restoreSession()

  if (to.meta.requiresAuth && !authenticated) return { name: 'login' }
  if (to.meta.public && authenticated) return { name: 'dashboard' }
  return true
})

export { session }
export default router
