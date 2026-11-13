export const CATEGORY_COLORS = {
  delivery_zone: '#22c55e',
  restricted_zone: '#ef4444',
  toll_zone: '#f59e0b',
  customer_area: '#3b82f6',
}

export const CATEGORY_LABELS = {
  delivery_zone: 'Delivery Zone',
  restricted_zone: 'Restricted Zone',
  toll_zone: 'Toll Zone',
  customer_area: 'Customer Area',
}

export const CATEGORY_OPTIONS = [
  { value: 'delivery_zone', label: 'Delivery Zone' },
  { value: 'restricted_zone', label: 'Restricted Zone' },
  { value: 'toll_zone', label: 'Toll Zone' },
  { value: 'customer_area', label: 'Customer Area' },
]

export const VEHICLE_TYPE_OPTIONS = ['truck', 'car', 'van', 'motorcycle']

export function categoryColor(category) {
  return CATEGORY_COLORS[category] || '#64748b'
}

export function formatDateTime(iso) {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}
