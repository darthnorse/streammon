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
