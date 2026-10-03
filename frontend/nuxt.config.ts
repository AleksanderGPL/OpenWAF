// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  telemetry: false,
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  ssr: false,
  nitro: {
    preset: 'static',
    devProxy: {
      '/api': {
        target: 'http://127.0.0.1:3001/api',
        changeOrigin: true
      }
    }
  },
  modules: ['@nuxt/ui'],
  css: ['~/assets/css/main.css']
})
