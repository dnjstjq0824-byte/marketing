import { createApp } from 'vue'
import './styles/tokens.css'
import './styles/base.css'
import './styles/public.css'
import App from './App.vue'
import router from './router/index.js'
import { applyTheme } from './stores/theme.js'

applyTheme()
const app = createApp(App)
app.use(router)
app.mount('#app')

// PWA: register the service worker when supported. Failure is silent:
// the console works fully without it.
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/sw.js').catch(() => {})
}
