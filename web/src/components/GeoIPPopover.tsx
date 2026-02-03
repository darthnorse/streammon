import { useState, useRef, useEffect } from 'react'
import type { GeoResult } from '../types'
import { api } from '../lib/api'

interface GeoIPPopoverProps {
  ip: string
  children: React.ReactNode
}

function isPrivateIP(ip: string): boolean {
  return /^(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.|169\.254\.|127\.|0\.|::1$|::ffff:(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.|127\.)|^fe80:|^fc|^fd)/.test(ip)
}

function PopoverContent({ ip, geo, loading, error }: {
  ip: string
  geo: GeoResult | null
  loading: boolean
  error: string
}) {
  if (isPrivateIP(ip)) {
    return <div className="text-muted dark:text-muted-dark">Local Network</div>
  }
  if (loading) {
    return <div className="text-muted dark:text-muted-dark">Loading...</div>
  }
  if (error) {
    return <div className="text-muted dark:text-muted-dark">{error}</div>
  }
  if (geo) {
    return (
      <div className="space-y-1">
        <div className="font-semibold">
          {[geo.city, geo.country].filter(Boolean).join(', ') || 'Unknown location'}
        </div>
        {geo.isp && (
          <div className="text-muted dark:text-muted-dark">{geo.isp}</div>
        )}
        <div className="text-muted dark:text-muted-dark font-mono text-[10px]">
          {geo.lat.toFixed(4)}, {geo.lng.toFixed(4)}
        </div>
      </div>
    )
  }
  return null
}

export function GeoIPPopover({ ip, children }: GeoIPPopoverProps) {
  const [open, setOpen] = useState(false)
  const [geo, setGeo] = useState<GeoResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [open])

  function handleToggle() {
    if (open) {
      setOpen(false)
      return
    }
    setOpen(true)
    if (isPrivateIP(ip) || geo) return
    setLoading(true)
    setError('')
    api.get<GeoResult>(`/api/geoip/${encodeURIComponent(ip)}`)
      .then(data => { setGeo(data); setLoading(false) })
      .catch(() => { setError('No geo data'); setLoading(false) })
  }

  return (
    <div ref={ref} className="relative inline-block">
      <button type="button" onClick={handleToggle} className="cursor-pointer">
        {children}
      </button>
      {open && (
        <div className="absolute z-50 top-full mt-1 right-0 w-48 card p-3 text-xs shadow-lg">
          <PopoverContent ip={ip} geo={geo} loading={loading} error={error} />
        </div>
      )}
    </div>
  )
}
