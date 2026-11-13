import React, { createContext, useCallback, useContext, useRef, useState } from 'react'

const ToastContext = createContext(null)

let nextId = 1

export function ToastProvider({ children }) {
  const [toasts, setToasts] = useState([])
  const timersRef = useRef({})

  const dismiss = useCallback((id) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
    if (timersRef.current[id]) {
      clearTimeout(timersRef.current[id])
      delete timersRef.current[id]
    }
  }, [])

  const toast = useCallback(
    (message, opts = {}) => {
      const id = nextId++
      const duration = opts.duration ?? 7000
      setToasts((prev) => [
        ...prev,
        {
          id,
          message,
          type: opts.type || 'info',
          title: opts.title,
          duration,
          data: opts.data,
          event: opts.event,
        },
      ])
      if (duration > 0) {
        timersRef.current[id] = setTimeout(() => dismiss(id), duration)
      }
      return id
    },
    [dismiss],
  )

  const value = { toast, dismiss, toasts }
  return <ToastContext.Provider value={value}>{children}</ToastContext.Provider>
}

export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}
