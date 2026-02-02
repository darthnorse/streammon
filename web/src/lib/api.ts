class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(url: string, options: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...options.headers },
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError(res.status, (body as Record<string, string>).error || `HTTP ${res.status}`)
  }
  return res.json() as Promise<T>
}

export const api = {
  get<T>(url: string): Promise<T> {
    return request<T>(url, { method: 'GET' })
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
}
