import React, { createContext, useCallback, useContext, useMemo, useRef, useState } from 'react'
import useWebSocket from '../hooks/useWebSocket'
import { useToast } from '../components/Toast'

const AlertsContext = createContext(null)

const MAX_FEED = 50

function isAlertEvent(data) {
  return (
    data &&
    typeof data === 'object' &&
    typeof data.event_type === 'string' &&
    data.vehicle &&
    data.geofence
  )
}

export function AlertsProvider({ children }) {
  const [alerts, setAlerts] = useState([])
  const [unreadCount, setUnreadCount] = useState(0)
  const [feedOpen, setFeedOpen] = useState(false)
  const { toast } = useToast()
  const toastFnRef = useRef(toast)
  toastFnRef.current = toast

  const handleWsMessage = useCallback((data) => {
    if (!isAlertEvent(data)) return
    setAlerts((prev) => [data, ...prev].slice(0, MAX_FEED))
    setUnreadCount((c) => c + 1)
    toastFnRef.current?.(null, { type: data.event_type === 'exit' ? 'exit' : 'entry', event: data })
  }, [])

  const wsStatus = useWebSocket(handleWsMessage)

  const openFeed = useCallback(() => {
    setFeedOpen(true)
    setUnreadCount(0)
  }, [])

  const closeFeed = useCallback(() => setFeedOpen(false), [])

  const toggleFeed = useCallback(() => {
    setFeedOpen((open) => {
      if (!open) setUnreadCount(0)
      return !open
    })
  }, [])

  const value = useMemo(
    () => ({
      alerts,
      unreadCount,
      feedOpen,
      wsStatus,
      openFeed,
      closeFeed,
      toggleFeed,
    }),
    [alerts, unreadCount, feedOpen, wsStatus, openFeed, closeFeed, toggleFeed],
  )

  return <AlertsContext.Provider value={value}>{children}</AlertsContext.Provider>
}

export function useAlerts() {
  const ctx = useContext(AlertsContext)
  if (!ctx) throw new Error('useAlerts must be used within AlertsProvider')
  return ctx
}
