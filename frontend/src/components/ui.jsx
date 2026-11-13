import React from 'react'

export function Spinner({ className = 'h-4 w-4' }) {
  return (
    <svg
      className={`animate-spin text-current ${className}`}
      xmlns="http://www.w3.org/2000/svg"
      fill="none"
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
    </svg>
  )
}

export function ButtonSpinner() {
  return <Spinner className="h-4 w-4" />
}

export function TableSkeleton({ rows = 5, cols = 4 }) {
  return (
    <>
      {Array.from({ length: rows }).map((_, r) => (
        <tr key={r} className="animate-pulse border-b border-slate-100">
          {Array.from({ length: cols }).map((__, c) => (
            <td key={c} className="px-3 py-3">
              <div className="h-4 w-full rounded bg-slate-200" />
            </td>
          ))}
        </tr>
      ))}
    </>
  )
}

export function EmptyState({ message }) {
  return (
    <tr>
      <td colSpan={100} className="px-4 py-10 text-center text-sm text-slate-400">
        {message}
      </td>
    </tr>
  )
}

export function ErrorRow({ message }) {
  return (
    <tr>
      <td colSpan={100} className="px-4 py-10 text-center text-sm text-red-600">
        {message}
      </td>
    </tr>
  )
}
