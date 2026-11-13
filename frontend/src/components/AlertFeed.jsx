import React from 'react'

function EventBadge({ type }) {
  const isEntry = type === 'entry'
  return (
    <span className={`badge ${isEntry ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}>
      {isEntry ? 'ENTRY' : 'EXIT'}
    </span>
  )
}

export default function AlertFeed({ alerts, onClose }) {
  return (
    <div
      className="absolute right-0 top-full z-[1001] mt-2 w-[min(22rem,calc(100vw-1.5rem))] overflow-hidden rounded-lg border border-slate-200 bg-white shadow-xl"
      role="dialog"
      aria-label="Recent live alerts"
    >
      <div className="flex items-center justify-between border-b border-slate-200 bg-slate-50 px-3 py-2">
        <p className="text-sm font-semibold text-slate-700">Live Alerts</p>
        <button
          type="button"
          onClick={onClose}
          className="rounded p-1 text-slate-400 hover:bg-slate-200 hover:text-slate-600"
          aria-label="Close alert feed"
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

      <div className="max-h-80 overflow-y-auto">
        {alerts.length === 0 ? (
          <p className="px-4 py-8 text-center text-sm text-slate-400">No alerts yet</p>
        ) : (
          <ul className="divide-y divide-slate-100">
            {alerts.map((alert, index) => (
              <li
                key={alert.event_id || `${alert.timestamp}-${index}`}
                className={`border-l-4 px-3 py-2 ${
                  alert.event_type === 'exit' ? 'border-l-red-500' : 'border-l-green-500'
                }`}
              >
                <div className="mb-1 flex items-center justify-between gap-2">
                  <EventBadge type={alert.event_type} />
                  <span className="text-[11px] text-slate-400">
                    {new Date(alert.timestamp).toLocaleString()}
                  </span>
                </div>
                <p className="truncate text-sm font-medium text-slate-800">
                  {alert.vehicle?.vehicle_number}
                  {alert.vehicle?.driver_name ? ` — ${alert.vehicle.driver_name}` : ''}
                </p>
                <p className="truncate text-xs text-slate-500">
                  {alert.geofence?.geofence_name}
                  {alert.geofence?.category ? ` · ${alert.geofence.category}` : ''}
                </p>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
