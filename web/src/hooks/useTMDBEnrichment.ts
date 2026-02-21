import { useState, useEffect } from 'react'
import { api } from '../lib/api'
import type { TMDBMovieDetails, TMDBTVDetails, TMDBMovieEnvelope, TMDBTVEnvelope } from '../types'

interface TMDBEnrichmentState {
  movie: TMDBMovieDetails | null
  tv: TMDBTVDetails | null
  loading: boolean
}

export function useTMDBEnrichment(
  tmdbId: string | undefined,
  mediaType: string | undefined
): TMDBEnrichmentState {
  const [state, setState] = useState<TMDBEnrichmentState>({
    movie: null,
    tv: null,
    loading: false,
  })

  useEffect(() => {
    if (!tmdbId) {
      setState({ movie: null, tv: null, loading: false })
      return
    }

    const isTV = mediaType === 'episode' || mediaType === 'tv'
    const endpoint = isTV
      ? `/api/tmdb/tv/${tmdbId}`
      : `/api/tmdb/movie/${tmdbId}`

    const controller = new AbortController()
    setState({ movie: null, tv: null, loading: true })

    api.get<TMDBMovieEnvelope | TMDBTVEnvelope>(endpoint, controller.signal)
      .then(data => {
        if (!controller.signal.aborted) {
          if (isTV) {
            setState({ movie: null, tv: (data as TMDBTVEnvelope).tmdb, loading: false })
          } else {
            setState({ movie: (data as TMDBMovieEnvelope).tmdb, tv: null, loading: false })
          }
        }
      })
      .catch(err => {
        if (err instanceof Error && err.name === 'AbortError') return
        if (!controller.signal.aborted) {
          setState({ movie: null, tv: null, loading: false })
        }
      })

    return () => controller.abort()
  }, [tmdbId, mediaType])

  return state
}
