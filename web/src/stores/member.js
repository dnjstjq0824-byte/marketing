// Public member-auth state + JSON API helper for /promo views.
// Separate token from the operator console: the two logins never share.
import { reactive } from 'vue'

const TOKEN_KEY = 'somun_member_token'

export const member = reactive({
  token: localStorage.getItem(TOKEN_KEY) || '',
  user: null,
})

export function setMemberSession(token, user) {
  member.token = token
  member.user = user
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

/** Fetch JSON from the promo API. Throws {status, message} on error. */
export async function promoFetch(path, opts = {}) {
  const headers = { ...(opts.headers || {}) }
  const isForm = opts.body instanceof FormData
  if (!isForm && opts.body !== undefined && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json'
  }
  if (member.token) headers['Authorization'] = `Bearer ${member.token}`
  const res = await fetch(path, { ...opts, headers })
  let data = null
  try {
    data = await res.json()
  } catch {
    data = null
  }
  if (!res.ok) {
    const message = (data && data.error) || `request failed (${res.status})`
    if (res.status === 401) setMemberSession('', null)
    throw { status: res.status, message }
  }
  return data
}

export async function memberSignup(email, password, nickname, inviteCode) {
  const data = await promoFetch('/api/promo/signup', {
    method: 'POST',
    body: JSON.stringify({ email, password, nickname, inviteCode }),
  })
  setMemberSession(data.token, data.member)
  return data.member
}

export async function memberLogin(email, password) {
  const data = await promoFetch('/api/promo/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
  setMemberSession(data.token, data.member)
  return data.member
}

export async function memberLogout() {
  try {
    await promoFetch('/api/promo/logout', { method: 'POST' })
  } catch {
    /* session already gone */
  }
  setMemberSession('', null)
}

export async function memberRefresh() {
  if (!member.token) return null
  try {
    const me = await promoFetch('/api/promo/me')
    member.user = { id: me.id, email: me.email, nickname: me.nickname }
    return member.user
  } catch {
    setMemberSession('', null)
    return null
  }
}

/** Per-order fee line. Percent is never shown: fixed won per order only. */
export function formatFee(n) {
  return `주문 1건당 ${Number(n || 0).toLocaleString('ko-KR')}원`
}

/** Storefront categories (admin picks one per product). */
export const CATEGORIES = ['뷰티', '패션', '식품', '생활', '건강', '디지털', '육아', '반려동물', '여행', '기타']

/** D-day line for a YYYY-MM-DD deadline. Empty means always open. */
export function ddayUntil(iso) {
  if (!iso) return { text: '상시 모집', over: true }
  const end = new Date(iso + 'T00:00:00')
  if (Number.isNaN(end.getTime())) return { text: '상시 모집', over: true }
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const days = Math.round((end.getTime() - today.getTime()) / 86400000)
  if (days < 0) return { text: '마감일 지남', over: true }
  if (days === 0) return { text: '오늘 마감', over: false }
  return { text: `${days}일 남음`, over: false }
}

export function formatWon(n) {
  return `${Number(n || 0).toLocaleString('ko-KR')}원`
}
