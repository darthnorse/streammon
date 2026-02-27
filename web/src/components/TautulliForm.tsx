import type { IntegrationSettings } from '../types'
import { IntegrationImportForm, type IntegrationImportConfig } from './IntegrationImportForm'

const tautulliConfig: IntegrationImportConfig = {
  label: 'Tautulli',
  apiPath: '/api/settings/tautulli',
  serverType: 'plex',
  placeholderUrl: 'http://localhost:8181',
  apiKeyHelp: 'Found in Tautulli Settings \u2192 Web Interface \u2192 API Key',
  idPrefix: 'tautulli',
}

interface TautulliFormProps {
  settings?: IntegrationSettings
  onClose: () => void
  onSaved: () => void
}

export function TautulliForm({ settings, onClose, onSaved }: TautulliFormProps) {
  return (
    <IntegrationImportForm
      config={tautulliConfig}
      settings={settings}
      onClose={onClose}
      onSaved={onSaved}
    />
  )
}
