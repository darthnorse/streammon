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

export const navLinks = [
  { to: '/', label: 'Dashboard', icon: '▣' },
  { to: '/history', label: 'History', icon: '☰' },
  { to: '/settings', label: 'Settings', icon: '⚙' },
] as const
