<template>
  <div class="wcal">
    <div v-if="!loaded && !loadError" class="card center"><Skeleton :rows="3" /></div>
    <div v-else-if="loadError" class="card center">
      <p>달력을 불러오지 못했습니다. <button class="ghost small" @click="load">다시 시도</button></p>
    </div>
    <template v-else>
      <div class="card">
        <Calendar expanded borderless transparent locale="ko" :attributes="attrs" @dayclick="onDay" />
        <p class="legend"><span class="k sched">예약</span><span class="k done">발행</span>
          <span v-if="unsched" class="un">일정 없음 {{ unsched }}건 <RouterLink :to="{ path: '/works', query: { tab: 'queued' } }">대기 보기</RouterLink></span>
        </p>
      </div>
      <div v-if="selDay" class="card">
        <h3>{{ selLabel }}</h3>
        <p v-if="dayLoading" class="muted">불러오는 중…</p>
        <ul v-else-if="dayItems.length" class="daylist">
          <li v-for="d in dayItems" :key="d.key">
            <span class="k" :class="d.kind">{{ d.kind === 'sched' ? '예약' : '발행' }}</span>
            <RouterLink v-if="d.to" :to="d.to">{{ d.title }}</RouterLink><span v-else>{{ d.title }}</span>
            <span class="muted small">{{ d.meta }}</span>
          </li>
        </ul>
        <p v-else class="muted">이 날 일정이 없습니다.</p>
      </div>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted } from 'vue'
import { Calendar } from 'v-calendar'
import 'v-calendar/style.css'
import Skeleton from '../Skeleton.vue'
import { dayKey, useWorkCalendar } from '../../lib/useWorkCalendar.js'

const { unsched, loaded, loadError, selDay, dayItems, dayLoading, schedDates, doneDates, load, selectDay } = useWorkCalendar()
const attrs = computed(() => [
  { key: 'sched', dates: schedDates.value, dot: { color: 'orange' } },
  { key: 'done', dates: doneDates.value, dot: { color: 'green' } },
  ...(selDay.value ? [{ key: 'sel', dates: [new Date(selDay.value + 'T12:00:00')], highlight: { color: 'orange', fillMode: 'light' } }] : []),
])
const selLabel = computed(() => {
  const [y, m, d] = selDay.value.split('-').map(Number)
  return `${y}년 ${m}월 ${d}일`
})
function onDay(day) {
  const key = `${day.year}-${String(day.month).padStart(2, '0')}-${String(day.day).padStart(2, '0')}`
  selectDay(key === selDay.value ? '' : key)
}
onMounted(load)
</script>

<style scoped>
.wcal { display: flex; flex-direction: column; gap: var(--s3); }
.card { background: var(--panel); border: 1px solid var(--line-soft); border-radius: var(--r-lg); padding: var(--s4); box-shadow: var(--shadow); }
.center { text-align: center; }
.card h3 { margin: 0 0 8px; font-size: 16px; }
.legend { display: flex; gap: 12px; align-items: center; margin: 12px 0 0; font-size: 13px; color: var(--muted); flex-wrap: wrap; }
.k { border-radius: 999px; padding: 1px 10px; font-size: 12px; font-weight: 700; white-space: nowrap; }
.k.sched { background: var(--accent-soft); color: var(--accent-dark); }
.k.done { background: var(--ok-soft); color: var(--ok); }
.un { margin-left: auto; }
.daylist { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 8px; }
.daylist li { display: flex; gap: 10px; align-items: baseline; flex-wrap: wrap; }
.ghost { border: 1px solid var(--line); background: var(--panel); color: var(--ink); border-radius: var(--r-md); padding: var(--s1) var(--s3); }
.small { font-size: 13px; }
</style>
