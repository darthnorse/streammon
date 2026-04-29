import { createContext, useContext, ReactNode } from 'react'
import { useSSE } from '../hooks/useSSE'
import type { ActiveStream } from '../types'

interface SessionsState {
  sessions: ActiveStream[]
  connected: boolean
}

const EMPTY: SessionsState = { sessions: [], connected: false }
const SessionsContext = createContext<SessionsState>(EMPTY)

export function useSessions(): SessionsState {
  return useContext(SessionsContext)
}

interface SessionsProviderProps {
  enabled: boolean
  children: ReactNode
}

export function SessionsProvider({ enabled, children }: SessionsProviderProps) {
  if (!enabled) {
    return <SessionsContext.Provider value={EMPTY}>{children}</SessionsContext.Provider>
  }
  return <ActiveSessionsProvider>{children}</ActiveSessionsProvider>
}

function ActiveSessionsProvider({ children }: { children: ReactNode }) {
  const state = useSSE('/api/dashboard/sse')
  return <SessionsContext.Provider value={state}>{children}</SessionsContext.Provider>
}
