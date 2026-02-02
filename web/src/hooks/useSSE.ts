import { useState, useEffect, useRef } from 'react'
import type { ActiveStream } from '../types'

interface SSEState {
  sessions: ActiveStream[]
  connected: boolean
}

export function useSSE(url: string): SSEState {
  const [sessions, setSessions] = useState<ActiveStream[]>([])
  const [connected, setConnected] = useState(false)
  const retryTimeout = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => {
    let es: EventSource
    let cancelled = false

    function connect() {
      if (cancelled) return
      es = new EventSource(url)

      es.onopen = () => {
        if (!cancelled) setConnected(true)
      }

      es.onmessage = (event: MessageEvent) => {
        if (cancelled) return
        const data = JSON.parse(event.data as string) as ActiveStream[]
        setSessions(data)
      }

      es.onerror = () => {
        if (cancelled) return
        setConnected(false)
        es.close()
        retryTimeout.current = setTimeout(connect, 3000)
      }
    }

    connect()

    return () => {
      cancelled = true
      es?.close()
      if (retryTimeout.current) clearTimeout(retryTimeout.current)
    }
  }, [url])

  return { sessions, connected }
}
