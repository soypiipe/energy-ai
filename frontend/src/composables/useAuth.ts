import { computed, ref } from 'vue'
import { api, tokenStore } from '@/api/client'

const token = ref<string | null>(tokenStore.get())

/** Estado de sesión compartido por toda la app. */
export function useAuth() {
  return {
    isAuthenticated: computed(() => !!token.value),
    async login(username: string, password: string) {
      const res = await api.login(username, password)
      tokenStore.set(res.access_token)
      token.value = res.access_token
    },
    logout() {
      tokenStore.clear()
      token.value = null
    },
  }
}
