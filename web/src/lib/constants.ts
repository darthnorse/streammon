import type { MediaType } from '../types'

export const mediaTypeLabels: Record<MediaType, string> = {
  movie: 'Movie',
  episode: 'TV',
  livetv: 'Live TV',
  track: 'Music',
  audiobook: 'Audiobook',
  book: 'Book',
}

export const PER_PAGE = 20

export const MS_PER_MINUTE = 60_000
export const MS_PER_HOUR = 3_600_000
export const MS_PER_DAY = 86_400_000

export const plexBtnClass = 'px-4 py-2.5 text-sm font-semibold rounded-lg bg-[#e5a00d] text-gray-900 hover:bg-[#cc8e0b] transition-colors'

export const navLinks = [
  { to: '/', label: 'Dashboard', icon: '▣' },
  { to: '/history', label: 'History', icon: '☰' },
  { to: '/library', label: 'Library', icon: '▤' },
  { to: '/statistics', label: 'Statistics', icon: '◐' },
  { to: '/rules', label: 'Rules', icon: '⚑' },
  { to: '/settings', label: 'Settings', icon: '⚙' },
] as const
