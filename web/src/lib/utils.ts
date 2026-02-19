import type { Server } from '../types'

export function errorMessage(err: unknown): string {
  if (err instanceof Error) return err.message
  return String(err)
}

export function buildServerOptions(servers: Server[]): { value: string; label: string }[] {
  return servers.map(s => ({
    value: String(s.id),
    label: s.deleted_at ? `${s.name} (deleted)` : s.name,
  }))
}
