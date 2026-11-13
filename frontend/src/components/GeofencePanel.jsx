import React from 'react'
import { CATEGORY_LABELS, categoryColor, formatDateTime } from '../constants'
import { Spinner } from './ui'

export default function GeofencePanel({
  geofences,
  loading,
  error,
  categoryFilter,
  onCategoryFilterChange,
  onSelect,
  onDrawToggle,
  drawMode,
}) {
  return (
    <aside className="flex h-64 shrink-0 flex-col border-b border-slate-200 bg-white lg:h-auto lg:w-80 lg:border-b-0 lg:border-r">
      <div className="flex items-center justify-between gap-2 border-b border-slate-200 px-3 py-2">
        <h2 className="text-sm font-semibold text-slate-700">Geofences</h2>
        <button
          type="button"
          onClick={onDrawToggle}
          className={drawMode ? 'btn-danger !px-2 !py-1 text-xs' : 'btn-primary !px-2 !py-1 text-xs'}
        >
          {drawMode ? 'Stop drawing' : 'Draw geofence'}
        </button>
      </div>

      <div className="border-b border-slate-200 px-3 py-2">
        <label className="label-field" htmlFor="gf-filter">
          Category filter
        </label>
        <select
          id="gf-filter"
          className="input-field"
          value={categoryFilter}
          onChange={(e) => onCategoryFilterChange(e.target.value)}
        >
          <option value="">All categories</option>
          <option value="delivery_zone">Delivery Zone</option>
          <option value="restricted_zone">Restricted Zone</option>
          <option value="toll_zone">Toll Zone</option>
          <option value="customer_area">Customer Area</option>
        </select>
      </div>

      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center gap-2 py-8 text-sm text-slate-500">
            <Spinner /> Loading geofences…
          </div>
        ) : error ? (
          <p className="px-3 py-4 text-sm text-red-600">{error}</p>
        ) : geofences.length === 0 ? (
          <p className="px-3 py-8 text-center text-sm text-slate-400">
            No geofences yet. Draw one to get started.
          </p>
        ) : (
          <ul className="divide-y divide-slate-100">
            {geofences.map((gf) => (
              <li key={gf.id}>
                <button
                  type="button"
                  onClick={() => onSelect(gf)}
                  className="flex w-full items-start gap-2 px-3 py-2.5 text-left hover:bg-slate-50"
                >
                  <span
                    className="mt-1 h-3 w-3 shrink-0 rounded-full"
                    style={{ backgroundColor: categoryColor(gf.category) }}
                    aria-hidden="true"
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium text-slate-700">
                      {gf.name}
                    </span>
                    <span className="block text-xs text-slate-500">
                      {CATEGORY_LABELS[gf.category] || gf.category}
                      {gf.created_at ? ` · ${formatDateTime(gf.created_at)}` : ''}
                    </span>
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </aside>
  )
}
