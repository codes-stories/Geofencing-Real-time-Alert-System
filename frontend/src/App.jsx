import React, { useCallback, useState } from 'react'
import { ToastProvider } from './components/Toast'
import ToastContainer from './components/ToastContainer'
import { AlertsProvider } from './context/AlertsContext'
import Header from './components/Header'
import Tabs from './components/Tabs'
import MapView from './components/MapView'
import VehiclePanel from './components/VehiclePanel'
import AlertsPanel from './components/AlertsPanel'
import ViolationsPanel from './components/ViolationsPanel'

function Shell() {
  const [tab, setTab] = useState('map')
  const [centerVehicleId, setCenterVehicleId] = useState(null)

  const handleCenterOnMap = useCallback((vehicleId) => {
    setCenterVehicleId(vehicleId)
    setTab('map')
  }, [])

  const clearCenterVehicle = useCallback(() => setCenterVehicleId(null), [])

  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <Tabs active={tab} onChange={setTab} />

      <main className="mx-auto w-full max-w-7xl flex-1 p-3 sm:p-4">
        {tab === 'map' && (
          <div className="overflow-hidden rounded-lg border border-slate-200 bg-white shadow-sm lg:h-[calc(100vh-9rem)]">
            <MapView centerVehicleId={centerVehicleId} clearCenterVehicle={clearCenterVehicle} />
          </div>
        )}
        {tab === 'vehicles' && <VehiclePanel onCenterOnMap={handleCenterOnMap} />}
        {tab === 'alerts' && <AlertsPanel />}
        {tab === 'violations' && <ViolationsPanel />}
      </main>

      <ToastContainer />
    </div>
  )
}

export default function App() {
  return (
    <ToastProvider>
      <AlertsProvider>
        <Shell />
      </AlertsProvider>
    </ToastProvider>
  )
}
