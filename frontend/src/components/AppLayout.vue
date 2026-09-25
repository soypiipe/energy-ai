<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import BrandLogo from './BrandLogo.vue'
import RunAnalysisButton from './RunAnalysisButton.vue'
import { api } from '@/api/client'
import { useAuth } from '@/composables/useAuth'
import { useAnalysis } from '@/composables/useAnalysis'

const route = useRoute()
const router = useRouter()
const { logout } = useAuth()
const { version } = useAnalysis()

const openAnomalies = ref(0)
const menuOpen = ref(false)

const nav = [
  { to: '/', name: 'dashboard', label: 'Dashboard', icon: 'pi-th-large' },
  { to: '/meters', name: 'meters', label: 'Medidores', icon: 'pi-bolt' },
  { to: '/anomalies', name: 'anomalies', label: 'Anomalías IA', icon: 'pi-sparkles' },
]

// El módulo activo incluye sus pantallas hijas (medidor → Medidores, investigación → Anomalías IA).
const activeName = computed(() => {
  const n = String(route.name ?? '')
  return n === 'meter' ? 'meters' : n === 'anomaly' ? 'anomalies' : n
})

async function loadCount() {
  try {
    openAnomalies.value = (await api.dashboard()).anomalies.open
  } catch { /* el contador es decorativo: si falla, la vista principal ya mostrará el error */ }
}

onMounted(loadCount)
watch(version, loadCount)
watch(() => route.fullPath, () => { menuOpen.value = false })

function signOut() {
  logout()
  router.replace({ name: 'login' })
}
</script>

<template>
  <div class="shell">
    <a class="skip" href="#main">Saltar al contenido</a>

    <aside class="side" :class="{ open: menuOpen }" aria-label="Navegación principal">
      <div class="side-top"><BrandLogo :size="30" /></div>
      <nav>
        <RouterLink v-for="item in nav" :key="item.name" :to="item.to" class="nav-item" :class="{ active: activeName === item.name }"
          :aria-current="activeName === item.name ? 'page' : undefined">
          <i class="pi" :class="item.icon" aria-hidden="true" />
          <span>{{ item.label }}</span>
          <span v-if="item.name === 'anomalies' && openAnomalies > 0" class="count" :aria-label="`${openAnomalies} abiertas`">{{ openAnomalies }}</span>
        </RouterLink>
      </nav>
      <div class="side-foot">
        <div class="user">
          <span class="avatar" aria-hidden="true">D</span>
          <div><div class="uname">demo</div><div class="muted small">Operador</div></div>
        </div>
        <Button icon="pi pi-sign-out" text severity="secondary" rounded aria-label="Cerrar sesión" @click="signOut" />
      </div>
    </aside>
    <div v-if="menuOpen" class="scrim" @click="menuOpen = false" />

    <div class="main-col">
      <header class="top">
        <Button class="burger" icon="pi pi-bars" text severity="secondary" aria-label="Abrir menú" @click="menuOpen = !menuOpen" />
        <h1 class="title">{{ route.meta.title }}</h1>
        <div class="spacer" />
        <RunAnalysisButton size="small" />
      </header>
      <main id="main" class="content" tabindex="-1">
        <RouterView />
      </main>
    </div>
  </div>
</template>

<style scoped>
.shell { display: grid; grid-template-columns: 240px 1fr; min-height: 100%; }
.skip { position: absolute; left: -999px; top: 8px; background: var(--accent); color: #0B0F19; padding: 8px 12px; border-radius: 8px; z-index: 100; }
.skip:focus { left: 8px; }

.side {
  position: sticky; top: 0; height: 100vh;
  display: flex; flex-direction: column;
  background: var(--surface); border-right: 1px solid var(--border);
  padding: 20px 14px;
}
.side-top { padding: 4px 8px 24px; }
nav { display: grid; gap: 4px; }
.nav-item {
  display: flex; align-items: center; gap: 12px;
  padding: 10px 12px; border-radius: 10px;
  color: var(--muted); font-weight: 500; text-decoration: none;
  transition: background .15s, color .15s;
}
.nav-item:hover { background: var(--surface-2); color: var(--text); text-decoration: none; }
.nav-item.active { background: var(--accent-soft); color: var(--accent); }
.nav-item .pi { font-size: 16px; }
.count {
  margin-left: auto; min-width: 22px; padding: 0 7px; text-align: center;
  background: var(--crit); color: #fff; border-radius: 999px; font-size: 12px; font-weight: 700; line-height: 20px;
}
.side-foot { margin-top: auto; display: flex; align-items: center; justify-content: space-between; padding-top: 16px; border-top: 1px solid var(--border); }
.user { display: flex; align-items: center; gap: 10px; }
.avatar { width: 32px; height: 32px; border-radius: 50%; background: var(--accent-soft); color: var(--accent); display: grid; place-content: center; font-weight: 700; }
.uname { font-weight: 600; line-height: 1.2; }
.small { font-size: 12px; }

.main-col { min-width: 0; display: flex; flex-direction: column; }
.top {
  position: sticky; top: 0; z-index: 10;
  display: flex; align-items: center; gap: 12px;
  height: 64px; padding: 0 28px;
  background: color-mix(in srgb, var(--bg) 85%, transparent); backdrop-filter: blur(8px);
  border-bottom: 1px solid var(--border);
}
.title { font-size: 18px; }
.spacer { flex: 1; }
.burger { display: none; }
.content { padding: 28px; max-width: 1400px; width: 100%; margin: 0 auto; outline: none; }
.scrim { display: none; }

@media (max-width: 900px) {
  .shell { grid-template-columns: 1fr; }
  .side { position: fixed; z-index: 30; width: 260px; transform: translateX(-100%); transition: transform .2s; }
  .side.open { transform: none; }
  .scrim { display: block; position: fixed; inset: 0; z-index: 20; background: rgba(0,0,0,.5); }
  .burger { display: inline-flex; }
  .top { padding: 0 16px; }
  .content { padding: 16px; }
}
</style>
