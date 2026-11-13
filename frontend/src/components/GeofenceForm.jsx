import React, { useMemo, useState } from 'react'
import { post } from '../api/client'
import { useToast } from './Toast'
import { CATEGORY_OPTIONS } from '../constants'
import { ButtonSpinner } from './ui'

export function parseCoordinateText(text) {
  const points = []
  const errors = []
  const lines = text.split('\n')
  lines.forEach((line, i) => {
    const trimmed = line.trim()
    if (!trimmed) return
    const parts = trimmed.split(',').map((p) => p.trim())
    if (parts.length !== 2) {
      errors.push(`Line ${i + 1}: expected "lat,lng"`)
      return
    }
    const lat = Number(parts[0])
    const lng = Number(parts[1])
    if (!Number.isFinite(lat) || !Number.isFinite(lng) || Math.abs(lat) > 90 || Math.abs(lng) > 180) {
      errors.push(`Line ${i + 1}: invalid coordinates`)
      return
    }
    points.push([lat, lng])
  })
  return { points, errors }
}

function coordsToText(coordinates) {
  return (coordinates || []).map(([lat, lng]) => `${lat},${lng}`).join('\n')
}

function PolygonPreview({ points }) {
  const svg = useMemo(() => {
    if (!points || points.length < 2) return null
    const lats = points.map((p) => p[0])
    const lngs = points.map((p) => p[1])
    const minLat = Math.min(...lats)
    const maxLat = Math.max(...lats)
    const minLng = Math.min(...lngs)
    const maxLng = Math.max(...lngs)
    const spanLat = maxLat - minLat || 0.001
    const spanLng = maxLng - minLng || 0.001
    const pad = 8
    const w = 240
    const h = 140
    const project = ([lat, lng]) => {
      const x = pad + ((lng - minLng) / spanLng) * (w - pad * 2)
      const y = h - pad - ((lat - minLat) / spanLat) * (h - pad * 2)
      return `${x.toFixed(1)},${y.toFixed(1)}`
    }
    const closed = points.length >= 3 ? [...points, points[0]] : points
    return {
      polyline: closed.map(project).join(' '),
      w,
      h,
      dots: points.map(project),
    }
  }, [points])

  if (!svg) return null

  return (
    <div className="rounded-md border border-slate-200 bg-slate-50 p-2">
      <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-slate-500">Preview</p>
      <svg viewBox={`0 0 ${svg.w} ${svg.h}`} className="h-32 w-full">
        <polygon
          points={svg.polyline}
          fill="rgba(59,130,246,0.15)"
          stroke="#3b82f6"
          strokeWidth="2"
        />
        {svg.dots.map((d, i) => {
          const [x, y] = d.split(',')
          return <circle key={i} cx={x} cy={y} r="3" fill="#3b82f6" />
        })}
      </svg>
    </div>
  )
}

export default function GeofenceForm({ initialCoordinates, onClose, onCreated }) {
  const { toast } = useToast()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [category, setCategory] = useState('delivery_zone')
  const [coordsText, setCoordsText] = useState(() => coordsToText(initialCoordinates))
  const [submitting, setSubmitting] = useState(false)

  const { points, errors } = useMemo(() => parseCoordinateText(coordsText), [coordsText])

  async function handleSubmit(event) {
    event.preventDefault()
    if (!name.trim()) {
      toast('Please enter a geofence name.', { type: 'error' })
      return
    }
    if (points.length < 3) {
      toast('A geofence needs at least 3 coordinate points.', { type: 'error' })
      return
    }
    if (errors.length > 0) {
      toast(`Fix coordinate errors first: ${errors[0]}`, { type: 'error' })
      return
    }

    setSubmitting(true)
    try {
      // The API requires a closed ring: first vertex must equal the last.
      const first = points[0]
      const last = points[points.length - 1]
      const alreadyClosed = first[0] === last[0] && first[1] === last[1]
      const ring = alreadyClosed ? points : [...points, first]
      const body = { name: name.trim(), coordinates: ring, category }
      if (description.trim()) body.description = description.trim()
      await post('/geofences', body)
      toast(`Geofence "${name.trim()}" created.`, { type: 'success' })
      onCreated?.()
      onClose()
    } catch (err) {
      toast(err.message, { type: 'error' })
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="absolute inset-y-0 right-0 z-[900] flex w-full flex-col border-l border-slate-200 bg-white shadow-xl sm:w-96">
      <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
        <h2 className="text-sm font-semibold text-slate-800">Create Geofence</h2>
        <button
          type="button"
          onClick={onClose}
          className="rounded p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600"
          aria-label="Close geofence form"
        >
          <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
            <path
              fillRule="evenodd"
              d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
              clipRule="evenodd"
            />
          </svg>
        </button>
      </div>

      <form onSubmit={handleSubmit} className="flex-1 space-y-3 overflow-y-auto p-4">
        <div>
          <label className="label-field" htmlFor="gf-name">
            Name *
          </label>
          <input
            id="gf-name"
            className="input-field"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Downtown Delivery Zone"
            required
          />
        </div>

        <div>
          <label className="label-field" htmlFor="gf-desc">
            Description
          </label>
          <textarea
            id="gf-desc"
            className="input-field"
            rows={2}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Optional details about this zone"
          />
        </div>

        <div>
          <label className="label-field" htmlFor="gf-category">
            Category *
          </label>
          <select
            id="gf-category"
            className="input-field"
            value={category}
            onChange={(e) => setCategory(e.target.value)}
          >
            {CATEGORY_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label className="label-field" htmlFor="gf-coords">
            Coordinates * (one lat,lng per line)
          </label>
          <textarea
            id="gf-coords"
            className="input-field font-mono text-xs"
            rows={6}
            value={coordsText}
            onChange={(e) => setCoordsText(e.target.value)}
            placeholder={'37.78,-122.42\n37.78,-122.40\n37.77,-122.41'}
          />
          <p className="mt-1 text-xs text-slate-500">{points.length} valid point(s)</p>
          {errors.length > 0 && (
            <ul className="mt-1 list-inside list-disc text-xs text-red-600">
              {errors.slice(0, 3).map((e) => (
                <li key={e}>{e}</li>
              ))}
            </ul>
          )}
        </div>

        <PolygonPreview points={points} />

        <div className="flex gap-2 pt-1">
          <button type="submit" className="btn-primary flex-1" disabled={submitting}>
            {submitting && <ButtonSpinner />}
            {submitting ? 'Creating…' : 'Create Geofence'}
          </button>
          <button type="button" className="btn-secondary" onClick={onClose} disabled={submitting}>
            Cancel
          </button>
        </div>
      </form>
    </div>
  )
}
