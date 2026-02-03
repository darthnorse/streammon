import { useState, useEffect } from 'react'
import { api } from '../lib/api'
import type { ItemDetails } from '../types'

interface ItemDetailsState {
  data: ItemDetails | null
  loading: boolean
  error: Error | null
}

export function useItemDetails(serverId: number, itemId: string | null): ItemDetailsState {
  const [state, setState] = useState<ItemDetailsState>({
    data: null,
    loading: false,
    error: null,
  })

  useEffect(() => {
    if (!itemId || !serverId) {
      setState({ data: null, loading: false, error: null })
      return
    }

    const controller = new AbortController()
    setState({ data: null, loading: true, error: null })

    const url = `/api/servers/${serverId}/items/${itemId}`
    api.get<ItemDetails>(url, controller.signal)
      .then(data => {
        if (!controller.signal.aborted) {
          setState({ data, loading: false, error: null })
        }
      })
      .catch(err => {
        if (err instanceof Error && err.name === 'AbortError') {
          return
        }
        if (!controller.signal.aborted) {
          setState({ data: null, loading: false, error: err as Error })
        }
      })

    return () => {
      controller.abort()
    }
  }, [serverId, itemId])

  return state
}
