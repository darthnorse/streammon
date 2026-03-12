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

async function request<T>(url: string, options: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...options.headers },
  })
  await throwIfError(res)
  if (res.status === 204 || res.headers.get('content-length') === '0') {
    return undefined as unknown as T
  }
  return res.json() as Promise<T>
}

export const api = {
  get<T>(url: string, signal?: AbortSignal): Promise<T> {
    return request<T>(url, { method: 'GET', signal })
  },
  post<T>(url: string, body?: unknown): Promise<T> {
    return request<T>(url, { method: 'POST', body: body ? JSON.stringify(body) : undefined })
  },
  put<T>(url: string, body?: unknown): Promise<T> {
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
