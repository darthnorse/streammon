import { useState, useEffect, useCallback } from 'react'
import { api } from '../lib/api'
import { dispatchRequestChanged } from './useRequestCount'

interface UseOverseerrRequestParams {
  overseerrConfigured: boolean
  effectiveTmdbId: string | undefined
  effectiveMediaType: 'movie' | 'tv' | null
  seasonNumbers: number[]
}

export function useOverseerrRequest({
  overseerrConfigured,
  effectiveTmdbId,
  effectiveMediaType,
  seasonNumbers,
}: UseOverseerrRequestParams) {
  const [overseerrStatus, setOverseerrStatus] = useState<number | undefined>()
  const [overseerrChecked, setOverseerrChecked] = useState(false)
  const [requesting, setRequesting] = useState(false)
  const [requestSuccess, setRequestSuccess] = useState(false)
  const [requestError, setRequestError] = useState('')
  const [selectedSeasons, setSelectedSeasons] = useState<number[]>([])
  const [allSeasons, setAllSeasons] = useState(true)

  useEffect(() => {
    setRequesting(false)
    setRequestSuccess(false)
    setRequestError('')
  }, [effectiveTmdbId, effectiveMediaType])

  useEffect(() => {
    setSelectedSeasons(seasonNumbers)
    setAllSeasons(true)
  }, [seasonNumbers])

  useEffect(() => {
    if (!overseerrConfigured || !effectiveTmdbId || !effectiveMediaType) return
    setOverseerrStatus(undefined)
    setOverseerrChecked(false)
    const controller = new AbortController()
    const endpoint = effectiveMediaType === 'movie'
      ? `/api/overseerr/movie/${effectiveTmdbId}`
      : `/api/overseerr/tv/${effectiveTmdbId}`
    api.get<{ mediaInfo?: { status?: number } }>(endpoint, controller.signal)
      .then(data => {
        if (!controller.signal.aborted) setOverseerrStatus(data.mediaInfo?.status)
      })
      .catch(err => {
        if (err instanceof Error && err.name === 'AbortError') return
        console.warn('Overseerr status check failed:', err)
      })
      .finally(() => { if (!controller.signal.aborted) setOverseerrChecked(true) })
    return () => controller.abort()
  }, [overseerrConfigured, effectiveTmdbId, effectiveMediaType])

  // Deleted (status 7) excluded — users can re-request after maintenance deletion
  const alreadyRequested = overseerrStatus != null && overseerrStatus >= 2 && overseerrStatus <= 6
  const canRequest = overseerrConfigured && overseerrChecked && !alreadyRequested && !requestSuccess && !!effectiveTmdbId

  const handleRequest = useCallback(async () => {
    if (!effectiveTmdbId || !effectiveMediaType) return
    setRequesting(true)
    setRequestError('')
    try {
      const body: Record<string, unknown> = {
        mediaType: effectiveMediaType,
        mediaId: Number(effectiveTmdbId),
      }
      if (effectiveMediaType === 'tv') body.seasons = selectedSeasons
      await api.post('/api/overseerr/requests', body)
      setRequestSuccess(true)
      dispatchRequestChanged()
    } catch (err) {
      setRequestError(err instanceof Error ? err.message : String(err))
    } finally {
      setRequesting(false)
    }
  }, [effectiveTmdbId, effectiveMediaType, selectedSeasons])

  const toggleSeason = useCallback((num: number) => {
    if (allSeasons) {
      setAllSeasons(false)
      setSelectedSeasons([num])
      return
    }
    setSelectedSeasons(prev =>
      prev.includes(num) ? prev.filter(n => n !== num) : [...prev, num],
    )
  }, [allSeasons])

  const toggleAllSeasons = useCallback(() => {
    if (allSeasons) {
      setAllSeasons(false)
      setSelectedSeasons([])
    } else {
      setAllSeasons(true)
      setSelectedSeasons(seasonNumbers)
    }
  }, [allSeasons, seasonNumbers])

  return {
    overseerrStatus,
    requesting,
    requestSuccess,
    requestError,
    selectedSeasons,
    allSeasons,
    alreadyRequested,
    canRequest,
    handleRequest,
    toggleSeason,
    toggleAllSeasons,
  }
}
