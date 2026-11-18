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
import PromoHomeView from '../views/PromoHomeView.vue'
import PromoLoginView from '../views/PromoLoginView.vue'
import SignupView from '../views/SignupView.vue'
import PromoProductsView from '../views/PromoProductsView.vue'
import PromoProductDetailView from '../views/PromoProductDetailView.vue'
import PromoGuideView from '../views/PromoGuideView.vue'
import PromoMeView from '../views/PromoMeView.vue'
import PromoWorkView from '../views/PromoWorkView.vue'
import PromoAdminView from '../views/PromoAdminView.vue'
import PromoTermsView from '../views/PromoTermsView.vue'
import PromoPrivacyView from '../views/PromoPrivacyView.vue'
import NotFoundView from '../views/NotFoundView.vue'
import BrandView from '../views/BrandView.vue'
import { auth, refreshMe } from '../stores/auth.js'
import { member, memberRefresh } from '../stores/member.js'

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
  // W19: pre-rebuild bookmarks land on their replacement screen.
  { path: '/review', redirect: { name: 'works' } },
  { path: '/queue', redirect: { name: 'works', query: { tab: 'queued' } } },
  { path: '/readings', redirect: { name: 'works' } },
  { path: '/products', redirect: { name: 'new-work' } },
  { path: '/chat', redirect: { name: 'connect' } },
  { path: '/agents', redirect: { name: 'connect' } },
  { path: '/channels', redirect: { name: 'connect' } },
  { path: '/owned', redirect: { name: 'connect' } },
  { path: '/resident', redirect: { name: 'teams' } },
  { path: '/criteria', redirect: { name: 'settings' } },
  { path: '/system', redirect: { name: 'settings' } },
  { path: '/metrics', redirect: { name: 'home' } },
  { path: '/mobile', redirect: { name: 'home' } },
  // W24 public consignment surface (no operator login needed).
  { path: '/promo', name: 'promo-home', component: PromoHomeView },
  { path: '/promo/login', name: 'promo-login', component: PromoLoginView },
  { path: '/promo/signup', name: 'promo-signup', component: SignupView },
  { path: '/promo/products', name: 'promo-products', component: PromoProductsView },
  { path: '/promo/products/:id', name: 'promo-product-detail', component: PromoProductDetailView },
  { path: '/promo/guide', name: 'promo-guide', component: PromoGuideView },
  { path: '/promo/me', name: 'promo-me', component: PromoMeView },
  { path: '/promo/terms', name: 'promo-terms', component: PromoTermsView },
  { path: '/promo/privacy', name: 'promo-privacy', component: PromoPrivacyView },
  { path: '/promo/work', name: 'promo-work', component: PromoWorkView },
  { path: '/promo/admin', name: 'promo-admin', component: PromoAdminView },
  // Brand homepage: public, no login needed.
  { path: '/brand', name: 'brand', component: BrandView },
  { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundView },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to) => {
  if (to.path.startsWith('/promo')) {
    if (to.path === '/promo/login' || to.path === '/promo/signup') {
      if (!member.token) return true
      const me = member.user || (await memberRefresh())
      return me ? (typeof to.query.next === 'string' && to.query.next.startsWith('/promo') ? to.query.next : '/promo') : true
    }
    if (to.path === '/promo/me' || to.path === '/promo/work') {
      if (!member.token) return { path: '/promo/login', query: { next: to.fullPath } }
      if (!member.user) {
        const me = await memberRefresh()
        if (!me) return { path: '/promo/login', query: { next: to.fullPath } }
      }
    }
    if (to.path === '/promo/admin') {
      if (!auth.token) return { path: '/login', query: { next: to.fullPath } }
      if (!auth.user) {
        const user = await refreshMe()
        if (!user) return { path: '/login', query: { next: to.fullPath } }
      }
    }
    return true
  }
  if (to.path === '/brand') return true
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
