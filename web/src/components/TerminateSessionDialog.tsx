import { useState } from 'react'
import type { ActiveStream } from '../types'
import { api } from '../lib/api'
import { useModal } from '../hooks/useModal'
import { formInputClass } from '../lib/constants'

interface TerminateSessionDialogProps {
  stream: ActiveStream
  onClose: () => void
}

export function TerminateSessionDialog({ stream, onClose }: TerminateSessionDialogProps) {
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [terminating, setTerminating] = useState(false)
  const modalRef = useModal(() => { if (!terminating) onClose() })

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setTerminating(true)
    setError('')
    try {
      await api.post('/api/sessions/terminate', {
        server_id: stream.server_id,
        session_id: stream.session_id,
        ...(stream.plex_session_uuid && { plex_session_uuid: stream.plex_session_uuid }),
        message: message.trim() || undefined,
      })
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setTerminating(false)
    }
  }

  const displayTitle = stream.grandparent_title
    ? `${stream.grandparent_title} — ${stream.title}`
    : stream.title

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
      onClick={e => { if (e.target === e.currentTarget && !terminating) onClose() }}
      role="dialog"
      aria-modal="true"
      aria-label="Terminate Stream"
    >
      <div
        ref={modalRef}
        className="card w-full max-w-md max-h-[90vh] overflow-y-auto p-0 animate-slide-up"
      >
        <div className="flex items-center justify-between px-6 py-4
                        border-b border-border dark:border-border-dark">
          <h2 className="text-lg font-semibold">Terminate Stream</h2>
          <button
            onClick={onClose}
            disabled={terminating}
            aria-label="Close"
            className="text-muted dark:text-muted-dark hover:text-gray-800
                       dark:hover:text-gray-100 transition-colors text-xl leading-none
                       disabled:opacity-50"
          >
            &times;
          </button>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-4">
          <p className="text-sm text-gray-600 dark:text-gray-300">
            Stop <span className="font-medium text-gray-900 dark:text-gray-50">{stream.user_name}</span>'s stream of <span className="font-medium text-gray-900 dark:text-gray-50">{displayTitle}</span>?
          </p>

          <div>
            <label htmlFor="terminate-message" className="block text-sm font-medium mb-1.5">
              Message (optional)
            </label>
            <textarea
              id="terminate-message"
              value={message}
              onChange={e => { setMessage(e.target.value); setError('') }}
              placeholder="Your stream has been terminated by an administrator."
              rows={3}
              maxLength={500}
              className={formInputClass}
            />
            <p className="text-xs text-muted dark:text-muted-dark mt-1">
              Displayed to the user when the stream is stopped
            </p>
          </div>

          {error && (
            <div className="text-sm text-red-500 dark:text-red-400 font-mono px-1">
              {error}
            </div>
          )}

          <div className="flex items-center gap-3 pt-2 justify-end">
            <button
              type="button"
              onClick={onClose}
              disabled={terminating}
              className="px-4 py-2.5 text-sm font-medium rounded-lg
                         border border-border dark:border-border-dark
                         hover:border-accent/30 transition-colors
                         disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={terminating}
              className="px-5 py-2.5 text-sm font-semibold rounded-lg
                         bg-red-600 text-white hover:bg-red-700
                         disabled:opacity-50 transition-colors"
            >
              {terminating ? 'Terminating...' : 'Terminate'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
