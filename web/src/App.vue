<template>
  <template v-if="ready">
    <AppShell v-if="shellOn" :title="title">
      <RouterView />
    </AppShell>
    <RouterView v-else />
  </template>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppShell from './components/AppShell.vue'

const route = useRoute()
const router = useRouter()

const TITLES = {
  '/': '홈',
  '/new': '새 작업',
  '/works': '작업',
  '/media': '미디어',
  '/teams': '팀',
  '/connect': '연결',
  '/settings': '설정',
}

// 렌더를 첫 내비게이션 완료까지 미룬다. 안 그러면 가드 해결 전 START
// 위치(path '/')로 셸이 한 틱 먼저 붙어 토큰 없이 API를 쏜다.
const ready = ref(false)
router.isReady().then(() => {
  ready.value = true
})

const shellOn = computed(() => route.path !== '/login' && route.path !== '/brand' && !route.path.startsWith('/promo') && route.name !== 'not-found')
const title = computed(() => {
  if (route.path.startsWith('/works/')) return '작업 상세'
  if (route.path.startsWith('/teams/')) return '팀 상세'
  return TITLES[route.path] || ''
})
</script>
