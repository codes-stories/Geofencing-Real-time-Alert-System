export const BASE = (import.meta.env.VITE_API_BASE || '').replace(/\/$/, '')

async function request(path, options = {}) {
  const url = `${BASE}${path}`
  let res
  try {
    res = await fetch(url, {
      headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
      ...options,
    })
  } catch (err) {
    throw new Error('Network error — could not reach the server. Please try again.')
  }

  let data = null
  const text = await res.text()
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = null
    }
  }

  if (!res.ok) {
    const message =
      (data && data.error) || `Request failed with status ${res.status}`
    const error = new Error(message)
    error.status = res.status
    throw error
  }

  return data
}

export function get(path, params) {
  let query = ''
  if (params) {
    const search = new URLSearchParams()
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && value !== '') {
        search.set(key, value)
      }
    })
    const qs = search.toString()
    if (qs) query = `?${qs}`
  }
  return request(`${path}${query}`, { method: 'GET' })
}

export function post(path, body) {
  return request(path, {
    method: 'POST',
    body: JSON.stringify(body ?? {}),
  })
}

export function wsUrl() {
  if (!BASE) {
    const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
    return `${proto}://${window.location.host}/ws/alerts`
  }
  const wsBase = BASE.replace(/^http/, 'ws')
  return `${wsBase}/ws/alerts`
}
