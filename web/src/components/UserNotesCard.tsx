import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { updateUserNotes } from '../lib/api'

interface UserNotesCardProps {
  userName: string
}

export function UserNotesCard({ userName }: UserNotesCardProps) {
  const encoded = encodeURIComponent(userName)
  const { data, loading, error, refetch } = useFetch<{ notes: string }>(`/api/users/${encoded}/notes`)
  const notes = data?.notes ?? ''

  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const [saving, setSaving] = useState(false)

  const startEdit = () => {
    setDraft(notes)
    setEditing(true)
  }

  const save = async () => {
    setSaving(true)
    try {
      await updateUserNotes(userName, draft)
      setEditing(false)
      refetch()
    } finally {
      setSaving(false)
    }
  }

  if (loading && !data) return null

  return (
    <div className="card p-4">
      <div className="flex items-center justify-between mb-2">
        <h2 className="text-sm font-semibold text-muted dark:text-muted-dark">Notes</h2>
        {!editing && !error && (
          <button onClick={startEdit} className="text-sm hover:text-accent hover:underline">
            {notes ? 'Edit' : '+ Add a note'}
          </button>
        )}
      </div>

      {error && !editing ? (
        <div className="flex items-center justify-between gap-2">
          <p className="text-sm text-muted dark:text-muted-dark">Failed to load notes.</p>
          <button onClick={() => refetch()} className="text-sm hover:text-accent hover:underline">
            Retry
          </button>
        </div>
      ) : editing ? (
        <div className="space-y-2">
          <textarea
            value={draft}
            onChange={e => setDraft(e.target.value)}
            rows={4}
            maxLength={5000}
            placeholder="Private note about this user (admins only)"
            className="w-full rounded-lg border border-border dark:border-border-dark bg-transparent p-2 text-sm"
          />
          <div className="flex gap-2">
            <button
              onClick={save}
              disabled={saving}
              className="px-4 py-2 text-sm font-medium rounded-lg bg-accent text-gray-900 hover:bg-accent/90 disabled:opacity-50"
            >
              {saving ? 'Saving…' : 'Save'}
            </button>
            <button
              onClick={() => setEditing(false)}
              disabled={saving}
              className="px-4 py-2 text-sm hover:text-accent hover:underline disabled:opacity-50"
            >
              Cancel
            </button>
          </div>
        </div>
      ) : notes ? (
        <p className="text-sm whitespace-pre-wrap">{notes}</p>
      ) : (
        <p className="text-sm text-muted dark:text-muted-dark">No note yet.</p>
      )}
    </div>
  )
}
