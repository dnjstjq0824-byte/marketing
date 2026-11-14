// Toast host state (UI_DESIGN §1-3): bottom center, max 3.
import { reactive } from 'vue'

let seq = 1
export const toasts = reactive([])

export function toast(message, kind = 'ok', ms = 0) {
  const id = seq++
  const item = { id, message, kind }
  toasts.push(item)
  while (toasts.length > 3) toasts.shift()
  if (kind !== 'error') {
    setTimeout(() => dismiss(id), ms || 3000)
  }
  return id
}

export function dismiss(id) {
  const i = toasts.findIndex((t) => t.id === id)
  if (i >= 0) toasts.splice(i, 1)
}
