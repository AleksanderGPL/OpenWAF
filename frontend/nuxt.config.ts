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
  modules: ['@nuxt/ui', 'nuxt-echarts', '@nuxt/icon', '@pinia/nuxt', '@nuxtjs/i18n'],
  i18n: {
    experimental: { prerenderMessages: true },
    defaultLocale: 'en',
    strategy: 'no_prefix',
    langDir: 'locales',
    detectBrowserLanguage: {
      useCookie: true,
      cookieKey: 'openwaf-locale',
      fallbackLocale: 'en'
    },
    locales: [
      { code: 'en', name: 'English', file: 'en.json' },
      { code: 'pl', name: 'Polski', file: 'pl.json' }
    ]
  },
  icon: {
    provider: 'iconify'
  },
  echarts: {
    renderer: 'svg',
    charts: ['LineChart', 'BarChart', 'PieChart'],
    components: ['GridComponent', 'TooltipComponent', 'LegendComponent']
  },
  css: ['~/assets/css/main.css'],
  app: {
    head: {
      link: [
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        { rel: 'stylesheet', href: 'https://fonts.googleapis.com/css2?family=Figtree:ital,wght@0,300..900;1,300..900&display=swap' }
      ]
    }
  }
})