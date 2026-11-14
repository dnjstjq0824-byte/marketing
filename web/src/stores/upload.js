// Chunked upload queue (UI_DESIGN S2): init → parts → complete,
// per-file progress, cancel, and failed-chunk retry.
import { reactive } from 'vue'
import { apiFetch, auth } from './auth.js'

const CHUNK = 5 * 1024 * 1024

export const uploads = reactive([])

let seq = 1

function row(file) {
  return {
    id: seq++,
    file,
    name: file.name,
    size: file.size,
    uploadId: '',
    done: 0,
    total: Math.max(1, Math.ceil(file.size / CHUNK)),
    state: 'queued', // queued|active|paused|done|error|cancelled
    error: '',
    mediaId: null,
    cancelled: false,
  }
}

export function queueFiles(files) {
  const rows = [...files].map((f) => {
    const r = row(f)
    uploads.push(r)
    return r
  })
  pump()
  return rows
}

export function cancelUpload(id) {
  const r = uploads.find((x) => x.id === id)
  if (r && (r.state === 'queued' || r.state === 'active')) {
    r.cancelled = true
    r.state = 'cancelled'
  }
}

export function removeUpload(id) {
  const i = uploads.findIndex((x) => x.id === id)
  if (i >= 0) uploads.splice(i, 1)
}

export function retryUpload(id) {
  const r = uploads.find((x) => x.id === id)
  if (!r || (r.state !== 'error' && r.state !== 'cancelled')) return
  r.cancelled = false
  r.error = ''
  r.state = 'queued'
  pump()
}

let pumping = false

async function pump() {
  if (pumping) return
  pumping = true
  try {
    for (;;) {
      const r = uploads.find((x) => x.state === 'queued')
      if (!r) return
      r.state = 'active'
      try {
        await runRow(r)
      } catch (e) {
        if (!r.cancelled) {
          r.state = 'error'
          r.error = e.message || '업로드 실패'
        }
      }
    }
  } finally {
    pumping = false
  }
}

async function runRow(r) {
  if (!r.uploadId) {
    const init = await apiFetch('/api/media/uploads', {
      method: 'POST',
      body: JSON.stringify({ name: r.name, size: r.size, type: r.file.type || '' }),
    })
    r.uploadId = init.id
    r.done = init.doneParts || 0
  }
  for (let n = r.done + 1; n <= r.total; n++) {
    if (r.cancelled) return
    const blob = r.file.slice((n - 1) * CHUNK, n * CHUNK)
    let ok = false
    let lastErr = null
    for (let attempt = 0; attempt < 3 && !ok; attempt++) {
      if (r.cancelled) return
      try {
        await putPart(r.uploadId, n, blob)
        ok = true
      } catch (e) {
        lastErr = e
      }
    }
    if (!ok) throw lastErr || new Error('업로드 실패')
    r.done = n
  }
  if (r.cancelled) return
  const done = await apiFetch(`/api/media/uploads/${r.uploadId}/complete`, { method: 'POST' })
  r.mediaId = done.id
  r.state = 'done'
}

async function putPart(uploadId, n, blob) {
  const headers = { 'Content-Type': 'application/octet-stream' }
  if (auth.token) headers['Authorization'] = `Bearer ${auth.token}`
  const res = await fetch(`/api/media/uploads/${uploadId}/parts/${n}`, { method: 'PUT', headers, body: blob })
  if (!res.ok) {
    let message = `request failed (${res.status})`
    try {
      const data = await res.json()
      if (data && data.error) message = data.error
    } catch { /* keep default */ }
    throw { status: res.status, message }
  }
}
