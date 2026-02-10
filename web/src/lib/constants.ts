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
} from 'lucide-react'
import type { MediaType, Severity } from '../types'

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
} satisfies Record<string, React.ComponentType<{ className?: string }>>

export const navLinks = [
  { to: '/', label: 'Dashboard', icon: 'LayoutDashboard', adminOnly: true },
  { to: '/requests', label: 'Requests', icon: 'Film', adminOnly: false },
  { to: '/history', label: 'History', icon: 'History', adminOnly: true },
  { to: '/my-stats', label: 'My Stats', icon: 'User', adminOnly: false },
  { to: '/statistics', label: 'Statistics', icon: 'BarChart3', adminOnly: true },
  { to: '/library', label: 'Library', icon: 'Library', adminOnly: true },
  { to: '/users', label: 'Users', icon: 'Users', adminOnly: true },
  { to: '/rules', label: 'Rules', icon: 'ShieldAlert', adminOnly: true },
  { to: '/settings', label: 'Settings', icon: 'Settings', adminOnly: true },
] as const

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
