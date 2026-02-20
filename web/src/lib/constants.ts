import {
  LayoutDashboard,
  History,
  BarChart3,
  Library,
  Users,
  User,
  ShieldAlert,
  Settings,
  Film,
  CalendarDays,
} from 'lucide-react'
import type { MediaType, Role, Severity, ServerType, TMDBMediaResult } from '../types'

export const mediaTypeLabels: Record<MediaType, string> = {
  movie: 'Movie',
  episode: 'TV',
  livetv: 'Live TV',
  track: 'Music',
  audiobook: 'Audiobook',
  book: 'Book',
}

export const PER_PAGE_OPTIONS = [10, 25, 50] as const
export const PER_PAGE = 25
export const SEARCH_DEBOUNCE_MS = 300

export const MS_PER_MINUTE = 60_000
export const MS_PER_HOUR = 3_600_000
export const MS_PER_DAY = 86_400_000

export const plexBtnClass = 'px-4 py-2.5 text-sm font-semibold rounded-lg bg-[#e5a00d] text-gray-900 hover:bg-[#cc8e0b] transition-colors'

export const inputClass = 'w-full px-3 py-2 rounded-lg border border-border dark:border-border-dark bg-surface dark:bg-surface-dark focus:outline-none focus:ring-2 focus:ring-accent'

export const formInputClass = `w-full px-3 py-2.5 rounded-lg text-sm font-mono
  bg-surface dark:bg-surface-dark
  border border-border dark:border-border-dark
  focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20
  transition-colors placeholder:text-muted/40 dark:placeholder:text-muted-dark/40`

export const navIconMap = {
  LayoutDashboard,
  History,
  BarChart3,
  Library,
  Users,
  User,
  ShieldAlert,
  Settings,
  Film,
  CalendarDays,
} satisfies Record<string, React.ComponentType<{ className?: string }>>

export interface NavLink {
  to: string
  label: string
  icon: keyof typeof navIconMap
  visibility: 'all' | Role
  requires?: 'sonarr' | 'overseerr' | 'discover' | 'profile'
}

export interface IntegrationStatus {
  sonarr: boolean
  overseerr: boolean
  discover: boolean
  profile: boolean
}

export const navLinks: NavLink[] = [
  { to: '/', label: 'Dashboard', icon: 'LayoutDashboard', visibility: 'admin' },
  { to: '/discover', label: 'Discover', icon: 'Film', visibility: 'all', requires: 'discover' },
  { to: '/history', label: 'History', icon: 'History', visibility: 'admin' },
  { to: '/my-stats', label: 'My Stats', icon: 'User', visibility: 'viewer', requires: 'profile' },
  { to: '/statistics', label: 'Statistics', icon: 'BarChart3', visibility: 'admin' },
  { to: '/calendar', label: 'Calendar', icon: 'CalendarDays', visibility: 'all', requires: 'sonarr' },
  { to: '/library', label: 'Library', icon: 'Library', visibility: 'admin' },
  { to: '/users', label: 'Users', icon: 'Users', visibility: 'admin' },
  { to: '/rules', label: 'Rules', icon: 'ShieldAlert', visibility: 'admin' },
  { to: '/settings', label: 'Settings', icon: 'Settings', visibility: 'admin' },
]

export function visibleNavLinks(role: Role | undefined, integrations?: IntegrationStatus): NavLink[] {
  return navLinks.filter(link => {
    if (link.visibility !== 'all' && link.visibility !== role) return false
    if (link.requires && integrations && !integrations[link.requires]) return false
    return true
  })
}

export const SERVER_ACCENT: Record<ServerType, string> = {
  plex: 'bg-warn/10 text-warn',
  emby: 'bg-emby/10 text-emby',
  jellyfin: 'bg-jellyfin/10 text-jellyfin',
}

export const SEVERITY_COLORS: Record<Severity, string> = {
  info: 'bg-blue-500/20 text-blue-400',
  warning: 'bg-amber-500/20 text-amber-400',
  critical: 'bg-red-500/20 text-red-400',
}

export const DISCOVER_CATEGORIES = [
  { path: 'trending', title: 'Trending' },
  { path: 'movies', title: 'Popular Movies' },
  { path: 'movies/upcoming', title: 'Upcoming Movies' },
  { path: 'tv', title: 'Popular Series' },
  { path: 'tv/upcoming', title: 'Upcoming Series' },
] as const

export const MEDIA_GRID_CLASS = 'grid grid-cols-3 sm:[grid-template-columns:repeat(auto-fill,minmax(150px,1fr))] gap-3'

export function isSelectableMedia(r: TMDBMediaResult): boolean {
  return r.media_type === 'movie' || r.media_type === 'tv'
}

export function resolveNavLabel(link: { to: string; label: string }, integrations: IntegrationStatus): string {
  return link.to === '/discover' && integrations.overseerr ? 'Requests' : link.label
}

export const btnOutline = 'px-3 py-1.5 text-xs font-medium rounded-md border border-border dark:border-border-dark hover:border-accent/30 transition-colors'
export const btnDanger = 'px-3 py-1.5 text-xs font-medium rounded-md border border-red-300 dark:border-red-500/30 text-red-600 dark:text-red-400 hover:bg-red-500/10 transition-colors'
