import { translateApiMessage } from '~/utils/i18n'

export type AuthUser = {
  id: number
  username: string
  name: string
  role: string
  createdAt: string
}

const usernameStorageKey = 'openwaf.username'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<AuthUser | null>(null)

  async function fetchUser() {
    try {
      user.value = await $fetch<AuthUser>('/api/auth/')
    } catch {
      user.value = null
    }
    return user.value
  }

  async function signIn(username: string, password: string) {
    user.value = await $fetch<AuthUser>('/api/auth/sign-in', {
      method: 'POST',
      body: { username, password }
    })
    return user.value
  }

  async function signUp(body: { username: string, password: string, name: string }) {
    return await $fetch<AuthUser>('/api/auth/setup', {
      method: 'POST',
      body
    })
  }

  async function signOut() {
    await $fetch('/api/auth/sign-out', { method: 'POST' })
    user.value = null
  }

  function rememberedUsername() {
    if (!import.meta.client) return ''
    return localStorage.getItem(usernameStorageKey) ?? ''
  }

  function rememberUsername(username: string | null) {
    if (!import.meta.client) return
    if (username) localStorage.setItem(usernameStorageKey, username)
    else localStorage.removeItem(usernameStorageKey)
  }

  function authErrorMessage(error: unknown, fallback: string) {
    if (typeof error === 'object' && error && 'data' in error) {
      const message = (error as { data?: { message?: string } }).data?.message
      if (message) return translateApiMessage(message)
    }
    return fallback
  }

  return {
    user,
    fetchUser,
    signIn,
    signUp,
    signOut,
    rememberedUsername,
    rememberUsername,
    authErrorMessage
  }
})
