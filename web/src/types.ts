export type MediaType = 'movie' | 'episode' | 'livetv' | 'track' | 'audiobook' | 'book'
export type ServerType = 'plex' | 'emby' | 'jellyfin'
export type Role = 'admin' | 'viewer'
export type TranscodeDecision = 'direct play' | 'copy' | 'transcode'

export interface Server {
  id: number
  name: string
  type: ServerType
  url: string
  enabled: boolean
  show_recent_media: boolean
  created_at: string
  updated_at: string
}

export interface User {
  id: number
  name: string
  email: string
  role: Role
  thumb_url: string
  created_at: string
  updated_at: string
}

export interface WatchHistoryEntry {
  id: number
  server_id: number
  user_name: string
  media_type: MediaType
  title: string
  parent_title: string
  grandparent_title: string
  year: number
  duration_ms: number
  watched_ms: number
  player: string
  platform: string
  ip_address: string
  started_at: string
  stopped_at: string
  created_at: string
}

export interface ActiveStream {
  session_id: string
  server_id: number
  server_name: string
  server_type: ServerType
  user_name: string
  media_type: MediaType
  title: string
  parent_title: string
  grandparent_title: string
  year: number
  duration_ms: number
  progress_ms: number
  player: string
  platform: string
  ip_address: string
  started_at: string
  video_codec?: string
  audio_codec?: string
  video_resolution?: string
  container?: string
  bitrate?: number
  audio_channels?: number
  subtitle_codec?: string
  video_decision?: TranscodeDecision
  audio_decision?: TranscodeDecision
  transcode_hw_accel?: boolean
  transcode_progress?: number
  bandwidth?: number
  thumb_url?: string
  transcode_container?: string
  transcode_video_codec?: string
  transcode_audio_codec?: string
}

export interface DayStat {
  date: string
  movies: number
  tv: number
  livetv: number
  music: number
  audiobooks: number
  books: number
}

export interface GeoResult {
  ip: string
  lat: number
  lng: number
  city: string
  country: string
  last_seen?: string
}

export interface PaginatedResult<T> {
  items: T[]
  total: number
  page: number
  per_page: number
}

export interface OIDCSettings {
  issuer: string
  client_id: string
  client_secret: string
  redirect_url: string
  enabled: boolean
}

export interface LibraryItem {
  item_id: string
  title: string
  year?: number
  media_type: MediaType
  thumb_url?: string
  added_at: string
  server_id: number
  server_name: string
  server_type: ServerType
}

export interface CastMember {
  name: string
  role?: string
  thumb_url?: string
}

export interface ItemDetails {
  id: string
  title: string
  year?: number
  summary?: string
  media_type: MediaType
  thumb_url?: string
  genres?: string[]
  directors?: string[]
  cast?: CastMember[]
  rating?: number
  content_rating?: string
  duration_ms?: number
  studio?: string
  series_title?: string
  season_number?: number
  episode_number?: number
  server_id: number
  server_name: string
  server_type: ServerType
}

export interface MediaStat {
  title: string
  year?: number
  play_count: number
  total_hours: number
}

export interface UserStat {
  user_name: string
  play_count: number
  total_hours: number
}

export interface LibraryStat {
  total_plays: number
  total_hours: number
  unique_users: number
  unique_movies: number
  unique_tv_shows: number
}

export interface SharerAlert {
  user_name: string
  unique_ips: number
  locations: string[]
  last_seen: string
}

export interface StatsResponse {
  top_movies: MediaStat[]
  top_tv_shows: MediaStat[]
  top_users: UserStat[]
  library: LibraryStat
  concurrent_peak: number
  concurrent_peak_at?: string
  locations: GeoResult[]
  potential_sharers: SharerAlert[]
}
