import { createContext, useContext, ReactNode } from 'react'
import { useSSE } from '../hooks/useSSE'
import type { ActiveStream } from '../types'

interface SessionsState {
  sessions: ActiveStream[]
  connected: boolean
}

// connected=true in the inert state because "not subscribed" is not a failure
// any consumer should render as "Reconnecting"; only the active path can flip
// connected to false via an SSE error.
const INERT: SessionsState = { sessions: [], connected: true }
const SessionsContext = createContext<SessionsState>(INERT)

export function useSessions(): SessionsState {
  return useContext(SessionsContext)
}

interface SessionsProviderProps {
  enabled: boolean
  children: ReactNode
}

// Two distinct component bodies so useSSE is only called when enabled —
// collapsing the branches would violate React's hooks rules.
export function SessionsProvider({ enabled, children }: SessionsProviderProps) {
  if (!enabled) {
    return <SessionsContext.Provider value={INERT}>{children}</SessionsContext.Provider>
  }
  return <ActiveSessionsProvider>{children}</ActiveSessionsProvider>
}

function ActiveSessionsProvider({ children }: { children: ReactNode }) {
  const state = useSSE('/api/dashboard/sse')
  return <SessionsContext.Provider value={state}>{children}</SessionsContext.Provider>
}
