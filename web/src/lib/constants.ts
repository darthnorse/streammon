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

export const navLinks = [
  { to: '/', label: 'Dashboard', icon: 'LayoutDashboard' },
  { to: '/history', label: 'History', icon: 'History' },
  { to: '/statistics', label: 'Statistics', icon: 'BarChart3' },
  { to: '/library', label: 'Library', icon: 'Library' },
  { to: '/users', label: 'Users', icon: 'Users' },
  { to: '/rules', label: 'Rules', icon: 'ShieldAlert' },
  { to: '/settings', label: 'Settings', icon: 'Settings' },
] as const

export const SEVERITY_COLORS: Record<Severity, string> = {
  info: 'bg-blue-500/20 text-blue-400',
  warning: 'bg-amber-500/20 text-amber-400',
  critical: 'bg-red-500/20 text-red-400',
}
