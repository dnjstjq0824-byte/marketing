// Signup field rules mirroring backend member/service.go.
// Client-side only: the server re-validates everything.

/** Email format check (single @, 5..120 chars). Empty means valid. */
export function emailError(email) {
  const v = (email || '').trim()
  if (!v) return '이메일을 입력하세요'
  if (v.length < 5 || v.length > 120) return '이메일 길이가 올바르지 않습니다'
  const at = v.indexOf('@')
  if (at <= 0 || at >= v.length - 1 || v.indexOf('@', at + 1) >= 0) return '이메일 형식이 올바르지 않습니다'
  return ''
}

/** Nickname is required, max 30 chars. Empty means valid. */
export function nicknameError(nickname) {
  const v = (nickname || '').trim()
  if (!v) return '닉네임을 입력하세요'
  if (v.length > 30) return '닉네임은 30자 이내로 입력하세요'
  return ''
}

/** Password needs 8+ chars. Empty means valid. */
export function passwordError(pw) {
  if (!pw) return '비밀번호를 입력하세요'
  if (pw.length < 8) return '비밀번호는 8자 이상 입력하세요'
  return ''
}

/** Confirm must match. Empty means valid. */
export function confirmError(pw, confirm) {
  if (!confirm) return '비밀번호를 한 번 더 입력하세요'
  if (pw !== confirm) return '비밀번호가 일치하지 않습니다'
  return ''
}

/** Invite code is required during beta. The server validates strictly. */
export function inviteError(code) {
  const v = (code || '').trim()
  if (!v) return '초대 코드를 입력하세요'
  if (v.length < 6) return '초대 코드 형식이 올바르지 않습니다'
  return ''
}

/** Strength 0..4 from character variety. */
export function passwordScore(pw) {
  if (!pw) return 0
  let s = 0
  if (/[a-z]/.test(pw)) s += 1
  if (/[A-Z]/.test(pw)) s += 1
  if (/[0-9]/.test(pw)) s += 1
  if (/[^a-zA-Z0-9]/.test(pw)) s += 1
  return s
}

export const STRENGTH_LABEL = ['입력하세요', '약함', '보통', '강함', '매우 강함']

/** Map promo signup server errors to short Korean copy. */
export function signupServerError(e) {
  if (e && e.status === 409) return '이미 가입된 이메일입니다'
  if (e && e.status === 403) return '초대 코드가 올바르지 않습니다'
  if (e && e.status === 429) return '시도가 너무 많습니다. 15분 뒤 다시 시도하세요'
  const m = (e && e.message) || ''
  if (m.includes('password')) return '비밀번호는 8자 이상 입력하세요'
  if (m.includes('email')) return '이메일 형식이 올바르지 않습니다'
  if (m.includes('nickname')) return '닉네임을 확인하세요(1~30자)'
  return '가입에 실패했습니다. 다시 시도하세요'
}
