import React from 'react'

export const TABS = [
  { id: 'map', label: 'Map' },
  { id: 'vehicles', label: 'Vehicles' },
  { id: 'alerts', label: 'Alerts' },
  { id: 'violations', label: 'Violations' },
]

export default function Tabs({ active, onChange }) {
  return (
    <nav className="border-b border-slate-200 bg-white" aria-label="Main navigation">
      <div className="mx-auto flex max-w-7xl gap-1 overflow-x-auto px-2 sm:px-4">
        {TABS.map((tab) => {
          const isActive = active === tab.id
          return (
            <button
              key={tab.id}
              type="button"
              onClick={() => onChange(tab.id)}
              className={`whitespace-nowrap border-b-2 px-3 py-3 text-sm font-medium transition-colors sm:px-4 ${
                isActive
                  ? 'border-blue-600 text-blue-600'
                  : 'border-transparent text-slate-500 hover:border-slate-300 hover:text-slate-700'
              }`}
              aria-current={isActive ? 'page' : undefined}
            >
              {tab.label}
            </button>
          )
        })}
      </div>
    </nav>
  )
}
