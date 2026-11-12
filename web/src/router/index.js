import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import ChatView from '../views/ChatView.vue'
import ProductInputView from '../views/ProductInputView.vue'
import ProductReadView from '../views/ProductReadView.vue'
import ContentReviewView from '../views/ContentReviewView.vue'
import PublishQueueView from '../views/PublishQueueView.vue'
import SettingsView from '../views/SettingsView.vue'
import ConnectViews from '../views/ConnectViews.vue'
import OpsViews from '../views/OpsViews.vue'

// 15 views in 6 groups (§8). Names are stable for menu wiring.
const routes = [
  { path: '/', name: 'home', component: HomeView },
  { path: '/chat', name: 'chat', component: ChatView },
  { path: '/products', name: 'products', component: ProductInputView },
  { path: '/readings', name: 'readings', component: ProductReadView },
  { path: '/review', name: 'review', component: ContentReviewView },
  { path: '/queue', name: 'queue', component: PublishQueueView },
  { path: '/metrics', name: 'metrics', component: OpsViews, props: { view: 'metrics' } },
  { path: '/agents', name: 'agents', component: ConnectViews, props: { view: 'agents' } },
  { path: '/channels', name: 'channels', component: ConnectViews, props: { view: 'channels' } },
  { path: '/owned', name: 'owned', component: ConnectViews, props: { view: 'owned' } },
  { path: '/criteria', name: 'criteria', component: OpsViews, props: { view: 'criteria' } },
  { path: '/resident', name: 'resident', component: OpsViews, props: { view: 'resident' } },
  { path: '/system', name: 'system', component: OpsViews, props: { view: 'system' } },
  { path: '/mobile', name: 'mobile', component: OpsViews, props: { view: 'mobile' } },
  { path: '/settings', name: 'settings', component: SettingsView },
]

export const menuGroups = [
  { label: '홈', items: [{ to: '/', label: '대시보드' }] },
  {
    label: '작업',
    items: [
      { to: '/chat', label: '대화' },
      { to: '/products', label: '상품 투입' },
      { to: '/readings', label: '상품 판독' },
      { to: '/review', label: '콘텐츠 검수' },
      { to: '/queue', label: '발행 큐' },
    ],
  },
  { label: '성과', items: [{ to: '/metrics', label: '지표' }] },
  {
    label: '연결',
    items: [
      { to: '/agents', label: '에이전트 연결' },
      { to: '/channels', label: '채널 계정' },
      { to: '/owned', label: '자체 채널' },
    ],
  },
  { label: '총괄', items: [{ to: '/criteria', label: '판정·기준' }] },
  {
    label: '시스템',
    items: [
      { to: '/resident', label: 'Muse 상주 관리' },
      { to: '/system', label: '시스템 구성' },
      { to: '/mobile', label: '모바일 앱' },
      { to: '/settings', label: '설정' },
    ],
  },
]

export default createRouter({
  history: createWebHistory(),
  routes,
})
