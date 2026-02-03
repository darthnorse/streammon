import { useState, useEffect, useRef, useCallback } from 'react'
import type { ActiveStream } from '../types'

interface SSEState {
  sessions: ActiveStream[]
  connected: boolean
}

const INITIAL_RETRY_MS = 1000
const MAX_RETRY_MS = 30000
const INTERPOLATION_INTERVAL_MS = 1000

function sessionKey(s: ActiveStream): string {
  return `${s.server_id}:${s.session_id}`
}

export function useSSE(url: string): SSEState {
  const [sessions, setSessions] = useState<ActiveStream[]>([])
  const [connected, setConnected] = useState(false)
  const retryTimeout = useRef<ReturnType<typeof setTimeout>>()
  const retryDelay = useRef(INITIAL_RETRY_MS)

  const mergeSessionData = useCallback((incoming: ActiveStream[]) => {
    setSessions(prev => {
      const prevMap = new Map(prev.map(s => [sessionKey(s), s]))

      return incoming.map(newSession => {
        const key = sessionKey(newSession)
        const existing = prevMap.get(key)

        if (!existing) {
          return newSession
        }

        // Use whichever progress is further ahead
        // Server value wins if it jumped forward (seek, new data)
        // Our interpolated value wins if server is behind (stale update)
        const serverProgress = newSession.progress_ms ?? 0
        const localProgress = existing.progress_ms ?? 0
        const useServerProgress = serverProgress >= localProgress

        return {
          ...newSession,
          progress_ms: useServerProgress ? serverProgress : localProgress,
        }
      })
    })
  }, [])

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
          mergeSessionData(data)
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
  }, [url, mergeSessionData])

  useEffect(() => {
    const interval = setInterval(() => {
      setSessions(prev => {
        if (prev.length === 0) return prev

        return prev.map(session => {
          const duration = session.duration_ms ?? 0
          const progress = session.progress_ms ?? 0

          // For live TV (duration=0), always interpolate
          // For regular media, interpolate until we reach the end
          if (duration === 0 || progress < duration) {
            const newProgress = progress + INTERPOLATION_INTERVAL_MS
            return {
              ...session,
              progress_ms: duration > 0 ? Math.min(newProgress, duration) : newProgress,
            }
          }
          return session
        })
      })
    }, INTERPOLATION_INTERVAL_MS)

    return () => clearInterval(interval)
  }, [])

  return { sessions, connected }
}
