import { createApp } from 'vue'
import App from './App.vue'
import router from './router/index.js'
import { refreshMe } from './stores/auth.js'

const app = createApp(App)
app.use(router)
app.mount('#app')

// Restore operator session (if a token was kept) without blocking first paint.
refreshMe()
