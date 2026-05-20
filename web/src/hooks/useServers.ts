import { useState, useEffect } from 'react'
import { api } from '../lib/api'
import type { Server } from '../types'

let cache: Server[] | null = null
let inflight: Promise<Server[]> | null = null
const listeners = new Set<(servers: Server[]) => void>()

function load(): Promise<Server[]> {
  if (inflight) return inflight
  inflight = api.get<Server[]>('/api/servers')
    .then(data => {
      cache = data
      inflight = null
      listeners.forEach(l => l(data))
      return data
    })
    .catch(err => {
      inflight = null
      throw err
    })
  return inflight
}

export function useServers(): Server[] {
  const [servers, setServers] = useState<Server[]>(cache ?? [])
  useEffect(() => {
    if (cache) {
      setServers(cache)
      return
    }
    const listener = (data: Server[]) => setServers(data)
    listeners.add(listener)
    load().catch(() => { /* consumers see [] on failure */ })
    return () => { listeners.delete(listener) }
  }, [])
  return servers
}
