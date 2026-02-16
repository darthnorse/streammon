import type { OverseerrSettings } from '../types'
import { IntegrationForm } from './IntegrationForm'

interface OverseerrFormProps {
  settings?: OverseerrSettings
  onClose: () => void
  onSaved: () => void
}

const overseerrConfig = {
  name: 'Overseerr / Seerr',
  settingsPath: '/api/settings/overseerr',
  testPath: '/api/settings/overseerr/test',
  urlPlaceholder: 'http://localhost:5055',
  apiKeyHint: 'Found in Overseerr / Seerr Settings \u2192 General \u2192 API Key',
} as const

function renderOverseerrWarnings(url: string) {
  const showHTTPWarning = url.startsWith('http://') &&
    !url.startsWith('http://localhost') &&
    !url.startsWith('http://127.0.0.1')

  return (
    <>
      {showHTTPWarning && (
        <p className="text-xs text-amber-600 dark:text-amber-400 bg-amber-500/10 rounded-lg px-3 py-2">
          Plex token attribution requires HTTPS (or localhost). With a plain HTTP URL, Overseerr / Seerr requests will fall back to email matching to avoid sending tokens over an unencrypted connection. We strongly recommend using Overseerr / Seerr&rsquo;s &ldquo;Import Plex Users&rdquo; feature (Users &rarr; Import Plex Users) to ensure email matching works for all users. If no email match is found, requests will fall back to the Overseerr / Seerr admin account.
        </p>
      )}
      <p className="text-xs text-amber-600 dark:text-amber-400 bg-amber-500/10 rounded-lg px-3 py-2">
        If you use per-user tagging in Radarr/Sonarr, disable &ldquo;Tag Requests&rdquo; in Overseerr / Seerr Settings &rarr; Services &rarr; Radarr/Sonarr.
        A <a href="https://github.com/sct/overseerr/issues/4306" target="_blank" rel="noopener noreferrer" className="underline hover:text-amber-800 dark:hover:text-amber-300">known Overseerr bug</a> causes
        tag creation to fail with newer Radarr/Sonarr versions.
      </p>
    </>
  )
}

export function OverseerrForm({ settings, onClose, onSaved }: OverseerrFormProps) {
  return (
    <IntegrationForm
      config={overseerrConfig}
      settings={settings}
      onClose={onClose}
      onSaved={onSaved}
      renderExtra={renderOverseerrWarnings}
    />
  )
}
