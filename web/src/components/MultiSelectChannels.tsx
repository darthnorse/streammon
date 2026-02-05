import { useFetch } from '../hooks/useFetch'
import type { NotificationChannel } from '../types'

interface MultiSelectChannelsProps {
  selectedIds: number[]
  onChange: (ids: number[]) => void
}

export function MultiSelectChannels({ selectedIds, onChange }: MultiSelectChannelsProps) {
  const { data: channels, loading, error } = useFetch<NotificationChannel[]>('/api/notifications')

  if (loading) {
    return <div className="text-sm text-muted dark:text-muted-dark">Loading channels...</div>
  }

  if (error) {
    return <div className="text-sm text-red-500 dark:text-red-400">Failed to load channels</div>
  }

  if (!channels?.length) {
    return (
      <div className="text-sm text-muted dark:text-muted-dark">
        No notification channels configured.{' '}
        <a href="/rules" className="text-accent hover:underline">Create one</a> first.
      </div>
    )
  }

  const handleToggle = (channelId: number) => {
    if (selectedIds.includes(channelId)) {
      onChange(selectedIds.filter(id => id !== channelId))
    } else {
      onChange([...selectedIds, channelId])
    }
  }

  return (
    <div className="space-y-2">
      {channels.map(channel => (
        <label
          key={channel.id}
          className="flex items-center gap-3 p-2 rounded-lg hover:bg-surface dark:hover:bg-surface-dark cursor-pointer transition-colors"
        >
          <input
            type="checkbox"
            checked={selectedIds.includes(channel.id)}
            onChange={() => handleToggle(channel.id)}
            className="w-4 h-4 rounded border-border dark:border-border-dark"
          />
          <div className="flex-1 min-w-0">
            <div className="font-medium text-sm">{channel.name}</div>
            <div className="text-xs text-muted dark:text-muted-dark capitalize">
              {channel.channel_type}
            </div>
          </div>
          {!channel.enabled && (
            <span className="text-xs text-amber-400">Disabled</span>
          )}
        </label>
      ))}
    </div>
  )
}
