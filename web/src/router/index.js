import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import LoginView from '../views/LoginView.vue'
import NewWorkView from '../views/NewWorkView.vue'
import WorksView from '../views/WorksView.vue'
import WorkDetailView from '../views/WorkDetailView.vue'
import MediaView from '../views/MediaView.vue'
import ConnectView from '../views/ConnectView.vue'
import TeamsView from '../views/TeamsView.vue'
import TeamDetailView from '../views/TeamDetailView.vue'
import SettingsView from '../views/SettingsView.vue'
import { auth, refreshMe } from '../stores/auth.js'

// 7 screens (UI_DESIGN §3-1) + login. Removed paths land on home (§8).
const routes = [
  { path: '/login', name: 'login', component: LoginView },
  { path: '/', name: 'home', component: HomeView },
  { path: '/new', name: 'new-work', component: NewWorkView },
  { path: '/works', name: 'works', component: WorksView },
  { path: '/works/:id', name: 'work-detail', component: WorkDetailView },
  { path: '/media', name: 'media', component: MediaView },
  { path: '/connect', name: 'connect', component: ConnectView },
  { path: '/teams', name: 'teams', component: TeamsView },
  { path: '/teams/:id', name: 'team-detail', component: TeamDetailView },
  { path: '/settings', name: 'settings', component: SettingsView },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to) => {
  if (to.path === '/login') {
    if (!auth.token) return true
    const user = auth.user || (await refreshMe())
    return user ? '/' : true
  }
  if (!auth.token) return { path: '/login', query: { next: to.fullPath } }
  if (!auth.user) {
    const user = await refreshMe()
    if (!user) return { path: '/login', query: { next: to.fullPath } }
  }
  return true
})

export default router
