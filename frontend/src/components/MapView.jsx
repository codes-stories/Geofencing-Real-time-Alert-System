import React, { useCallback, useEffect, useRef, useState } from 'react'
import L from 'leaflet'
import { get, post } from '../api/client'
import { useToast } from './Toast'
import GeofencePanel from './GeofencePanel'
import GeofenceForm from './GeofenceForm'
import LocationUpdatePanel from './LocationUpdatePanel'
import { CATEGORY_LABELS, categoryColor, formatDateTime } from '../constants'
import { Spinner } from './ui'

const DEFAULT_CENTER = [37.7749, -122.4194]
const DEFAULT_ZOOM = 13
const POLL_MS = 15000

function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function geofencePopupHtml(gf) {
  return `
    <div class="text-sm" style="min-width:180px">
      <p style="font-weight:600;margin-bottom:4px">${escapeHtml(gf.name)}</p>
      <p style="color:#475569;font-size:12px;margin-bottom:6px">${escapeHtml(gf.description || 'No description')}</p>
      <p style="font-size:12px">Category: <strong>${escapeHtml(
        CATEGORY_LABELS[gf.category] || gf.category,
      )}</strong></p>
      <p style="font-size:12px">Created: ${escapeHtml(formatDateTime(gf.created_at))}</p>
    </div>
  `
}

function buildVehiclePopupContent(vehicle, locDetail, onSetLocationHere) {
  const root = document.createElement('div')
  root.style.minWidth = '200px'
  const currentGf = locDetail?.current_geofences || []
  const gfHtml =
    currentGf.length > 0
      ? currentGf.map((g) => `<li>${escapeHtml(g.geofence_name)}</li>`).join('')
      : '<li style="color:#94a3b8">None</li>'

  root.innerHTML = `
    <div class="text-sm">
      <p style="font-weight:600;margin-bottom:2px">${escapeHtml(vehicle.vehicle_number)}</p>
      <p style="color:#475569;font-size:12px">Driver: ${escapeHtml(vehicle.driver_name || '—')}</p>
      <p style="color:#475569;font-size:12px">Type: ${escapeHtml(vehicle.vehicle_type || '—')}</p>
      <p style="color:#475569;font-size:12px">Phone: ${escapeHtml(vehicle.phone || '—')}</p>
      <p style="color:#475569;font-size:12px;margin-top:6px;font-weight:600">Current geofences:</p>
      <ul style="list-style:disc;padding-left:16px;color:#475569;font-size:12px">${gfHtml}</ul>
    </div>
  `

  const button = document.createElement('button')
  button.type = 'button'
  button.textContent = 'Set location here'
  button.style.cssText =
    'margin-top:8px;width:100%;border-radius:6px;background:#2563eb;color:#fff;font-size:12px;font-weight:600;padding:6px 8px;cursor:pointer'
  button.addEventListener('click', () => onSetLocationHere(vehicle.id))
  root.appendChild(button)
  return root
}

export default function MapView({ centerVehicleId, clearCenterVehicle }) {
  const { toast } = useToast()

  const mapElRef = useRef(null)
  const mapRef = useRef(null)
  const geoLayerRef = useRef(null)
  const vehicleLayerRef = useRef(null)
  const drawLayerRef = useRef(null)
  const polygonRefs = useRef({})
  const vehiclesRef = useRef([])
  const [mapReady, setMapReady] = useState(0)

  const [geofences, setGeofences] = useState([])
  const [geofencesLoading, setGeofencesLoading] = useState(true)
  const [geofencesError, setGeofencesError] = useState(null)
  const [categoryFilter, setCategoryFilter] = useState('')

  const [vehicles, setVehicles] = useState([])
  const [vehiclesLoading, setVehiclesLoading] = useState(true)
  const [locations, setLocations] = useState({})
  const [locationsLoaded, setLocationsLoaded] = useState(false)

  const [drawMode, setDrawMode] = useState(false)
  const [drawPoints, setDrawPoints] = useState([])
  const [formOpen, setFormOpen] = useState(false)
  const [formCoords, setFormCoords] = useState([])

  const [locMode, setLocMode] = useState(false)
  const [locVehicleId, setLocVehicleId] = useState(null)
  const [locBusy, setLocBusy] = useState(false)

  vehiclesRef.current = vehicles

  const fetchGeofences = useCallback(
    async (category) => {
      setGeofencesLoading(true)
      try {
        const data = await get('/geofences', category ? { category } : undefined)
        setGeofences(data.geofences || [])
        setGeofencesError(null)
      } catch (err) {
        setGeofencesError(err.message)
        toast(err.message, { type: 'error' })
      } finally {
        setGeofencesLoading(false)
      }
    },
    [toast],
  )

  const fetchLocations = useCallback(async (vehicleList) => {
    const list = vehicleList ?? vehiclesRef.current
    if (!list || list.length === 0) {
      setLocations({})
      setLocationsLoaded(true)
      return
    }
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
    setLocationsLoaded(true)
  }, [])

  const fetchVehiclesAndLocations = useCallback(async () => {
    setVehiclesLoading(true)
    try {
      const data = await get('/vehicles')
      const list = data.vehicles || []
      setVehicles(list)
      await fetchLocations(list)
    } catch (err) {
      toast(err.message, { type: 'error' })
    } finally {
      setVehiclesLoading(false)
    }
  }, [fetchLocations, toast])

  useEffect(() => {
    fetchGeofences(categoryFilter)
  }, [categoryFilter, fetchGeofences])

  useEffect(() => {
    fetchVehiclesAndLocations()
  }, [fetchVehiclesAndLocations])

  // Light poll: refresh vehicle locations every 15s
  useEffect(() => {
    const interval = setInterval(() => {
      fetchLocations()
    }, POLL_MS)
    return () => clearInterval(interval)
  }, [fetchLocations])

  // Initialize Leaflet map
  useEffect(() => {
    if (!mapElRef.current || mapRef.current) return
    const map = L.map(mapElRef.current, {
      center: DEFAULT_CENTER,
      zoom: DEFAULT_ZOOM,
      zoomControl: true,
    })
    const tiles = L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    })
    tiles.addTo(map)
    geoLayerRef.current = L.layerGroup().addTo(map)
    vehicleLayerRef.current = L.layerGroup().addTo(map)
    drawLayerRef.current = L.layerGroup().addTo(map)
    mapRef.current = map
    setMapReady((c) => c + 1)

    return () => {
      map.remove()
      mapRef.current = null
      geoLayerRef.current = null
      vehicleLayerRef.current = null
      drawLayerRef.current = null
      polygonRefs.current = {}
    }
  }, [])

  // Render geofence polygons
  useEffect(() => {
    const layer = geoLayerRef.current
    if (!layer) return
    layer.clearLayers()
    polygonRefs.current = {}
    geofences.forEach((gf) => {
      if (!Array.isArray(gf.coordinates) || gf.coordinates.length < 3) return
      const color = categoryColor(gf.category)
      const polygon = L.polygon(gf.coordinates, {
        color,
        weight: 2,
        fillColor: color,
        fillOpacity: 0.15,
      })
      polygon.bindPopup(geofencePopupHtml(gf))
      polygon.addTo(layer)
      polygonRefs.current[gf.id] = polygon
    })
  }, [geofences, mapReady])

  // Render vehicle markers
  useEffect(() => {
    const layer = vehicleLayerRef.current
    if (!layer) return
    layer.clearLayers()
    vehicles.forEach((v) => {
      const detail = locations[v.id]
      const pos = detail?.current_location
      if (!pos || typeof pos.latitude !== 'number' || typeof pos.longitude !== 'number') return

      const icon = L.divIcon({
        className: '',
        html: `<div style="width:28px;height:28px;border-radius:9999px;background:#2563eb;border:2px solid #fff;box-shadow:0 1px 4px rgba(0,0,0,0.4);display:flex;align-items:center;justify-content:center;font-size:14px;line-height:1">&#128666;</div>`,
        iconSize: [28, 28],
        iconAnchor: [14, 14],
        popupAnchor: [0, -14],
      })

      const marker = L.marker([pos.latitude, pos.longitude], { icon })
      const content = buildVehiclePopupContent(v, detail, (vehicleId) => {
        mapRef.current?.closePopup()
        setDrawMode(false)
        setDrawPoints([])
        setLocVehicleId(vehicleId)
        setLocMode(true)
        toast('Click anywhere on the map to set this vehicle’s location.', { type: 'info' })
      })
      marker.bindPopup(content, { maxWidth: 280 })
      marker.addTo(layer)
    })
  }, [vehicles, locations, mapReady, toast])

  // Draw mode: click to add vertices
  useEffect(() => {
    const map = mapRef.current
    if (!map || !drawMode) return
    const container = map.getContainer()
    container.style.cursor = 'crosshair'
    const onClick = (e) => {
      setDrawPoints((prev) => [...prev, [e.latlng.lat, e.latlng.lng]])
    }
    map.on('click', onClick)
    return () => {
      map.off('click', onClick)
      container.style.cursor = ''
    }
  }, [drawMode])

  // Location mode: click to update vehicle location
  useEffect(() => {
    const map = mapRef.current
    if (!map || !locMode) return
    const container = map.getContainer()
    container.style.cursor = 'crosshair'
    const onClick = async (e) => {
      if (!locVehicleId) {
        toast('Choose a vehicle first.', { type: 'error' })
        return
      }
      if (locBusy) return
      const vehicleId = locVehicleId
      setLocBusy(true)
      try {
        const data = await post('/vehicles/location', {
          vehicle_id: vehicleId,
          latitude: e.latlng.lat,
          longitude: e.latlng.lng,
          timestamp: new Date().toISOString(),
        })
        const inside = data.current_geofences || []
        const detail = inside.length
          ? `Inside: ${inside.map((g) => g.geofence_name).join(', ')}`
          : 'not inside any geofence'
        toast(`Location updated — ${detail}`, { type: 'success' })
        await fetchLocations()
      } catch (err) {
        toast(err.message, { type: 'error' })
      } finally {
        setLocBusy(false)
      }
    }
    map.on('click', onClick)
    return () => {
      map.off('click', onClick)
      container.style.cursor = ''
    }
  }, [locMode, locVehicleId, locBusy, fetchLocations, toast])

  // Render draw preview / active drawing
  useEffect(() => {
    const layer = drawLayerRef.current
    if (!layer) return
    layer.clearLayers()

    const preview = drawMode
      ? drawPoints
      : formOpen
        ? formCoords
        : []
    if (preview.length === 0) return

    if (preview.length > 1) {
      L.polyline(preview, { color: '#3b82f6', weight: 2, dashArray: '6 6' }).addTo(layer)
    }
    if (drawMode && preview.length >= 3) {
      L.polyline([preview[preview.length - 1], preview[0]], {
        color: '#3b82f6',
        weight: 1,
        dashArray: '3 6',
        opacity: 0.7,
      }).addTo(layer)
    }
    if (drawMode) {
      preview.forEach((pt) => {
        L.circleMarker(pt, {
          radius: 5,
          color: '#ffffff',
          weight: 2,
          fillColor: '#3b82f6',
          fillOpacity: 1,
        }).addTo(layer)
      })
    } else if (preview.length >= 3) {
      L.polygon([...preview, preview[0]], {
        color: '#3b82f6',
        weight: 2,
        fillColor: '#3b82f6',
        fillOpacity: 0.15,
        dashArray: '6 6',
      }).addTo(layer)
    }
  }, [drawMode, drawPoints, formOpen, formCoords, mapReady])

  // Center on requested vehicle (from Vehicles tab)
  useEffect(() => {
    if (!centerVehicleId) return
    if (!locationsLoaded) return
    const map = mapRef.current
    const detail = locations[centerVehicleId]
    const pos = detail?.current_location
    if (map && pos) {
      map.flyTo([pos.latitude, pos.longitude], 16, { duration: 0.8 })
    } else {
      toast('No location available for this vehicle yet.', { type: 'info' })
    }
    clearCenterVehicle?.()
  }, [centerVehicleId, locations, locationsLoaded, clearCenterVehicle, toast])

  const handleFinishDraw = () => {
    if (drawPoints.length < 3) {
      toast('Add at least 3 points before finishing.', { type: 'error' })
      return
    }
    setFormCoords([...drawPoints, drawPoints[0]])
    setFormOpen(true)
    setDrawMode(false)
    setDrawPoints([])
    setLocMode(false)
  }

  const handleCancelDraw = () => {
    setDrawMode(false)
    setDrawPoints([])
  }

  const handleToggleDraw = () => {
    if (drawMode) {
      handleCancelDraw()
      return
    }
    setLocMode(false)
    setFormOpen(false)
    setDrawPoints([])
    setDrawMode(true)
    toast('Draw mode on — click the map to add polygon vertices.', { type: 'info' })
  }

  const handleSelectGeofence = (gf) => {
    const polygon = polygonRefs.current[gf.id]
    const map = mapRef.current
    if (polygon && map) {
      map.flyToBounds(polygon.getBounds(), { padding: [40, 40], maxZoom: 16, duration: 0.8 })
      polygon.openPopup()
    }
  }

  const handleFormCreated = () => {
    fetchGeofences(categoryFilter)
  }

  return (
    <div className="flex flex-col lg:h-full lg:flex-row">
      <GeofencePanel
        geofences={geofences}
        loading={geofencesLoading}
        error={geofencesError}
        categoryFilter={categoryFilter}
        onCategoryFilterChange={setCategoryFilter}
        onSelect={handleSelectGeofence}
        onDrawToggle={handleToggleDraw}
        drawMode={drawMode}
      />

      <div className="relative h-[60vh] min-h-[380px] flex-1 lg:h-auto">
        <div ref={mapElRef} className="absolute inset-0 z-0" />

        {geofencesLoading && geofences.length === 0 && (
          <div className="absolute left-1/2 top-3 z-[800] flex -translate-x-1/2 items-center gap-2 rounded-md bg-white px-3 py-2 text-sm text-slate-600 shadow">
            <Spinner /> Loading map data…
          </div>
        )}

        {drawMode && (
          <div className="absolute bottom-4 left-1/2 z-[850] flex -translate-x-1/2 items-center gap-2 rounded-lg bg-white p-2 shadow-lg ring-1 ring-slate-200">
            <span className="px-1 text-xs text-slate-500">{drawPoints.length} point(s)</span>
            <button type="button" className="btn-primary !py-1.5 text-xs" onClick={handleFinishDraw}>
              Finish
            </button>
            <button type="button" className="btn-secondary !py-1.5 text-xs" onClick={handleCancelDraw}>
              Cancel
            </button>
          </div>
        )}

        {locMode && !drawMode && (
          <LocationUpdatePanel
            vehicles={vehicles}
            loading={vehiclesLoading}
            selectedVehicleId={locVehicleId}
            onSelectVehicle={setLocVehicleId}
            onCancel={() => {
              setLocMode(false)
              setLocVehicleId(null)
            }}
          />
        )}

        {formOpen && (
          <GeofenceForm
            initialCoordinates={formCoords}
            onClose={() => {
              setFormOpen(false)
              setFormCoords([])
            }}
            onCreated={handleFormCreated}
          />
        )}
      </div>
    </div>
  )
}
