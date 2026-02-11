import type { SonarrSettings } from '../types'
import { IntegrationForm } from './IntegrationForm'

interface SonarrFormProps {
  settings?: SonarrSettings
  onClose: () => void
  onSaved: () => void
}

const sonarrConfig = {
  name: 'Sonarr',
  settingsPath: '/api/settings/sonarr',
  testPath: '/api/settings/sonarr/test',
  urlPlaceholder: 'http://localhost:8989',
  apiKeyHint: 'Found in Sonarr Settings \u2192 General \u2192 API Key',
} as const

export function SonarrForm({ settings, onClose, onSaved }: SonarrFormProps) {
  return (
    <IntegrationForm
      config={sonarrConfig}
      settings={settings}
      onClose={onClose}
      onSaved={onSaved}
    />
  )
}
