import { computed, ref } from 'vue'
import { apiFetch } from '../stores/auth.js'

export function dayKey(d) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

export function useWorkCalendar() {
  const items = ref([])
  const unsched = ref(0)
  const loaded = ref(false)
  const loadError = ref(false)
  const selDay = ref('')
  const dayItems = ref([])
  const dayLoading = ref(false)

  const schedDates = computed(() => items.value.filter((i) => i.kind === 'sched').map((i) => i.at))
  const doneDates = computed(() => items.value.filter((i) => i.kind === 'done').map((i) => i.at))

  async function load() {
    loadError.value = false
    try {
      const [qc, logs] = await Promise.all([
        apiFetch('/api/contents?status=queued'),
        apiFetch('/api/publish-logs'),
      ])
      const out = []
      let lost = 0
      for (const c of qc.contents || []) {
        if (!c.scheduledAt) { lost++; continue }
        const when = new Date(c.scheduledAt)
        if (Number.isNaN(when.getTime())) { lost++; continue }
        out.push({ day: dayKey(when), kind: 'sched', at: when, contentId: c.id, title: c.title || '(제목 없음)', meta: `${c.channel || ''} · ${c.targetAccount || ''}`.trim(), productId: c.productId })
      }
      for (const l of logs.logs || []) {
        if (l.status !== 'done' || !l.executedAt) continue
        const when = new Date(l.executedAt)
        if (Number.isNaN(when.getTime())) continue
        out.push({ day: dayKey(when), kind: 'done', at: when, contentId: l.contentId, title: '', meta: `${l.channel || ''} · ${l.account || ''}`.trim(), productId: 0 })
      }
      items.value = out
      unsched.value = lost
      loaded.value = true
    } catch {
      loadError.value = true
    }
  }

  async function selectDay(key) {
    selDay.value = key
    dayItems.value = []
    if (!key) return
    dayLoading.value = true
    const rows = items.value.filter((it) => it.day === key).sort((a, b) => a.at - b.at).slice(0, 20)
    const out = []
    for (const r of rows) {
      if (selDay.value !== key) return
      let title = r.title
      let to = r.productId ? `/works/${r.productId}` : ''
      if (!title && r.contentId) {
        try {
          const c = await apiFetch(`/api/contents/${r.contentId}`)
          title = c.title || '(제목 없음)'
          if (c.productId) to = `/works/${c.productId}`
        } catch { title = '(제목 없음)' }
      }
      const hh = String(r.at.getHours()).padStart(2, '0')
      const mm = String(r.at.getMinutes()).padStart(2, '0')
      out.push({ key: `${r.kind}-${r.contentId}-${r.at.getTime()}`, kind: r.kind, title, meta: `${hh}:${mm} ${r.meta}`.trim(), to })
    }
    if (selDay.value !== key) return
    dayItems.value = out
    dayLoading.value = false
  }

  return { items, unsched, loaded, loadError, selDay, dayItems, dayLoading, schedDates, doneDates, load, selectDay }
}
