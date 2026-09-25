import { createRouter, createWebHistory } from 'vue-router'
import { useAuth } from '@/composables/useAuth'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { public: true, title: 'Iniciar sesión' } },
    {
      path: '/',
      component: () => import('@/components/AppLayout.vue'),
      children: [
        { path: '', name: 'dashboard', component: () => import('@/views/DashboardView.vue'), meta: { title: 'Dashboard' } },
        { path: 'meters', name: 'meters', component: () => import('@/views/MetersView.vue'), meta: { title: 'Medidores' } },
        { path: 'meters/:id', name: 'meter', component: () => import('@/views/MeterDetailView.vue'), meta: { title: 'Detalle del medidor' } },
        { path: 'anomalies', name: 'anomalies', component: () => import('@/views/AnomaliesView.vue'), meta: { title: 'Anomalías IA' } },
        { path: 'anomalies/:id', name: 'anomaly', component: () => import('@/views/InvestigationView.vue'), meta: { title: 'Investigación' } },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach((to) => {
  const { isAuthenticated } = useAuth()
  if (!to.meta.public && !isAuthenticated.value) return { name: 'login', query: { redirect: to.fullPath } }
  if (to.name === 'login' && isAuthenticated.value) return { name: 'dashboard' }
})

router.afterEach((to) => {
  document.title = `${to.meta.title ?? 'Voltix'} · Voltix`
})
