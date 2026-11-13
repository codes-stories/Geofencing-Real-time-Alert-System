import React, { useEffect, useState } from 'react'
import { get, post } from '../api/client'
import { useToast } from './Toast'
import { ButtonSpinner, Spinner } from './ui'

const EVENT_TYPES = [
  { value: 'entry', label: 'Entry' },
  { value: 'exit', label: 'Exit' },
  { value: 'both', label: 'Both' },
]

export default function AlertConfigForm({ onConfigured }) {
  const { toast } = useToast()
  const [geofences, setGeofences] = useState([])
  const [vehicles, setVehicles] = useState([])
  const [optionsLoading, setOptionsLoading] = useState(true)
  const [optionsError, setOptionsError] = useState(null)

  const [geofenceId, setGeofenceId] = useState('')
  const [vehicleId, setVehicleId] = useState('')
  const [eventType, setEventType] = useState('entry')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      setOptionsLoading(true)
      try {
        const [gfData, vehData] = await Promise.all([get('/geofences'), get('/vehicles')])
        if (cancelled) return
        setGeofences(gfData.geofences || [])
        setVehicles(vehData.vehicles || [])
        setOptionsError(null)
      } catch (err) {
        if (!cancelled) {
          setOptionsError(err.message)
          toast(err.message, { type: 'error' })
        }
      } finally {
        if (!cancelled) setOptionsLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [toast])

  async function handleSubmit(event) {
    event.preventDefault()
    if (!geofenceId) {
      toast('Please select a geofence.', { type: 'error' })
      return
    }
    setSubmitting(true)
    try {
      const body = { geofence_id: geofenceId, event_type: eventType }
      if (vehicleId) body.vehicle_id = vehicleId
      await post('/alerts/configure', body)
      toast('Alert rule configured.', { type: 'success' })
      setGeofenceId('')
      setVehicleId('')
      setEventType('entry')
      onConfigured?.()
    } catch (err) {
      toast(err.message, { type: 'error' })
    } finally {
      setSubmitting(false)
    }
  }

  if (optionsLoading) {
    return (
      <div className="card flex items-center justify-center gap-2 text-sm text-slate-500">
        <Spinner /> Loading alert options…
      </div>
    )
  }

  if (optionsError && geofences.length === 0 && vehicles.length === 0) {
    return (
      <div className="card">
        <p className="text-sm text-red-600">{optionsError}</p>
      </div>
    )
  }

  return (
    <form onSubmit={handleSubmit} className="card space-y-3">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-600">
        Configure Alert Rule
      </h2>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div>
          <label className="label-field" htmlFor="ac-geofence">
            Geofence *
          </label>
          <select
            id="ac-geofence"
            className="input-field"
            value={geofenceId}
            onChange={(e) => setGeofenceId(e.target.value)}
            required
          >
            <option value="">Choose a geofence…</option>
            {geofences.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name}
              </option>
            ))}
          </select>
          {geofences.length === 0 && (
            <p className="mt-1 text-xs text-amber-600">No geofences available — create one in the Map tab.</p>
          )}
        </div>

        <div>
          <label className="label-field" htmlFor="ac-vehicle">
            Vehicle
          </label>
          <select
            id="ac-vehicle"
            className="input-field"
            value={vehicleId}
            onChange={(e) => setVehicleId(e.target.value)}
          >
            <option value="">All vehicles</option>
            {vehicles.map((v) => (
              <option key={v.id} value={v.id}>
                {v.vehicle_number} — {v.driver_name}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div>
        <span className="label-field">Event type *</span>
        <div className="inline-flex w-full overflow-hidden rounded-md border border-slate-300 sm:w-auto">
          {EVENT_TYPES.map((opt, i) => (
            <label
              key={opt.value}
              className={`flex-1 cursor-pointer px-4 py-2 text-center text-sm font-medium transition-colors sm:flex-none ${
                i > 0 ? 'border-l border-slate-300' : ''
              } ${
                eventType === opt.value
                  ? 'bg-blue-600 text-white'
                  : 'bg-white text-slate-600 hover:bg-slate-50'
              }`}
            >
              <input
                type="radio"
                name="event_type"
                className="sr-only"
                value={opt.value}
                checked={eventType === opt.value}
                onChange={() => setEventType(opt.value)}
              />
              {opt.label}
            </label>
          ))}
        </div>
      </div>

      <div className="flex justify-end">
        <button type="submit" className="btn-primary" disabled={submitting}>
          {submitting && <ButtonSpinner />}
          {submitting ? 'Saving…' : 'Configure Alert'}
        </button>
      </div>
    </form>
  )
}
