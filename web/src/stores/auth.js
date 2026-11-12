// Shared operator-auth state + JSON API helper for all views.
// No external store library: a tiny module singleton is enough for tranche 1.
import { reactive } from 'vue'

const TOKEN_KEY = 'somun_operator_token'

export const auth = reactive({
  token: localStorage.getItem(TOKEN_KEY) || '',
  user: null,
})

export function setSession(token, user) {
  auth.token = token
  auth.user = user
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

/** Fetch JSON from the console API. Throws {status, message} on error. */
export async function apiFetch(path, opts = {}) {
  const headers = { ...(opts.headers || {}) }
  const isForm = opts.body instanceof FormData
  if (!isForm && opts.body !== undefined && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json'
  }
  if (auth.token) headers['Authorization'] = `Bearer ${auth.token}`
  const res = await fetch(path, { ...opts, headers })
  let data = null
  try {
    data = await res.json()
  } catch {
    data = null
  }
  if (!res.ok) {
    const message = (data && data.error) || `request failed (${res.status})`
    if (res.status === 401) setSession('', null)
    throw { status: res.status, message }
  }
  return data
}

export async function login(username, password) {
  const data = await apiFetch('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })
  setSession(data.token, data.user)
  return data.user
}

export async function logout() {
  try {
    await apiFetch('/api/auth/logout', { method: 'POST' })
  } catch {
    /* session already gone */
  }
  setSession('', null)
}

export async function refreshMe() {
  if (!auth.token) return null
  try {
    const me = await apiFetch('/api/auth/me')
    auth.user = { id: me.id, username: me.username }
    return auth.user
  } catch {
    setSession('', null)
    return null
  }
}
