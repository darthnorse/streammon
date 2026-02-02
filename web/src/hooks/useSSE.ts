import { useState, useEffect, useRef } from 'react'
import type { ActiveStream } from '../types'

interface SSEState {
  sessions: ActiveStream[]
  connected: boolean
}

const INITIAL_RETRY_MS = 1000
const MAX_RETRY_MS = 30000

export function useSSE(url: string): SSEState {
  const [sessions, setSessions] = useState<ActiveStream[]>([])
  const [connected, setConnected] = useState(false)
  const retryTimeout = useRef<ReturnType<typeof setTimeout>>()
  const retryDelay = useRef(INITIAL_RETRY_MS)

  useEffect(() => {
    let es: EventSource
    let cancelled = false

    function connect() {
      if (cancelled) return
      es = new EventSource(url)

      es.onopen = () => {
        if (!cancelled) {
          setConnected(true)
          retryDelay.current = INITIAL_RETRY_MS
        }
      }

      es.onmessage = (event: MessageEvent) => {
        if (cancelled) return
        try {
          const data = JSON.parse(event.data as string) as ActiveStream[]
          setSessions(data)
        } catch {
        }
      }

      es.onerror = () => {
        if (cancelled) return
        setConnected(false)
        es.close()
        retryTimeout.current = setTimeout(connect, retryDelay.current)
        retryDelay.current = Math.min(retryDelay.current * 2, MAX_RETRY_MS)
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
