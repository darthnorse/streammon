export interface PlexPin {
  id: number
  code: string
  authToken: string | null
}

export interface PlexConnection {
  uri: string
  local: boolean
  relay: boolean
  protocol: string
}

export interface PlexResource {
  name: string
  clientIdentifier: string
  accessToken: string
  provides: string
  owned: boolean
  connections: PlexConnection[]
}

const STORAGE_KEY = 'streammon_plex_client_id'

function generateUUID(): string {
  // Fallback for browsers without crypto.randomUUID (non-HTTPS or older browsers)
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID()
  }
  // Simple UUID v4 fallback
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
    const r = Math.random() * 16 | 0
    const v = c === 'x' ? r : (r & 0x3 | 0x8)
    return v.toString(16)
  })
}

export function getClientId(): string {
  let id = localStorage.getItem(STORAGE_KEY)
  if (!id) {
    id = generateUUID()
    localStorage.setItem(STORAGE_KEY, id)
  }
  return id
}

export function plexHeaders(): Record<string, string> {
  return {
    'X-Plex-Product': 'StreamMon',
    'X-Plex-Version': '1.0.0',
    'X-Plex-Client-Identifier': getClientId(),
    'X-Plex-Platform': 'Web',
    'X-Plex-Platform-Version': navigator.userAgent,
    'X-Plex-Device': 'Browser',
    'X-Plex-Device-Name': 'StreamMon Web',
    'Accept': 'application/json',
  }
}

async function plexFetch<T>(url: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(url, {
    ...options,
    headers: { ...plexHeaders(), ...options.headers },
  })
  if (!res.ok) {
    throw new Error(`Plex API error: ${res.status}`)
  }
  return res.json() as Promise<T>
}

export function requestPin(): Promise<PlexPin> {
  return plexFetch<PlexPin>('https://plex.tv/api/v2/pins?strong=true', {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
  })
}

export function checkPin(pinId: number): Promise<PlexPin> {
  return plexFetch<PlexPin>(`https://plex.tv/api/v2/pins/${pinId}`, {
    method: 'GET',
  })
}

export function getAuthUrl(clientId: string, code: string): string {
  const params = new URLSearchParams({
    clientID: clientId,
    code,
    'context[device][product]': 'StreamMon',
  })
  return `https://app.plex.tv/auth/#!?${params.toString()}`
}

export async function fetchResources(token: string): Promise<PlexResource[]> {
  const resources = await plexFetch<PlexResource[]>(
    'https://plex.tv/api/v2/resources?includeHttps=1&includeRelay=1',
    { headers: { 'X-Plex-Token': token } },
  )
  return resources.filter(r => r.provides.includes('server') && r.owned)
}
