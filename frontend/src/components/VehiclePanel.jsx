import React, { useCallback, useEffect, useState } from 'react'
import { get } from '../api/client'
import { useToast } from './Toast'
import VehicleForm from './VehicleForm'
import { formatDateTime } from '../constants'
import { EmptyState, ErrorRow, Spinner, TableSkeleton } from './ui'

function StatusBadge({ status }) {
  const active = status === 'active' || status === 'online'
  return (
    <span
      className={`badge ${
        active ? 'bg-green-100 text-green-700' : 'bg-slate-100 text-slate-600'
      }`}
    >
      {status || 'unknown'}
    </span>
  )
}

export default function VehiclePanel({ onCenterOnMap }) {
  const { toast } = useToast()
  const [vehicles, setVehicles] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [locations, setLocations] = useState({})
  const [locationsLoading, setLocationsLoading] = useState(false)

  const fetchLocations = useCallback(async (list) => {
    if (!list || list.length === 0) {
      setLocations({})
      return
    }
    setLocationsLoading(true)
    try {
      const results = await Promise.all(
        list.map(async (v) => {
          try {
            const data = await get(`/vehicles/location/${v.id}`)
            return [v.id, data]
          } catch {
            return [v.id, null]
          }
        }),
      )
      const next = {}
      results.forEach(([id, data]) => {
        if (data) next[id] = data
      })
      setLocations(next)
    } finally {
      setLocationsLoading(false)
    }
  }, [])

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const data = await get('/vehicles')
      const list = data.vehicles || []
      setVehicles(list)
      setError(null)
      await fetchLocations(list)
    } catch (err) {
      setError(err.message)
      toast(err.message, { type: 'error' })
    } finally {
      setLoading(false)
    }
  }, [fetchLocations, toast])

  useEffect(() => {
    refresh()
  }, [refresh])

  return (
    <div className="space-y-4">
      <VehicleForm onCreated={refresh} />

      <div className="card overflow-hidden !p-0">
        <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-600">
            Vehicles
          </h2>
          <button
            type="button"
            className="btn-secondary !py-1 text-xs"
            onClick={refresh}
            disabled={loading}
          >
            {loading && <Spinner className="h-3.5 w-3.5" />} Refresh
          </button>
        </div>

        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-200">
            <thead className="bg-slate-50">
              <tr>
                <th className="table-th">Vehicle number</th>
                <th className="table-th">Driver</th>
                <th className="table-th">Type</th>
                <th className="table-th">Phone</th>
                <th className="table-th">Status</th>
                <th className="table-th">Created</th>
                <th className="table-th">Current geofences</th>
                <th className="table-th">Map</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {loading ? (
                <TableSkeleton rows={4} cols={8} />
              ) : error ? (
                <ErrorRow message={error} />
              ) : vehicles.length === 0 ? (
                <EmptyState message="No vehicles registered yet." />
              ) : (
                vehicles.map((v) => {
                  const detail = locations[v.id]
                  const geofences = detail?.current_geofences || []
                  return (
                    <tr key={v.id} className="hover:bg-slate-50">
                      <td className="table-td font-medium text-slate-800">{v.vehicle_number}</td>
                      <td className="table-td">{v.driver_name || '—'}</td>
                      <td className="table-td">{v.vehicle_type || '—'}</td>
                      <td className="table-td">{v.phone || '—'}</td>
                      <td className="table-td">
                        <StatusBadge status={v.status} />
                      </td>
                      <td className="table-td">{formatDateTime(v.created_at)}</td>
                      <td className="table-td">
                        {locationsLoading && !detail ? (
                          <Spinner className="h-3.5 w-3.5 text-slate-400" />
                        ) : geofences.length > 0 ? (
                          <span className="flex flex-wrap gap-1">
                            {geofences.map((g) => (
                              <span key={g.geofence_id} className="badge bg-blue-50 text-blue-700">
                                {g.geofence_name}
                              </span>
                            ))}
                          </span>
                        ) : (
                          '—'
                        )}
                      </td>
                      <td className="table-td">
                        <button
                          type="button"
                          className="btn-secondary !px-2 !py-1 text-xs"
                          onClick={() => onCenterOnMap?.(v.id)}
                        >
                          Center on map
                        </button>
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
