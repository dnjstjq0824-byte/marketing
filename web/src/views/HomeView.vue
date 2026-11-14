<template>
  <div class="home">
    <ActionBanner :actions="data.actions" />
    <section v-if="showChecklist" class="card check" aria-label="시작 체크리스트">
      <h2 class="section-title">시작하기</h2>
      <ol>
        <li><RouterLink to="/connect">AI 계정 연결하기</RouterLink><StatusChip :kind="data.firstRun.aiConnected ? 'ok' : 'hold'">{{ data.firstRun.aiConnected ? '됨' : '안 됨' }}</StatusChip></li>
        <li><RouterLink to="/connect">발행 채널 연결하기</RouterLink><StatusChip :kind="data.firstRun.channelConnected ? 'ok' : 'hold'">{{ data.firstRun.channelConnected ? '됨' : '안 됨' }}</StatusChip></li>
        <li><RouterLink to="/new">첫 작업 만들기</RouterLink><StatusChip :kind="data.firstRun.hasWork ? 'ok' : 'hold'">{{ data.firstRun.hasWork ? '됨' : '안 됨' }}</StatusChip></li>
      </ol>
    </section>
    <Skeleton v-if="loading && !data.today" :rows="4" />
    <div v-else-if="loadError" class="card center">
      <p>불러오지 못했습니다.</p>
      <button @click="load">다시 시도</button>
    </div>
    <template v-else-if="data.today">
      <section class="stats" aria-label="오늘 현황">
        <RouterLink to="/works" class="stat"><b class="num">{{ data.today.running }}</b><span>진행 중</span></RouterLink>
        <RouterLink to="/works?tab=published" class="stat"><b class="num">{{ data.today.publishedToday }}</b><span>오늘 발행</span></RouterLink>
        <RouterLink to="/works?tab=queued" class="stat"><b class="num">{{ data.today.queued }}</b><span>발행 대기</span></RouterLink>
        <div class="stat"><b class="num">{{ data.today.weekViews }}</b><span>이번 주 조회</span></div>
        <RouterLink to="/works?tab=failed" class="stat"><b class="num">{{ data.today.failed }}</b><span>실패</span></RouterLink>
      </section>
      <section aria-label="진행 중인 작업">
        <div class="sect-head"><h2 class="section-title">진행 중</h2><RouterLink to="/works" class="more">전체 보기</RouterLink></div>
        <div v-if="data.running.length" class="cards">
          <WorkCard v-for="w in data.running" :key="w.id" :work="w" />
        </div>
        <EmptyState v-else title="진행 중인 작업이 없습니다" hint="새 작업을 만들면 여기서 진행됩니다" to="/new" actionLabel="새 작업" />
      </section>
      <section v-if="data.recentPosts.length" aria-label="최근 발행">
        <div class="sect-head"><h2 class="section-title">최근 발행</h2><RouterLink to="/works?tab=published" class="more">전체 보기</RouterLink></div>
        <div class="cards">
          <PostCard v-for="p in data.recentPosts" :key="p.id" :post="p" />
        </div>
      </section>
      <section v-if="data.recentMedia.length" aria-label="최근 미디어">
        <div class="sect-head"><h2 class="section-title">최근 미디어</h2><RouterLink to="/media" class="more">전체 보기</RouterLink></div>
        <div class="tiles">
          <MediaTile v-for="m in data.recentMedia" :key="m.id" :item="m" />
        </div>
      </section>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { apiFetch } from '../stores/auth.js'
import { usePoll } from '../stores/poll.js'
import ActionBanner from '../components/ActionBanner.vue'
import EmptyState from '../components/EmptyState.vue'
import MediaTile from '../components/MediaTile.vue'
import PostCard from '../components/PostCard.vue'
import Skeleton from '../components/Skeleton.vue'
import StatusChip from '../components/StatusChip.vue'
import WorkCard from '../components/WorkCard.vue'

const data = ref({ actions: [], today: null, running: [], recentPosts: [], recentMedia: [], firstRun: {} })
const loading = ref(true)
const loadError = ref(false)

const showChecklist = computed(() => {
  const f = data.value.firstRun
  return f && (!f.aiConnected || !f.channelConnected || !f.hasWork)
})

async function load() {
  loadError.value = false
  try {
    const res = await apiFetch('/api/overview')
    data.value = {
      actions: res.actions || [],
      today: res.today,
      running: res.running || [],
      recentPosts: res.recentPosts || [],
      recentMedia: res.recentMedia || [],
      firstRun: res.firstRun || {},
    }
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
}

const poll = usePoll(load, { isBusy: () => (data.value.today?.running || 0) > 0 })
onMounted(() => poll.start())
</script>

<style scoped>
.home { display: flex; flex-direction: column; gap: var(--s5); }
.card {
  background: var(--panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-lg);
  padding: var(--s4);
  box-shadow: var(--shadow);
  display: flex;
  flex-direction: column;
  gap: var(--s2);
}
.check { display: block; text-align: left; }
.check ol { margin: var(--s2) 0 0; padding-left: var(--s5); display: flex; flex-direction: column; gap: var(--s2); }
.check li { display: flex; align-items: center; gap: var(--s3); }
.sect-head { display: flex; align-items: center; margin-bottom: var(--s3); }
.sect-head h2 { flex: 1; }
.more { font-size: 13px; }
.center { text-align: center; }
.center button {
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: var(--r-md);
  padding: var(--s2) var(--s4);
  margin-top: var(--s2);
}
.stats { display: grid; grid-template-columns: repeat(5, 1fr); gap: var(--s3); }
.stat {
  background: var(--panel);
  border: 1px solid var(--line-soft);
  border-radius: var(--r-lg);
  padding: var(--s4) var(--s2);
  text-align: center;
  text-decoration: none;
  color: var(--ink);
  box-shadow: var(--shadow);
  display: flex;
  flex-direction: column;
  gap: var(--s1);
}
.stat b { font-size: 24px; }
.stat span { font-size: 13px; color: var(--muted); }
.cards { display: flex; flex-direction: column; gap: var(--s3); }
.tiles { display: grid; grid-template-columns: repeat(6, 1fr); gap: var(--s2); }
@media (max-width: 900px) {
  .stats { grid-template-columns: repeat(5, 1fr); gap: var(--s2); }
  .stat { padding: var(--s3) var(--s1); }
  .stat b { font-size: 18px; }
  .tiles { grid-template-columns: repeat(3, 1fr); }
}
</style>
