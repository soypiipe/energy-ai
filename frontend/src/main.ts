import { createApp } from 'vue'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import 'primeicons/primeicons.css'
import './styles/theme.css'
import App from './App.vue'
import { router } from './router'
import { VoltixPreset } from './theme/preset'
import { setUnauthorizedHandler } from './api/client'
import { useAuth } from './composables/useAuth'

// Si el backend responde 401 con sesión activa (token vencido), se cierra la sesión y se vuelve al login.
setUnauthorizedHandler(() => {
  useAuth().logout()
  const current = router.currentRoute.value
  if (current.name !== 'login') router.push({ name: 'login', query: { redirect: current.fullPath, expired: '1' } })
})

createApp(App)
  .use(PrimeVue, { theme: { preset: VoltixPreset, options: { darkModeSelector: '.dark', cssLayer: false } } })
  .use(ToastService)
  .use(router)
  .mount('#app')
