import type { IntegrationSettings } from '../types'
import { IntegrationImportForm, type IntegrationImportConfig } from './IntegrationImportForm'

const jellystatConfig: IntegrationImportConfig = {
  label: 'Jellystat',
  apiPath: '/api/settings/jellystat',
  serverType: 'jellyfin',
  placeholderUrl: 'http://localhost:3000',
  apiKeyHelp: 'Found in Jellystat Settings \u2192 API Key',
  idPrefix: 'jellystat',
}

interface JellystatFormProps {
  settings?: IntegrationSettings
  onClose: () => void
  onSaved: () => void
}

export function JellystatForm({ settings, onClose, onSaved }: JellystatFormProps) {
  return (
    <IntegrationImportForm
      config={jellystatConfig}
      settings={settings}
      onClose={onClose}
      onSaved={onSaved}
    />
  )
}
