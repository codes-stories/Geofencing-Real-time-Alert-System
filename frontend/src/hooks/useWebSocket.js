import { useCallback, useEffect, useRef, useState } from 'react'
import { wsUrl } from '../api/client'

const MAX_BACKOFF = 10000

export default function useWebSocket(onMessage) {
  const [status, setStatus] = useState('connecting') // 'live' | 'reconnecting' | 'connecting'
  const wsRef = useRef(null)
  const onMessageRef = useRef(onMessage)
  const attemptRef = useRef(0)
  const timerRef = useRef(null)
  const closedRef = useRef(false)

  onMessageRef.current = onMessage

  const connect = useCallback(() => {
    if (closedRef.current) return
    let ws
    try {
      ws = new WebSocket(wsUrl())
    } catch {
      scheduleReconnect()
      return
    }
    wsRef.current = ws

    ws.onopen = () => {
      attemptRef.current = 0
      setStatus('live')
    }
    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        if (data && typeof data === 'object') {
          onMessageRef.current?.(data)
        }
      } catch {
        // ignore non-JSON messages (e.g. pongs)
      }
    }
    ws.onerror = () => {
      // onclose will fire afterwards and handle reconnection
    }
    ws.onclose = () => {
      if (closedRef.current) return
      setStatus('reconnecting')
      scheduleReconnect()
    }
  }, [])

  const scheduleReconnect = useCallback(() => {
    if (closedRef.current) return
    const attempt = attemptRef.current
    const delay = Math.min(1000 * 2 ** attempt, MAX_BACKOFF)
    attemptRef.current = attempt + 1
    clearTimeout(timerRef.current)
    timerRef.current = setTimeout(connect, delay)
  }, [connect])

  useEffect(() => {
    closedRef.current = false
    connect()
    return () => {
      closedRef.current = true
      clearTimeout(timerRef.current)
      attemptRef.current = 0
      const ws = wsRef.current
      wsRef.current = null
      if (ws) {
        ws.onopen = null
        ws.onmessage = null
        ws.onerror = null
        ws.onclose = null
        try {
          ws.close()
        } catch {
          // no-op
        }
      }
      setStatus('connecting')
    }
  }, [connect])

  return status
}
