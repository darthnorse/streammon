export const TMDB_IMG = 'https://image.tmdb.org/t/p'

/**
 * Key for looking a TMDB item up in the owned-library set served by
 * /api/library/tmdb-ids. TMDB numbers movies and TV in separate ID spaces, so
 * the bare number is ambiguous: movie 1399 and tv 1399 are different titles.
 */
export function ownedKey(item: { media_type: string; id: number }): string {
  return `${item.media_type}:${item.id}`
}
