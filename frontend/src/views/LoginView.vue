<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import InputText from 'primevue/inputtext'
import Password from 'primevue/password'
import Button from 'primevue/button'
import Message from 'primevue/message'
import BrandLogo from '@/components/BrandLogo.vue'
import { ApiError } from '@/api/client'
import { useAuth } from '@/composables/useAuth'

const route = useRoute()
const router = useRouter()
const { login } = useAuth()

const username = ref('demo')
const password = ref('')
const loading = ref(false)
const error = ref('')
const expired = route.query.expired === '1'

async function submit() {
  if (loading.value) return
  error.value = ''
  loading.value = true
  try {
    await login(username.value.trim(), password.value)
    const redirect = typeof route.query.redirect === 'string' && route.query.redirect.startsWith('/') ? route.query.redirect : '/'
    await router.replace(redirect)
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) error.value = 'Usuario o contraseña incorrectos.'
    else if (e instanceof ApiError && e.status === 429) error.value = 'Demasiados intentos. Espera un minuto e inténtalo de nuevo.'
    else if (e instanceof ApiError && e.status === 0) error.value = 'No se pudo conectar con el servidor. ¿Está corriendo la API?'
    else error.value = 'No se pudo iniciar sesión. Inténtalo de nuevo.'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main class="login">
    <section class="card" aria-labelledby="login-title">
      <BrandLogo :size="40" />
      <h1 id="login-title">Bienvenido de nuevo</h1>

      <Message v-if="expired && !error" severity="warn" :closable="false" class="msg">Tu sesión expiró. Inicia sesión otra vez.</Message>
      <Message v-if="error" severity="error" :closable="false" class="msg" role="alert">{{ error }}</Message>

      <form @submit.prevent="submit">
        <label for="username">Usuario</label>
        <InputText id="username" v-model="username" autocomplete="username" fluid required />

        <label for="password">Contraseña</label>
        <Password v-model="password" input-id="password" :feedback="false" toggle-mask autocomplete="current-password" fluid required />

        <Button type="submit" label="Iniciar sesión" icon="pi pi-arrow-right" icon-pos="right" :loading="loading" fluid class="submit" />
      </form>
    </section>
    <p class="foot muted">Voltix · Gestión de energía con IA</p>
  </main>
</template>

<style scoped>
.login {
  min-height: 100%;
  display: grid;
  place-content: center;
  justify-items: center;
  gap: 24px;
  padding: 24px 16px;
  background:
    radial-gradient(600px 300px at 50% 0%, rgba(45, 212, 167, 0.10), transparent 70%),
    var(--bg);
}
.card {
  width: min(400px, 100%);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 32px;
  display: grid;
  gap: 8px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.35);
}
h1 { margin-top: 20px; font-size: 24px; }
.lead { margin: 0 0 12px; }
form { display: grid; gap: 6px; }
label { font-size: 13px; font-weight: 500; margin-top: 10px; }
.submit { margin-top: 20px; }
.msg { margin: 4px 0 8px; }
.foot { font-size: 12px; margin: 0; }
</style>
