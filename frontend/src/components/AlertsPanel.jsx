import React, { useCallback, useEffect, useState } from 'react'
import { get } from '../api/client'
import { useToast } from './Toast'
import AlertConfigForm from './AlertConfigForm'
import { formatDateTime } from '../constants'
import { EmptyState, ErrorRow, Spinner, TableSkeleton } from './ui'

function EventTypeBadge({ type }) {
  const styles = {
    entry: 'bg-green-100 text-green-700',
    exit: 'bg-red-100 text-red-700',
    both: 'bg-blue-100 text-blue-700',
  }
  return <span className={`badge ${styles[type] || 'bg-slate-100 text-slate-600'}`}>{type}</span>
}

function AlertStatusBadge({ status }) {
  const active = status === 'active' || status === 'enabled'
  return (
    <span
      className={`badge ${active ? 'bg-green-100 text-green-700' : 'bg-slate-100 text-slate-600'}`}
    >
      {status || 'unknown'}
    </span>
  )
}

export default function AlertsPanel() {
  const { toast } = useToast()
  const [alerts, setAlerts] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const data = await get('/alerts')
      setAlerts(data.alerts || [])
      setError(null)
    } catch (err) {
      setError(err.message)
      toast(err.message, { type: 'error' })
    } finally {
      setLoading(false)
    }
  }, [toast])

  useEffect(() => {
    refresh()
  }, [refresh])

  return (
    <div className="space-y-4">
      <AlertConfigForm onConfigured={refresh} />

      <div className="card overflow-hidden !p-0">
        <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-600">
            Alert Rules
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
                <th className="table-th">Alert ID</th>
                <th className="table-th">Geofence</th>
                <th className="table-th">Vehicle</th>
                <th className="table-th">Event type</th>
                <th className="table-th">Status</th>
                <th className="table-th">Created</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {loading ? (
                <TableSkeleton rows={4} cols={6} />
              ) : error ? (
                <ErrorRow message={error} />
              ) : alerts.length === 0 ? (
                <EmptyState message="No alert rules configured yet." />
              ) : (
                alerts.map((a) => (
                  <tr key={a.alert_id} className="hover:bg-slate-50">
                    <td className="table-td font-mono text-xs text-slate-500">{a.alert_id}</td>
                    <td className="table-td font-medium text-slate-800">
                      {a.geofence_name || a.geofence_id}
                    </td>
                    <td className="table-td">
                      {a.vehicle_id ? (
                        a.vehicle_number || a.vehicle_id
                      ) : (
                        <span className="text-slate-500">All vehicles</span>
                      )}
                    </td>
                    <td className="table-td">
                      <EventTypeBadge type={a.event_type} />
                    </td>
                    <td className="table-td">
                      <AlertStatusBadge status={a.status} />
                    </td>
                    <td className="table-td">{formatDateTime(a.created_at)}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
