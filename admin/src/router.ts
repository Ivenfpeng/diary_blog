import { createRouter, createWebHistory } from 'vue-router'
import AdminLayout from './layouts/AdminLayout.vue'
import DashboardView from './views/DashboardView.vue'
import LoginView from './views/LoginView.vue'
import { restoreSession, session } from './state/session'

const router = createRouter({
  history: createWebHistory('/admin/'),
  routes: [
    { path: '/admin/login', name: 'login', component: LoginView, meta: { public: true } },
    {
      path: '/admin',
      component: AdminLayout,
      meta: { requiresAuth: true },
      children: [{ path: '', name: 'dashboard', component: DashboardView }],
    },
    { path: '/:pathMatch(.*)*', redirect: '/admin' },
  ],
})

router.beforeEach(async (to) => {
  const authenticated = await restoreSession()

  if (to.meta.requiresAuth && !authenticated) return '/admin/login'
  if (to.meta.public && authenticated) return '/admin'
  return true
})

export { session }
export default router
