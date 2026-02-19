import type { ActiveStream, WatchHistoryEntry, DayStat, User, Server, StatsResponse } from '../types'

export const baseStream: ActiveStream = {
  session_id: 's1',
  server_id: 1,
  server_name: 'Plex',
  server_type: 'plex',
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
  watched: true,
  session_count: 1,
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

export function createMockStats(overrides: Partial<StatsResponse> = {}): StatsResponse {
  return {
    top_movies: [],
    top_tv_shows: [],
    top_users: [],
    library: { total_plays: 0, total_hours: 0, unique_users: 0, unique_movies: 0, unique_tv_shows: 0 },
    locations: [],
    activity_by_day_of_week: [],
    activity_by_hour: [],
    platform_distribution: [],
    player_distribution: [],
    quality_distribution: [],
    concurrent_time_series: [],
    concurrent_peaks: { total: 0, direct_play: 0, direct_stream: 0, transcode: 0 },
    ...overrides,
  }
}

export const baseServer: Server = {
  id: 1,
  name: 'My Plex',
  type: 'plex',
  url: 'http://localhost:32400',
  machine_id: 'abc123machine',
  enabled: true,
  show_recent_media: false,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-06-15T00:00:00Z',
}
