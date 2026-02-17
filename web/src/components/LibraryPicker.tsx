import { useMemo } from 'react'
import { useFetch } from '../hooks/useFetch'
import { SERVER_ACCENT } from '../lib/constants'
import type { LibrariesResponse, Library, MediaType, RuleLibrary, ServerType } from '../types'

interface LibraryPickerProps {
  selected: RuleLibrary[]
  onChange: (libraries: RuleLibrary[]) => void
  mediaType: MediaType | null
  disabled?: boolean
}

const serverTypeLabel: Record<ServerType, string> = {
  plex: 'Plex',
  emby: 'Emby',
  jellyfin: 'Jellyfin',
}

interface ServerGroup {
  serverId: number
  serverName: string
  serverType: ServerType
  libraries: Library[]
}

function isSelected(selected: RuleLibrary[], serverId: number, libraryId: string): boolean {
  return selected.some(s => s.server_id === serverId && s.library_id === libraryId)
}

export function LibraryPicker({ selected, onChange, mediaType, disabled }: LibraryPickerProps) {
  const { data, loading, error } = useFetch<LibrariesResponse>('/api/libraries')

  const groups = useMemo<ServerGroup[]>(() => {
    if (!data?.libraries) return []

    const filtered = data.libraries.filter(lib => {
      if (mediaType === 'movie') return lib.type === 'movie'
      if (mediaType === 'episode') return lib.type === 'show'
      return lib.type === 'movie' || lib.type === 'show'
    })

    const groupMap = new Map<number, ServerGroup>()
    for (const lib of filtered) {
      let group = groupMap.get(lib.server_id)
      if (!group) {
        group = {
          serverId: lib.server_id,
          serverName: lib.server_name,
          serverType: lib.server_type,
          libraries: [],
        }
        groupMap.set(lib.server_id, group)
      }
      group.libraries.push(lib)
    }

    return Array.from(groupMap.values())
  }, [data?.libraries, mediaType])

  const handleToggle = (serverId: number, libraryId: string) => {
    if (disabled) return
    const exists = isSelected(selected, serverId, libraryId)
    if (exists) {
      onChange(selected.filter(s => !(s.server_id === serverId && s.library_id === libraryId)))
    } else {
      onChange([...selected, { server_id: serverId, library_id: libraryId }])
    }
  }

  if (loading) {
    return (
      <div className="card p-4 text-muted dark:text-muted-dark animate-pulse">
        Loading libraries...
      </div>
    )
  }

  if (error) {
    return (
      <div className="card p-4 text-red-500 text-sm">
        Failed to load libraries.
      </div>
    )
  }

  if (groups.length === 0) {
    return (
      <div className="card p-4 text-muted dark:text-muted-dark text-sm">
        {mediaType ? `No compatible libraries found for ${mediaType === 'movie' ? 'movies' : 'TV shows'}.` : 'No libraries found.'}
      </div>
    )
  }

  return (
    <div className="card p-4 space-y-4">
      {groups.map(group => (
        <div key={group.serverId}>
          <div className="flex items-center gap-2 mb-2">
            <span className="font-medium text-sm">{group.serverName}</span>
            <span className={`px-2 py-0.5 text-xs rounded-full ${SERVER_ACCENT[group.serverType]}`}>
              {serverTypeLabel[group.serverType]}
            </span>
          </div>
          <div className="space-y-1 pl-1">
            {group.libraries.map(lib => {
              const checked = isSelected(selected, lib.server_id, lib.id)
              const id = `lib-${lib.server_id}-${lib.id}`
              return (
                <label
                  key={id}
                  htmlFor={id}
                  className="flex items-center gap-2 py-1 cursor-pointer"
                >
                  <input
                    id={id}
                    type="checkbox"
                    checked={checked}
                    disabled={disabled}
                    onChange={() => handleToggle(lib.server_id, lib.id)}
                    className="rounded border-border dark:border-border-dark"
                  />
                  <span className="text-sm">{lib.name}</span>
                </label>
              )
            })}
          </div>
        </div>
      ))}
    </div>
  )
}
