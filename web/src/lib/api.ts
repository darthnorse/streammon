export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
    this.name = 'ApiError'
  }
}

async function throwIfError(res: Response): Promise<void> {
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    const { error: msg } = body as Record<string, string>
    throw new ApiError(res.status, msg || `HTTP ${res.status}`)
  }
}

// Empty-body contract: a 204 (or content-length: 0) response resolves to `undefined`,
// regardless of the declared T. Callers that don't need the response body should leave
// T unspecified so it defaults to `void`; callers that expect a body must only pass an
// explicit T for endpoints that actually return one.
async function request<T = void>(url: string, options: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...options.headers },
  })
  await throwIfError(res)
  if (res.status === 204 || res.headers.get('content-length') === '0') {
    return undefined as T
  }
  return res.json() as Promise<T>
}

export const api = {
  get<T>(url: string, signal?: AbortSignal): Promise<T> {
    return request<T>(url, { method: 'GET', signal })
  },
  post<T = void>(url: string, body?: unknown): Promise<T> {
    return request<T>(url, { method: 'POST', body: body ? JSON.stringify(body) : undefined })
  },
  put<T = void>(url: string, body?: unknown): Promise<T> {
    return request<T>(url, { method: 'PUT', body: body ? JSON.stringify(body) : undefined })
  },
  del(url: string): Promise<void> {
    return request<void>(url, { method: 'DELETE' })
  },
  async postSSE(url: string, body?: unknown, signal?: AbortSignal): Promise<Response> {
    const res = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: body ? JSON.stringify(body) : undefined,
      signal,
    })
    await throwIfError(res)
    return res
  },
  async uploadSSE(url: string, formData: FormData, signal?: AbortSignal): Promise<Response> {
    const res = await fetch(url, { method: 'POST', body: formData, signal })
    await throwIfError(res)
    return res
  },
}

export interface MaintenanceSettings {
  resolution_width_aware: boolean
}

export function getMaintenanceSettings(): Promise<MaintenanceSettings> {
  return api.get<MaintenanceSettings>('/api/settings/maintenance')
}

export function updateMaintenanceSettings(settings: MaintenanceSettings): Promise<MaintenanceSettings> {
  return api.put<MaintenanceSettings>('/api/settings/maintenance', settings)
}

export function updateUserNotes(name: string, notes: string): Promise<{ notes: string }> {
  return api.put<{ notes: string }>(`/api/users/${encodeURIComponent(name)}/notes`, { notes })
}
