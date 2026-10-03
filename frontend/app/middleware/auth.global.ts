export default defineNuxtRouteMiddleware(async (to) => {
  if (to.path === '/auth' || to.path.startsWith('/auth/')) return
  const auth = useAuthStore()
  if (!auth.user) await auth.fetchUser()
  if (!auth.user) return navigateTo('/auth')
})
