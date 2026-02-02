import type { ActiveStream, WatchHistoryEntry } from '../types'

export const baseStream: ActiveStream = {
  session_id: 's1',
  server_id: 1,
  server_name: 'Plex',
  user_name: 'alice',
  media_type: 'movie',
  title: 'Inception',
  parent_title: '',
  grandparent_title: '',
  year: 2010,
  duration_ms: 8880000,
  progress_ms: 4440000,
  player: 'Chrome',
  platform: 'Web',
  ip_address: '10.0.0.1',
  started_at: '2024-01-01T12:00:00Z',
}

export const baseHistoryEntry: WatchHistoryEntry = {
  id: 1,
  server_id: 1,
  user_name: 'alice',
  media_type: 'movie',
  title: 'Inception',
  parent_title: '',
  grandparent_title: '',
  year: 2010,
  duration_ms: 8880000,
  watched_ms: 8880000,
  player: 'Chrome',
  platform: 'Web',
  ip_address: '10.0.0.1',
  started_at: '2024-06-15T12:00:00Z',
  stopped_at: '2024-06-15T14:28:00Z',
  created_at: '2024-06-15T12:00:00Z',
}
