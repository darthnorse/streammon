import { useState, useEffect } from 'react'
import { getMapSettings, initDisplaySettings, type MapSettings } from '../lib/units'

// Reads from the localStorage cache first so the map paints without waiting on
// a round trip, then follows the values the server hands back.
export function useMapSettings(): MapSettings {
  const [settings, setSettings] = useState<MapSettings>(getMapSettings)

  useEffect(() => {
    initDisplaySettings()

    const handle = (e: Event) => setSettings((e as CustomEvent<MapSettings>).detail)
    window.addEventListener('map-settings-changed', handle)
    return () => window.removeEventListener('map-settings-changed', handle)
  }, [])

  return settings
}
