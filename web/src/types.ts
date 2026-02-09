export type MediaType = 'movie' | 'episode' | 'livetv' | 'track' | 'audiobook' | 'book'
export type ServerType = 'plex' | 'emby' | 'jellyfin'
export type Role = 'admin' | 'viewer'
export type TranscodeDecision = 'direct play' | 'copy' | 'transcode'

export interface Server {
  id: number
  name: string
  type: ServerType
  url: string
  machine_id?: string
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
  has_password: boolean
  created_at: string
  updated_at: string
}

export interface AdminUser extends User {
  provider: string
  provider_id: string
}

export interface UserSummary {
  name: string
  thumb_url: string
  last_streamed_at: string | null
  last_ip: string
  total_plays: number
  total_watched_ms: number
  trust_score: number
  last_played_title: string
  last_played_grandparent_title: string
  last_played_media_type: string
  last_played_server_id: number
  last_played_item_id: string
  last_played_grandparent_item_id: string
}

export interface WatchHistoryEntry {
  id: number
  server_id: number
  item_id?: string
  grandparent_item_id?: string
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
  season_number?: number
  episode_number?: number
  thumb_url?: string
  video_resolution?: string
  transcode_decision?: TranscodeDecision
  video_codec?: string
  audio_codec?: string
  audio_channels?: number
  bandwidth?: number
  video_decision?: TranscodeDecision
  audio_decision?: TranscodeDecision
  transcode_hw_decode?: boolean
  transcode_hw_encode?: boolean
  dynamic_range?: string
  paused_ms?: number
  watched?: boolean
  // Geo fields from ip_geo_cache (populated by ListHistory)
  city?: string
  country?: string
  isp?: string
}

export interface ActiveStream {
  session_id: string
  server_id: number
  item_id?: string
  grandparent_item_id?: string
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
  transcode_hw_decode?: boolean
  transcode_hw_encode?: boolean
  transcode_progress?: number
  bandwidth?: number
  thumb_url?: string
  transcode_container?: string
  transcode_video_codec?: string
  transcode_audio_codec?: string
  transcode_video_resolution?: string
  dynamic_range?: string
  season_number?: number
  episode_number?: number
  state?: 'playing' | 'paused' | 'buffering' | 'stopped'
  paused_ms?: number
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
  ip?: string
  lat: number
  lng: number
  city: string
  country: string
  isp?: string
  last_seen?: string
  users?: string[]
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
  series_title?: string
  year?: number
  media_type: MediaType
  thumb_url?: string
  added_at: string
  server_id: number
  server_name: string
  server_type: ServerType
  season_number?: number
  episode_number?: number
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
  video_resolution?: string
  video_codec?: string
  audio_codec?: string
  audio_channels?: number
  container?: string
  bitrate?: number
  watch_history?: WatchHistoryEntry[]
}

export interface MediaStat {
  title: string
  year?: number
  play_count: number
  total_hours: number
  thumb_url?: string
  server_id?: number
  item_id?: string
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

export interface DayOfWeekStat {
  day_of_week: number
  day_name: string
  play_count: number
}

export interface HourStat {
  hour: number
  play_count: number
}

export interface DistributionStat {
  name: string
  count: number
  percentage: number
}

export interface ConcurrentTimePoint {
  time: string
  direct_play: number
  transcode: number
  total: number
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
  activity_by_day_of_week: DayOfWeekStat[]
  activity_by_hour: HourStat[]
  platform_distribution: DistributionStat[]
  player_distribution: DistributionStat[]
  quality_distribution: DistributionStat[]
  concurrent_time_series: ConcurrentTimePoint[]
}

export type LibraryType = 'movie' | 'show' | 'music' | 'other'

export interface Library {
  id: string
  server_id: number
  server_name: string
  server_type: ServerType
  name: string
  type: LibraryType
  item_count: number
  child_count: number
  grandchild_count: number
  total_size: number
}

export interface LibrariesResponse {
  libraries: Library[]
  errors?: string[]
}

export interface LocationStat {
  city: string
  country: string
  session_count: number
  percentage: number
  last_seen: string
}

export interface DeviceStat {
  player: string
  platform: string
  session_count: number
  percentage: number
  last_seen: string
}

export interface ISPStat {
  isp: string
  session_count: number
  percentage: number
  last_seen: string
}

export interface UserDetailStats {
  session_count: number
  total_hours: number
  locations: LocationStat[]
  devices: DeviceStat[]
  isps: ISPStat[]
}

export interface TautulliSettings {
  url: string
  api_key: string
}

export interface TautulliImportResult {
  imported: number
  skipped: number
  total: number
  error?: string
}

// Shared chart tooltip payload types for Recharts
export interface ChartTooltipPayloadItem {
  color: string
  name: string
  value: number
}

export interface PieTooltipPayloadItem<T> {
  name: string
  value: number
  payload: T
}

// Rules system types
export type RuleType =
  | 'impossible_travel'
  | 'concurrent_streams'
  | 'simultaneous_locations'
  | 'device_velocity'
  | 'geo_restriction'
  | 'new_device'
  | 'new_location'
  | 'isp_velocity'

export type Severity = 'info' | 'warning' | 'critical'

export type ChannelType = 'discord' | 'webhook' | 'pushover' | 'ntfy'

export interface Rule {
  id: number
  name: string
  type: RuleType
  enabled: boolean
  config: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface RuleViolation {
  id: number
  rule_id: number
  rule_name?: string
  rule_type?: RuleType
  user_name: string
  severity: Severity
  message: string
  details?: Record<string, unknown>
  confidence_score: number
  session_key?: string
  occurred_at: string
  created_at: string
}

export interface HouseholdLocation {
  id: number
  user_name: string
  ip_address?: string
  city?: string
  country?: string
  latitude?: number
  longitude?: number
  auto_learned: boolean
  trusted: boolean
  session_count: number
  first_seen: string
  last_seen: string
  created_at: string
}

export interface UserTrustScore {
  user_name: string
  score: number
  violation_count: number
  last_violation_at?: string
  updated_at: string
}

export interface NotificationChannel {
  id: number
  name: string
  channel_type: ChannelType
  config: Record<string, unknown>
  enabled: boolean
  created_at: string
  updated_at: string
}

// Rule config types
export interface ConcurrentStreamsConfig {
  max_streams: number
  exempt_household?: boolean
  count_paused_as_one?: boolean
}

export interface GeoRestrictionConfig {
  allowed_countries?: string[]
  blocked_countries?: string[]
}

export interface ImpossibleTravelConfig {
  max_speed_km_h: number
  min_distance_km: number
  time_window_hours: number
}

export interface SimultaneousLocsConfig {
  min_distance_km: number
  exempt_household?: boolean
}

export interface DeviceVelocityConfig {
  max_devices_per_hour: number
  time_window_hours: number
}

export interface NewDeviceConfig {
  notify_on_new: boolean
}

export interface NewLocationConfig {
  notify_on_new: boolean
  min_distance_km: number
  severity_threshold_km: number
  exempt_household?: boolean
}

export interface ISPVelocityConfig {
  max_isps: number
  time_window_hours: number
}

// Notification channel configs
export interface DiscordConfig {
  webhook_url: string
}

export interface WebhookConfig {
  url: string
  method?: string
  headers?: Record<string, string>
}

export interface PushoverConfig {
  user_key: string
  api_token: string
}

export interface NtfyConfig {
  server_url?: string
  topic: string
  token?: string
}

// Rule type metadata for UI display
export const RULE_TYPES: { value: RuleType; label: string; description: string }[] = [
  { value: 'concurrent_streams', label: 'Concurrent Streams', description: 'Limit simultaneous streams per user' },
  { value: 'geo_restriction', label: 'Geo Restriction', description: 'Restrict streaming by country' },
  { value: 'simultaneous_locations', label: 'Simultaneous Locations', description: 'Detect streaming from multiple locations at once' },
  { value: 'impossible_travel', label: 'Impossible Travel', description: 'Detect physically impossible location changes' },
  { value: 'device_velocity', label: 'Device Velocity', description: 'Detect too many new devices in a short time' },
  { value: 'isp_velocity', label: 'ISP Velocity', description: 'Detect too many different ISPs in a time period' },
  { value: 'new_device', label: 'New Device', description: 'Alert when user streams from new device' },
  { value: 'new_location', label: 'New Location', description: 'Alert when user streams from new location' },
]

export const RULE_TYPE_LABELS: Record<RuleType, string> = Object.fromEntries(
  RULE_TYPES.map(rt => [rt.value, rt.label])
) as Record<RuleType, string>

// Maintenance types
export type CriterionType = 'unwatched_movie' | 'unwatched_tv_none' | 'unwatched_tv_low' | 'low_resolution' | 'large_files'

export interface ParamSpec {
  name: string
  type: 'int' | 'string'
  label: string
  default: number | string
  min?: number
  max?: number
}

export interface CriterionTypeInfo {
  type: CriterionType
  name: string
  description: string
  media_types: MediaType[]
  parameters: ParamSpec[]
}

export interface LibraryItemCache {
  id: number
  server_id: number
  library_id: string
  item_id: string
  media_type: MediaType
  title: string
  year: number
  added_at: string
  video_resolution?: string
  file_size?: number
  episode_count?: number
  thumb_url?: string
  synced_at: string
}

export interface MaintenanceRule {
  id: number
  server_id: number
  library_id: string
  name: string
  criterion_type: CriterionType
  parameters: Record<string, unknown>
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface MaintenanceRuleWithCount extends MaintenanceRule {
  candidate_count: number
}

export interface MaintenanceCandidate {
  id: number
  rule_id: number
  library_item_id: number
  reason: string
  computed_at: string
  item?: LibraryItemCache
}

export interface LibraryMaintenance {
  server_id: number
  server_name: string
  library_id: string
  library_name: string
  library_type: LibraryType
  total_items: number
  last_synced_at: string | null
  rules: MaintenanceRuleWithCount[]
}

export interface MaintenanceDashboard {
  libraries: LibraryMaintenance[]
}

export interface MaintenanceCandidatesResponse {
  items: MaintenanceCandidate[]
  total: number
  total_size: number
  exclusion_count: number
  page: number
  per_page: number
}

export interface MaintenanceExclusion {
  id: number
  rule_id: number
  library_item_id: number
  excluded_by: string
  excluded_at: string
  item?: LibraryItemCache
}

export interface MaintenanceExclusionsResponse {
  items: MaintenanceExclusion[]
  total: number
  page: number
  per_page: number
}

export interface BulkDeleteError {
  candidate_id: number
  title: string
  error: string
}

export interface BulkDeleteResult {
  deleted: number
  failed: number
  skipped: number  // Items skipped because they were excluded since page load
  total_size: number
  errors?: BulkDeleteError[]
}

// Overseerr types

export interface OverseerrSettings {
  url: string
  api_key: string
}

export interface OverseerrMediaInfo {
  id?: number
  tmdbId?: number
  tvdbId?: number
  status: number // 1=UNKNOWN, 2=PENDING, 3=PROCESSING, 4=PARTIALLY_AVAILABLE, 5=AVAILABLE, 6=DELETED
  requests?: OverseerrRequest[]
}

export interface OverseerrSearchResult {
  page: number
  totalPages: number
  totalResults: number
  results: OverseerrMediaResult[]
}

export interface OverseerrMediaResult {
  id: number
  mediaType: 'movie' | 'tv' | 'person'
  posterPath?: string
  backdropPath?: string
  voteAverage?: number
  voteCount?: number
  overview?: string
  genreIds?: number[]
  originalLanguage?: string
  popularity?: number
  mediaInfo?: OverseerrMediaInfo
  // Movie fields
  title?: string
  releaseDate?: string
  // TV fields
  name?: string
  firstAirDate?: string
}

export interface OverseerrGenre {
  id: number
  name: string
}

export interface OverseerrCast {
  id: number
  character?: string
  name: string
  profilePath?: string
  order?: number
}

export interface OverseerrCrew {
  id: number
  job: string
  name: string
  department?: string
  profilePath?: string
}

export interface OverseerrSeason {
  id: number
  seasonNumber: number
  name: string
  overview?: string
  posterPath?: string
  episodeCount: number
  airDate?: string
  episodes?: OverseerrEpisode[]
}

export interface OverseerrEpisode {
  id: number
  episodeNumber: number
  seasonNumber: number
  name: string
  overview?: string
  airDate?: string
  stillPath?: string
  voteAverage?: number
}

export interface OverseerrMovieDetails {
  id: number
  title: string
  overview?: string
  posterPath?: string
  backdropPath?: string
  releaseDate?: string
  runtime?: number
  voteAverage?: number
  voteCount?: number
  genres?: OverseerrGenre[]
  status?: string
  tagline?: string
  credits?: { cast?: OverseerrCast[]; crew?: OverseerrCrew[] }
  mediaInfo?: OverseerrMediaInfo
}

export interface OverseerrTVDetails {
  id: number
  name: string
  overview?: string
  posterPath?: string
  backdropPath?: string
  firstAirDate?: string
  lastAirDate?: string
  voteAverage?: number
  voteCount?: number
  genres?: OverseerrGenre[]
  status?: string
  tagline?: string
  numberOfEpisodes?: number
  numberOfSeasons?: number
  episodeRunTime?: number[]
  inProduction?: boolean
  seasons?: OverseerrSeason[]
  credits?: { cast?: OverseerrCast[]; crew?: OverseerrCrew[] }
  mediaInfo?: OverseerrMediaInfo
  networks?: { id: number; name: string; logoPath?: string }[]
}

export interface OverseerrRequestUser {
  id: number
  email?: string
  username?: string
  plexUsername?: string
  avatar?: string
}

export interface OverseerrRequest {
  id: number
  status: number // 1=PENDING, 2=APPROVED, 3=DECLINED
  media: OverseerrMediaInfo
  createdAt: string
  updatedAt: string
  requestedBy: OverseerrRequestUser
  modifiedBy?: OverseerrRequestUser | null
  is4k: boolean
  serverId?: number
  profileId?: number
  rootFolder?: string
  type: 'movie' | 'tv'
  seasons?: number[]
}

export interface OverseerrRequestList {
  pageInfo: { page: number; pages: number; results: number }
  results: OverseerrRequest[]
}

export interface OverseerrRequestCount {
  total: number
  movie: number
  tv: number
  pending: number
  approved: number
  declined: number
  processing: number
  available: number
}

