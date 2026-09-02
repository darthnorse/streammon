import { useState } from 'react'
import { ToggleSwitch } from './ToggleSwitch'
import { useMapSettings } from '../hooks/useMapSettings'
import { setMapSettings } from '../lib/units'
import { DEFAULT_TILE_URL, validateTileUrl, validateTileAttribution } from '../lib/mapUtils'
import { formInputClass, btnOutline } from '../lib/constants'

export function MapSettings() {
  const settings = useMapSettings()
  const storedUrl = settings.tileUrl === DEFAULT_TILE_URL ? '' : settings.tileUrl

  const [tileUrl, setTileUrl] = useState(storedUrl)
  const [attribution, setAttribution] = useState(settings.attribution)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)
  const [saving, setSaving] = useState(false)

  const handleSave = async () => {
    const invalid = validateTileUrl(tileUrl) ?? validateTileAttribution(attribution)
    if (invalid) {
      setError(invalid)
      setSaved(false)
      return
    }

    setSaving(true)
    setError(null)
    try {
      // Empty is sent as-is: it clears the override rather than pinning today's default.
      await setMapSettings({ tileUrl, attribution })
      setSaved(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save map settings')
      setSaved(false)
    } finally {
      setSaving(false)
    }
  }

  const handleToggleFilter = () => {
    setMapSettings({ darkFilter: !settings.darkFilter }).catch((err) => {
      setError(err instanceof Error ? err.message : 'Failed to save map settings')
    })
  }

  return (
    <div className="card p-5">
      <h3 className="font-semibold text-base mb-4">Map Basemap</h3>
      <p className="text-sm text-muted dark:text-muted-dark mb-4">
        Tiles come from OpenStreetMap by default. Point this at another XYZ tile provider if you
        prefer a different look or need a keyed service.
      </p>

      <label className="block text-xs font-medium text-muted dark:text-muted-dark mb-1" htmlFor="map-tile-url">
        Tile URL
      </label>
      <input
        id="map-tile-url"
        type="text"
        value={tileUrl}
        onChange={(e) => setTileUrl(e.target.value)}
        placeholder={DEFAULT_TILE_URL}
        className={formInputClass}
        spellCheck={false}
      />
      <p className="text-xs text-muted dark:text-muted-dark mt-1 mb-4">
        Leave empty for the OpenStreetMap default.
      </p>

      <label className="block text-xs font-medium text-muted dark:text-muted-dark mb-1" htmlFor="map-tile-attribution">
        Attribution
      </label>
      <input
        id="map-tile-attribution"
        type="text"
        value={attribution}
        onChange={(e) => setAttribution(e.target.value)}
        placeholder="© OpenStreetMap contributors"
        className={formInputClass}
        spellCheck={false}
      />
      <p className="text-xs text-muted dark:text-muted-dark mt-1 mb-4">
        Plain text credit shown on the map. Required by most tile providers.
      </p>

      <div className="flex items-center justify-between gap-4 py-3 border-t border-gray-200 dark:border-white/10">
        <div>
          <div className="text-sm font-medium">Darken tiles in dark mode</div>
          <p className="text-xs text-muted dark:text-muted-dark mt-0.5">
            Inverts the basemap so a light tile set matches the dark theme. Turn this off if the
            tile URL above already serves a dark basemap.
          </p>
        </div>
        <ToggleSwitch
          enabled={settings.darkFilter}
          onToggle={handleToggleFilter}
          label="Darken tiles in dark mode"
        />
      </div>

      <div className="flex items-center gap-3 mt-4">
        <button type="button" onClick={handleSave} disabled={saving} className={btnOutline}>
          {saving ? 'Saving…' : 'Save'}
        </button>
        {saved && !error && (
          <span className="text-xs text-green-600 dark:text-green-400">Saved</span>
        )}
      </div>

      {error && (
        <p className="text-sm text-red-500 dark:text-red-400 mt-3">{error}</p>
      )}
    </div>
  )
}
