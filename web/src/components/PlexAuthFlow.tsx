import { useState, useRef, useCallback, useEffect } from 'react'
import { getClientId, requestPin, checkPin, getAuthUrl } from '../lib/plexOAuth'
import { api } from '../lib/api'
import { plexBtnClass } from '../lib/constants'
import type { User } from '../types'

interface PlexAuthFlowProps {
  onSuccess: (user: User) => void
  endpoint: string
  buttonClassName?: string
  loadingMessage?: string
  centered?: boolean
}

type Phase = 'idle' | 'polling' | 'submitting'

function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message
  return String(err)
}

export function PlexAuthFlow({
  onSuccess,
  endpoint,
  buttonClassName = plexBtnClass,
  loadingMessage = 'Signing in...',
  centered = false,
}: PlexAuthFlowProps) {
  const [phase, setPhase] = useState<Phase>('idle')
  const [error, setError] = useState('')
  const abortRef = useRef<AbortController | null>(null)
  const popupRef = useRef<Window | null>(null)

  const cleanup = useCallback(() => {
    if (abortRef.current) {
      abortRef.current.abort()
      abortRef.current = null
    }
    if (popupRef.current && !popupRef.current.closed) {
      popupRef.current.close()
    }
    popupRef.current = null
  }, [])

  useEffect(() => cleanup, [cleanup])

  async function startAuth() {
    setError('')
    const popup = window.open('about:blank', 'plexAuth', 'width=800,height=700')
    if (!popup) {
      setError('Popup blocked. Please allow popups for this site.')
      return
    }
    popupRef.current = popup
    setPhase('polling')
    try {
      const pin = await requestPin()
      const clientId = getClientId()
      const authUrl = getAuthUrl(clientId, pin.code)
      if (!popup.closed) {
        popup.location.href = authUrl
      }

      abortRef.current = new AbortController()
      pollForToken(pin.id, abortRef.current.signal)
    } catch (err) {
      if (popup && !popup.closed) popup.close()
      setError('Failed to start Plex sign-in: ' + errorMessage(err))
      setPhase('idle')
    }
  }

  async function pollForToken(pinId: number, signal: AbortSignal) {
    const timeout = Date.now() + 5 * 60 * 1000
    while (!signal.aborted && Date.now() < timeout) {
      await new Promise(r => setTimeout(r, 1500))
      if (signal.aborted) return

      if (popupRef.current?.closed) {
        abortRef.current?.abort()
        setError('Sign-in window was closed. Please try again.')
        setPhase('idle')
        return
      }

      try {
        const result = await checkPin(pinId)
        if (result.authToken) {
          if (signal.aborted) return
          if (popupRef.current && !popupRef.current.closed) {
            popupRef.current.close()
          }
          await submitAuth(result.authToken)
          return
        }
      } catch { /* retry on next poll */ }
    }

    if (!signal.aborted) {
      setError('Sign-in timed out. Please try again.')
      setPhase('idle')
    }
  }

  async function submitAuth(authToken: string) {
    setPhase('submitting')
    try {
      const user = await api.post<User>(endpoint, { auth_token: authToken })
      if (abortRef.current?.signal.aborted) return
      onSuccess(user)
    } catch (err) {
      if (abortRef.current?.signal.aborted) return
      setError(errorMessage(err))
      setPhase('idle')
    }
  }

  function cancel() {
    cleanup()
    setPhase('idle')
    setError('')
  }

  const containerClass = centered ? 'text-center' : ''

  if (phase === 'idle') {
    return (
      <div className={containerClass}>
        <button onClick={startAuth} className={buttonClassName}>
          Sign in with Plex
        </button>
        {error && (
          <p className="text-sm text-red-500 dark:text-red-400 mt-2">{error}</p>
        )}
      </div>
    )
  }

  if (phase === 'polling') {
    return (
      <div className={containerClass}>
        <p className="text-sm">Waiting for Plex authorization...</p>
        <p className="text-xs text-muted dark:text-muted-dark mt-1">
          Complete sign-in in the popup window.
        </p>
        <button onClick={cancel} className="text-sm text-accent hover:underline mt-3">
          Cancel
        </button>
        {error && (
          <p className="text-sm text-red-500 dark:text-red-400 mt-2">{error}</p>
        )}
      </div>
    )
  }

  return (
    <div className={'text-sm text-muted dark:text-muted-dark ' + containerClass}>
      {loadingMessage}
    </div>
  )
}
