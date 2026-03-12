import { useState, useCallback } from 'react'
import type { ImportResult, ImportProgress } from '../types'
import { readSSEStream } from '../lib/sse'
import { useMountedRef } from './useMountedRef'

interface SSEImportState {
  importing: boolean
  importProgress: ImportProgress | null
  importResult: ImportResult | null
  error: string
  setError: (err: string) => void
  clearImport: () => void
  runSSEImport: (response: Response) => Promise<void>
}

export function useSSEImport(): SSEImportState {
  const [importing, setImporting] = useState(false)
  const [importProgress, setImportProgress] = useState<ImportProgress | null>(null)
  const [importResult, setImportResult] = useState<ImportResult | null>(null)
  const [error, setError] = useState('')
  const mountedRef = useMountedRef()

  const clearImport = useCallback(() => {
    setImportResult(null)
    setImportProgress(null)
    setError('')
  }, [])

  const runSSEImport = useCallback(async (response: Response) => {
    setImporting(true)
    setImportResult(null)
    setImportProgress(null)
    setError('')

    try {
      await readSSEStream(response, {
        onData(raw) {
          if (!mountedRef.current) return
          try {
            const data = JSON.parse(raw) as ImportProgress
            setImportProgress(data)

            if (data.type === 'complete') {
              setImportResult({
                imported: data.inserted,
                skipped: data.skipped,
                consolidated: data.consolidated,
                total: data.total,
              })
            } else if (data.type === 'error') {
              setError(data.error || 'Import failed')
              setImportResult({
                imported: data.inserted,
                skipped: data.skipped,
                consolidated: data.consolidated,
                total: data.total,
                error: data.error,
              })
            }
          } catch {}
        },
      })
    } catch (err) {
      if ((err as Error).name !== 'AbortError' && mountedRef.current) {
        setError((err as Error).message)
      }
    } finally {
      if (mountedRef.current) setImporting(false)
    }
  }, [mountedRef])

  return { importing, importProgress, importResult, error, setError, clearImport, runSSEImport }
}
