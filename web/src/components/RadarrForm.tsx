import type { RadarrSettings } from '../types'
import { IntegrationForm } from './IntegrationForm'

interface RadarrFormProps {
  settings?: RadarrSettings
  onClose: () => void
  onSaved: () => void
}

const radarrConfig = {
  name: 'Radarr',
  settingsPath: '/api/settings/radarr',
  testPath: '/api/settings/radarr/test',
  urlPlaceholder: 'http://localhost:7878',
  apiKeyHint: 'Found in Radarr Settings \u2192 General \u2192 API Key',
} as const

export function RadarrForm({ settings, onClose, onSaved }: RadarrFormProps) {
  return (
    <IntegrationForm
      config={radarrConfig}
      settings={settings}
      onClose={onClose}
      onSaved={onSaved}
    />
  )
}
