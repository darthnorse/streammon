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
  connections: PlexConnection[]
}

const STORAGE_KEY = 'streammon_plex_client_id'

export function getClientId(): string {
  let id = localStorage.getItem(STORAGE_KEY)
  if (!id) {
    id = crypto.randomUUID()
    localStorage.setItem(STORAGE_KEY, id)
  }
  return id
}

export function plexHeaders(): Record<string, string> {
  return {
    'X-Plex-Product': 'StreamMon',
    'X-Plex-Client-Identifier': getClientId(),
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
  return resources.filter(r => r.provides.includes('server'))
}
