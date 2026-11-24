import { reactive } from 'vue'

const KEY = 'somun.theme'

function readSaved() {
  try {
    const v = localStorage.getItem(KEY)
    if (v === 'light' || v === 'dark' || v === 'system') return v
  } catch { /* private mode */ }
  return 'system'
}

export const theme = reactive({
  mode: readSaved(),
})

export function applyTheme(mode = theme.mode) {
  theme.mode = mode
  try {
    localStorage.setItem(KEY, mode)
  } catch { /* private mode */ }
  const root = document.documentElement
  if (mode === 'light' || mode === 'dark') {
    root.setAttribute('data-theme', mode)
  } else {
    root.removeAttribute('data-theme')
  }
}

export function cycleTheme() {
  const order = ['light', 'dark', 'system']
  const next = order[(order.indexOf(theme.mode) + 1) % order.length]
  applyTheme(next)
  return next
}
