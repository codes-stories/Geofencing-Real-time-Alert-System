import React, { useEffect, useRef } from 'react'
import { useAlerts } from '../context/AlertsContext'
import AlertFeed from './AlertFeed'

function ConnectionIndicator({ status }) {
  const isLive = status === 'live'
  return (
    <div
      className={`flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold ${
        isLive ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'
      }`}
      title={isLive ? 'Connected to live alert stream' : 'Reconnecting to alert stream'}
    >
      <span className="relative flex h-2.5 w-2.5">
        <span
          className={`absolute inline-flex h-full w-full rounded-full ${
            isLive ? 'animate-ping bg-green-400 opacity-75' : 'bg-red-400'
          }`}
        />
        <span
          className={`relative inline-flex h-2.5 w-2.5 rounded-full ${
            isLive ? 'bg-green-500' : 'bg-red-500'
          }`}
        />
      </span>
      <span className="hidden sm:inline">{isLive ? 'Live' : 'Reconnecting'}</span>
    </div>
  )
}

export default function Header() {
  const { wsStatus, unreadCount, feedOpen, toggleFeed, alerts } = useAlerts()
  const containerRef = useRef(null)

  useEffect(() => {
    if (!feedOpen) return
    function onDocClick(event) {
      if (containerRef.current && !containerRef.current.contains(event.target)) {
        toggleFeed()
      }
    }
    function onKey(event) {
      if (event.key === 'Escape') toggleFeed()
    }
    document.addEventListener('mousedown', onDocClick)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDocClick)
      document.removeEventListener('keydown', onKey)
    }
  }, [feedOpen, toggleFeed])

  return (
    <header className="sticky top-0 z-[1000] border-b border-slate-200 bg-white shadow-sm">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between gap-3 px-3 sm:px-4">
        <div className="flex min-w-0 items-center gap-2 sm:gap-3">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-blue-600 text-sm font-bold text-white">
            G
          </div>
          <h1 className="truncate text-base font-semibold text-slate-800 sm:text-lg">
            Geofence Alert System
          </h1>
        </div>

        <div className="flex items-center gap-2 sm:gap-3">
          <ConnectionIndicator status={wsStatus} />

          <div className="relative" ref={containerRef}>
            <button
              type="button"
              onClick={toggleFeed}
              className={`relative rounded-md p-2 transition-colors ${
                feedOpen ? 'bg-blue-50 text-blue-600' : 'text-slate-500 hover:bg-slate-100 hover:text-slate-700'
              }`}
              aria-label={`Live alerts${unreadCount ? `, ${unreadCount} unread` : ''}`}
              aria-expanded={feedOpen}
              aria-haspopup="true"
            >
              <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
                />
              </svg>
              {unreadCount > 0 && (
                <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-[1rem] items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-bold leading-none text-white">
                  {unreadCount > 99 ? '99+' : unreadCount}
                </span>
              )}
            </button>

            {feedOpen && <AlertFeed alerts={alerts} onClose={toggleFeed} />}
          </div>
        </div>
      </div>
    </header>
  )
}
