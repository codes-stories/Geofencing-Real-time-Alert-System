import React from 'react'
import { useToast } from './Toast'

const typeStyles = {
  success: 'border-l-green-500',
  error: 'border-l-red-500',
  info: 'border-l-blue-500',
  entry: 'border-l-green-500',
  exit: 'border-l-red-500',
}

const badgeStyles = {
  entry: 'bg-green-100 text-green-700',
  exit: 'bg-red-100 text-red-700',
}

function AlertToastBody({ toast }) {
  const evt = toast.event
  if (!evt) return null
  const isEntry = evt.event_type === 'entry'
  const eventLabel = isEntry ? 'ENTRY' : 'EXIT'
  return (
    <div>
      <div className="mb-1 flex items-center gap-2">
        <span className={`badge ${isEntry ? badgeStyles.entry : badgeStyles.exit}`}>
          {eventLabel}
        </span>
        <span className="text-xs text-slate-400">
          {new Date(evt.timestamp).toLocaleString()}
        </span>
      </div>
      <p className="text-sm font-semibold text-slate-800">
        {evt.vehicle?.vehicle_number}
        {evt.vehicle?.driver_name ? ` — ${evt.vehicle.driver_name}` : ''}
      </p>
      <p className="text-sm text-slate-600">
        {evt.geofence?.geofence_name}
        {evt.geofence?.category ? (
          <span className="ml-1 text-xs text-slate-400">({evt.geofence.category})</span>
        ) : null}
      </p>
    </div>
  )
}

export default function ToastContainer() {
  const { toasts, dismiss } = useToast()

  return (
    <div
      className="pointer-events-none fixed bottom-4 right-4 z-[1100] flex w-[min(22rem,calc(100vw-2rem))] flex-col gap-2"
      aria-live="polite"
    >
      {toasts.map((t) => (
        <div
          key={t.id}
          className={`pointer-events-auto flex items-start gap-2 rounded-md border border-slate-200 border-l-4 bg-white px-3 py-2 shadow-lg ${
            typeStyles[t.type] || typeStyles.info
          }`}
          role="status"
        >
          <div className="min-w-0 flex-1">
            {t.title ? (
              <p className="mb-0.5 text-sm font-semibold text-slate-800">{t.title}</p>
            ) : null}
            {t.event ? (
              <AlertToastBody toast={t} />
            ) : (
              <p className="text-sm text-slate-700">{t.message}</p>
            )}
          </div>
          <button
            type="button"
            onClick={() => dismiss(t.id)}
            className="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
            aria-label="Dismiss notification"
          >
            <svg className="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
              <path
                fillRule="evenodd"
                d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                clipRule="evenodd"
              />
            </svg>
          </button>
        </div>
      ))}
    </div>
  )
}
