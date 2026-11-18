import { computed, ref } from 'vue'

/** 검색·정렬·페이지네이션 공용 로직. getRows는 반응형 행 배열을 돌려준다. */
export function useTable(getRows, keys, pageSize = 10) {
  const q = ref('')
  const sortKey = ref('')
  const sortDir = ref(1)
  const page = ref(1)

  const filtered = computed(() => {
    const rows = getRows()
    const needle = q.value.trim().toLowerCase()
    if (!needle) return rows
    return rows.filter((r) => keys.some((k) => String(r[k] ?? '').toLowerCase().includes(needle)))
  })

  const sorted = computed(() => {
    const rows = [...filtered.value]
    if (!sortKey.value) return rows
    const k = sortKey.value
    rows.sort((a, b) => {
      const x = a[k] ?? ''
      const y = b[k] ?? ''
      const d = typeof x === 'number' && typeof y === 'number' ? x - y : String(x).localeCompare(String(y), 'ko')
      return d * sortDir.value
    })
    return rows
  })

  const pages = computed(() => Math.max(1, Math.ceil(sorted.value.length / pageSize)))
  const safePage = computed(() => Math.min(page.value, pages.value))
  const rows = computed(() => sorted.value.slice((safePage.value - 1) * pageSize, safePage.value * pageSize))

  function toggleSort(k) {
    if (sortKey.value === k) sortDir.value = -sortDir.value
    else {
      sortKey.value = k
      sortDir.value = 1
    }
  }

  function reset() {
    page.value = 1
  }

  return { q, sortKey, sortDir, page: safePage, setPage: (p) => { page.value = p }, pages, rows, total: computed(() => sorted.value.length), toggleSort, reset }
}
