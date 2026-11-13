import React from 'react'
import { Spinner } from './ui'

export default function LocationUpdatePanel({
  vehicles,
  loading,
  selectedVehicleId,
  onSelectVehicle,
  onCancel,
}) {
  return (
    <div className="absolute left-3 top-3 z-[850] w-[min(20rem,calc(100%-1.5rem))] rounded-lg border border-amber-300 bg-amber-50 p-3 shadow-lg">
      <div className="mb-2 flex items-start justify-between gap-2">
        <div>
          <p className="text-sm font-semibold text-amber-900">Set vehicle location</p>
          <p className="text-xs text-amber-700">Pick a vehicle, then click a point on the map.</p>
        </div>
        <button
          type="button"
          onClick={onCancel}
          className="rounded p-1 text-amber-600 hover:bg-amber-100"
          aria-label="Cancel location update mode"
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
      <label className="label-field" htmlFor="loc-vehicle">
        Vehicle
      </label>
      <select
        id="loc-vehicle"
        className="input-field"
        value={selectedVehicleId || ''}
        onChange={(e) => onSelectVehicle(e.target.value || null)}
      >
        <option value="">Choose a vehicle…</option>
        {vehicles.map((v) => (
          <option key={v.id} value={v.id}>
            {v.vehicle_number} — {v.driver_name}
          </option>
        ))}
      </select>
      {loading && vehicles.length === 0 && (
        <p className="mt-2 flex items-center gap-1 text-xs text-amber-700">
          <Spinner className="h-3 w-3" /> Loading vehicles…
        </p>
      )}
    </div>
  )
}
