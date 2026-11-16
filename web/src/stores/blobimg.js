// Authed blob image loader: /api image paths need the operator token,
// which plain <img src> cannot send. One cache per path; blob: URLs and
// data: URIs pass through untouched.
import { auth } from './auth.js'

const cache = new Map()

export function isBlobable(src) {
  return typeof src === 'string' && src.startsWith('/api/')
}

export async function blobUrl(src) {
  if (!src || !isBlobable(src)) return src || ''
  if (cache.has(src)) return cache.get(src)
  const headers = {}
  if (auth.token) headers['Authorization'] = `Bearer ${auth.token}`
  const res = await fetch(src, { headers })
  if (!res.ok) throw new Error(`request failed (${res.status})`)
  const url = URL.createObjectURL(await res.blob())
  cache.set(src, url)
  return url
}

export function dropCache() {
  for (const url of cache.values()) URL.revokeObjectURL(url)
  cache.clear()
}
