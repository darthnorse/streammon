import type { ActiveStream, WatchHistoryEntry, DayStat, User, Server } from '../types'

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

export const baseUser: User = {
  id: 1,
  name: 'alice',
  email: 'alice@example.com',
  role: 'admin',
  thumb_url: '',
  created_at: '2024-01-15T00:00:00Z',
  updated_at: '2024-06-15T00:00:00Z',
}

export const emptyDayStat: DayStat = {
  date: '2024-06-15',
  movies: 0,
  tv: 0,
  livetv: 0,
  music: 0,
  audiobooks: 0,
  books: 0,
}

export const baseServer: Server = {
  id: 1,
  name: 'My Plex',
  type: 'plex',
  url: 'http://localhost:32400',
  enabled: true,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-06-15T00:00:00Z',
}
