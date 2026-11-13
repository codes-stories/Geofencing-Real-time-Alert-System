import React, { useCallback, useEffect, useState } from 'react'
import { get } from '../api/client'
import { useToast } from './Toast'
import { formatDateTime } from '../constants'
import { EmptyState, ErrorRow, Spinner, TableSkeleton } from './ui'

const LIMIT_OPTIONS = [25, 50, 100, 250]

const defaultFilters = {
  vehicle_id: '',
  geofence_id: '',
  start_date: '',
  end_date: '',
  limit: 50,
}

function toRfc3339(value) {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  return d.toISOString()
}

function EventTypeBadge({ type }) {
  return (
    <span
      className={`badge ${
        type === 'entry' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'
      }`}
    >
      {type}
    </span>
  )
}

export default function ViolationsPanel() {
  const { toast } = useToast()

  const [vehicles, setVehicles] = useState([])
  const [geofences, setGeofences] = useState([])

  const [filters, setFilters] = useState(defaultFilters)
  const [appliedFilters, setAppliedFilters] = useState(defaultFilters)
  const [offset, setOffset] = useState(0)
  const [violations, setViolations] = useState([])
  const [totalCount, setTotalCount] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    let cancelled = false
    Promise.allSettled([get('/vehicles'), get('/geofences')]).then(([veh, geo]) => {
      if (cancelled) return
      if (veh.status === 'fulfilled') setVehicles(veh.value.vehicles || [])
      if (geo.status === 'fulfilled') setGeofences(geo.value.geofences || [])
    })
    return () => {
      cancelled = true
    }
  }, [])

  const load = useCallback(
    async (activeFilters, activeOffset) => {
      setLoading(true)
      try {
        const params = {
          limit: activeFilters.limit,
          offset: activeOffset,
        }
        if (activeFilters.vehicle_id) params.vehicle_id = activeFilters.vehicle_id
        if (activeFilters.geofence_id) params.geofence_id = activeFilters.geofence_id
        const start = toRfc3339(activeFilters.start_date)
        const end = toRfc3339(activeFilters.end_date)
        if (start) params.start_date = start
        if (end) params.end_date = end

        const data = await get('/violations/history', params)
        setViolations(data.violations || [])
        setTotalCount(data.total_count ?? (data.violations || []).length)
        setError(null)
      } catch (err) {
        setError(err.message)
        toast(err.message, { type: 'error' })
      } finally {
        setLoading(false)
      }
    },
    [toast],
  )

  useEffect(() => {
    load(defaultFilters, 0)
  }, [load])

  const update = (field, value) =>
    setFilters((prev) => ({ ...prev, [field]: value }))

  const handleApply = (event) => {
    event?.preventDefault?.()
    setOffset(0)
    setAppliedFilters(filters)
    load(filters, 0)
  }

  const handlePrev = () => {
    const nextOffset = Math.max(0, offset - appliedFilters.limit)
    setOffset(nextOffset)
    load(appliedFilters, nextOffset)
  }

  const handleNext = () => {
    const nextOffset = offset + appliedFilters.limit
    setOffset(nextOffset)
    load(appliedFilters, nextOffset)
  }

  const shownFrom = totalCount === 0 ? 0 : offset + 1
  const shownTo = Math.min(offset + violations.length, totalCount)
  const hasPrev = offset > 0
  const hasNext = offset + appliedFilters.limit < totalCount

  return (
    <div className="space-y-4">
      <form onSubmit={handleApply} className="card space-y-3">
        <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-600">
          Filter Violations
        </h2>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-5">
          <div>
            <label className="label-field" htmlFor="vi-vehicle">
              Vehicle
            </label>
            <select
              id="vi-vehicle"
              className="input-field"
              value={filters.vehicle_id}
              onChange={(e) => update('vehicle_id', e.target.value)}
            >
              <option value="">All vehicles</option>
              {vehicles.map((v) => (
                <option key={v.id} value={v.id}>
                  {v.vehicle_number}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="label-field" htmlFor="vi-geofence">
              Geofence
            </label>
            <select
              id="vi-geofence"
              className="input-field"
              value={filters.geofence_id}
              onChange={(e) => update('geofence_id', e.target.value)}
            >
              <option value="">All geofences</option>
              {geofences.map((g) => (
                <option key={g.id} value={g.id}>
                  {g.name}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="label-field" htmlFor="vi-start">
              Start date
            </label>
            <input
              id="vi-start"
              type="datetime-local"
              className="input-field"
              value={filters.start_date}
              onChange={(e) => update('start_date', e.target.value)}
            />
          </div>

          <div>
            <label className="label-field" htmlFor="vi-end">
              End date
            </label>
            <input
              id="vi-end"
              type="datetime-local"
              className="input-field"
              value={filters.end_date}
              onChange={(e) => update('end_date', e.target.value)}
            />
          </div>

          <div>
            <label className="label-field" htmlFor="vi-limit">
              Limit
            </label>
            <select
              id="vi-limit"
              className="input-field"
              value={filters.limit}
              onChange={(e) => update('limit', Number(e.target.value))}
            >
              {LIMIT_OPTIONS.map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="flex justify-end">
          <button type="submit" className="btn-primary" disabled={loading}>
            {loading && <Spinner className="h-4 w-4" />} Apply Filters
          </button>
        </div>
      </form>

      <div className="card overflow-hidden !p-0">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-slate-200 px-4 py-3">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-600">
            Violation History
          </h2>
          {!loading && !error && (
            <span className="text-xs text-slate-500">
              Showing {shownFrom}–{shownTo} of {totalCount}
            </span>
          )}
        </div>

        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-200">
            <thead className="bg-slate-50">
              <tr>
                <th className="table-th">Timestamp</th>
                <th className="table-th">Vehicle number</th>
                <th className="table-th">Geofence</th>
                <th className="table-th">Event type</th>
                <th className="table-th">Latitude</th>
                <th className="table-th">Longitude</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {loading ? (
                <TableSkeleton rows={5} cols={6} />
              ) : error ? (
                <ErrorRow message={error} />
              ) : violations.length === 0 ? (
                <EmptyState message="No violations found for the selected filters." />
              ) : (
                violations.map((v, i) => (
                  <tr key={v.id || `${v.timestamp}-${i}`} className="hover:bg-slate-50">
                    <td className="table-td">{formatDateTime(v.timestamp)}</td>
                    <td className="table-td font-medium text-slate-800">
                      {v.vehicle_number || v.vehicle_id}
                    </td>
                    <td className="table-td">{v.geofence_name || v.geofence_id}</td>
                    <td className="table-td">
                      <EventTypeBadge type={v.event_type} />
                    </td>
                    <td className="table-td font-mono text-xs">
                      {typeof v.latitude === 'number' ? v.latitude.toFixed(5) : v.latitude}
                    </td>
                    <td className="table-td font-mono text-xs">
                      {typeof v.longitude === 'number' ? v.longitude.toFixed(5) : v.longitude}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        <div className="flex items-center justify-between gap-2 border-t border-slate-200 px-4 py-3">
          <button
            type="button"
            className="btn-secondary"
            onClick={handlePrev}
            disabled={!hasPrev || loading}
          >
            Prev
          </button>
          <span className="text-xs text-slate-500">
            Showing {shownFrom}–{shownTo} of {totalCount}
          </span>
          <button
            type="button"
            className="btn-secondary"
            onClick={handleNext}
            disabled={!hasNext || loading}
          >
            Next
          </button>
        </div>
      </div>
    </div>
  )
}
