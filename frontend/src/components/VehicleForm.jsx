import React, { useState } from 'react'
import { post } from '../api/client'
import { useToast } from './Toast'
import { VEHICLE_TYPE_OPTIONS } from '../constants'
import { ButtonSpinner } from './ui'

const initialForm = {
  vehicle_number: '',
  driver_name: '',
  vehicle_type: '',
  phone: '',
}

export default function VehicleForm({ onCreated }) {
  const { toast } = useToast()
  const [form, setForm] = useState(initialForm)
  const [submitting, setSubmitting] = useState(false)

  const update = (field) => (event) =>
    setForm((prev) => ({ ...prev, [field]: event.target.value }))

  async function handleSubmit(event) {
    event.preventDefault()
    if (!form.vehicle_number.trim() || !form.driver_name.trim()) {
      toast('Vehicle number and driver name are required.', { type: 'error' })
      return
    }
    setSubmitting(true)
    try {
      const data = await post('/vehicles', {
        vehicle_number: form.vehicle_number.trim(),
        driver_name: form.driver_name.trim(),
        vehicle_type: form.vehicle_type.trim() || 'car',
        phone: form.phone.trim(),
      })
      toast(`Vehicle ${data.vehicle_number || form.vehicle_number} registered.`, {
        type: 'success',
      })
      setForm(initialForm)
      onCreated?.()
    } catch (err) {
      toast(err.message, { type: 'error' })
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="card space-y-3">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-slate-600">
        Register Vehicle
      </h2>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div>
          <label className="label-field" htmlFor="v-number">
            Vehicle number *
          </label>
          <input
            id="v-number"
            className="input-field"
            placeholder="KA-01-AB-1234"
            value={form.vehicle_number}
            onChange={update('vehicle_number')}
            required
          />
        </div>

        <div>
          <label className="label-field" htmlFor="v-driver">
            Driver name *
          </label>
          <input
            id="v-driver"
            className="input-field"
            placeholder="John Doe"
            value={form.driver_name}
            onChange={update('driver_name')}
            required
          />
        </div>

        <div>
          <label className="label-field" htmlFor="v-type">
            Vehicle type
          </label>
          <input
            id="v-type"
            className="input-field"
            list="vehicle-type-options"
            placeholder="truck"
            value={form.vehicle_type}
            onChange={update('vehicle_type')}
          />
          <datalist id="vehicle-type-options">
            {VEHICLE_TYPE_OPTIONS.map((t) => (
              <option key={t} value={t} />
            ))}
          </datalist>
        </div>

        <div>
          <label className="label-field" htmlFor="v-phone">
            Phone
          </label>
          <input
            id="v-phone"
            className="input-field"
            placeholder="+1 555 0100"
            value={form.phone}
            onChange={update('phone')}
          />
        </div>
      </div>

      <div className="flex justify-end">
        <button type="submit" className="btn-primary" disabled={submitting}>
          {submitting && <ButtonSpinner />}
          {submitting ? 'Registering…' : 'Register Vehicle'}
        </button>
      </div>
    </form>
  )
}
