import type { IntegrationSettings } from '../types'
import { btnOutline, btnDanger } from '../lib/constants'
import { EmptyState } from './EmptyState'

interface IntegrationCardProps {
  name: string
  icon: string
  description: string
  settings: IntegrationSettings | null | undefined
  loading: boolean
  error: Error | string | null
  onConfigure: () => void
  onEdit: () => void
  onDelete: () => void
  onRetry: () => void
}

export function IntegrationCard({ name, icon, description, settings, loading, error, onConfigure, onEdit, onDelete, onRetry }: IntegrationCardProps) {
  const configured = !!settings?.url

  if (loading) {
    return <EmptyState icon="&#8635;" title="Loading..." />
  }

  if (error) {
    return (
      <EmptyState icon="!" title={`Failed to load ${name} settings`}>
        <button onClick={onRetry} className="text-sm text-accent hover:underline">Retry</button>
      </EmptyState>
    )
  }

  if (!configured || !settings) {
    return (
      <EmptyState icon={icon} title={`${name} Not Configured`} description={description}>
        <button
          onClick={onConfigure}
          className="px-4 py-2.5 text-sm font-semibold rounded-lg
                     bg-accent text-gray-900 hover:bg-accent/90 transition-colors"
        >
          Configure {name}
        </button>
      </EmptyState>
    )
  }

  return (
    <div className="card p-5">
      <div className="flex items-start justify-between mb-4">
        <h3 className="font-semibold text-base">{name}</h3>
        <span className={`badge ${settings.enabled ? 'badge-accent' : 'badge-muted'}`}>
          {settings.enabled ? 'Enabled' : 'Disabled'}
        </span>
      </div>
      <div className="space-y-2 text-sm mb-4">
        <div>
          <span className="text-muted dark:text-muted-dark">URL: </span>
          <span className="font-mono">{settings.url}</span>
        </div>
      </div>
      <div className="flex items-center gap-2 border-t border-border dark:border-border-dark pt-3">
        <button onClick={onEdit} className={btnOutline}>
          Edit
        </button>
        <button onClick={onDelete} className={btnDanger}>
          Remove
        </button>
      </div>
    </div>
  )
}
