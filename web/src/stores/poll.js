// One poll loop per screen (UI_DESIGN §1-3): fast while work runs,
// slow when idle, paused in background tabs.
import { onUnmounted, ref } from 'vue'

export function usePoll(fn, { fastMs = 5000, slowMs = 30000, isBusy = () => false } = {}) {
  const timer = ref(null)
  const stopped = ref(false)

  async function tick() {
    if (stopped.value) return
    if (!document.hidden) {
      try {
        await fn()
      } catch {
        /* next tick retries; screens show their own error state */
      }
    }
    if (!stopped.value) {
      timer.value = setTimeout(tick, isBusy() ? fastMs : slowMs)
    }
  }

  function start() {
    stopped.value = false
    tick()
  }

  function stop() {
    stopped.value = true
    if (timer.value) {
      clearTimeout(timer.value)
      timer.value = null
    }
  }

  onUnmounted(stop)
  return { start, stop }
}
