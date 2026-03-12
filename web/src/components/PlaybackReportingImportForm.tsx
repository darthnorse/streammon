import { useState, useEffect, useMemo } from 'react'
import type { Server } from '../types'
import { api } from '../lib/api'
import { formSelectClass } from '../lib/constants'
import { useModal } from '../hooks/useModal'
import { useFetch } from '../hooks/useFetch'
import { useSSEImport } from '../hooks/useSSEImport'
import { ImportProgressBar, ImportResultBanner } from './ImportProgress'

interface PlaybackReportingImportFormProps {
  onClose: () => void
}

export function PlaybackReportingImportForm({ onClose }: PlaybackReportingImportFormProps) {
  const { data: allServers } = useFetch<Server[]>('/api/servers')
  const servers = useMemo(
    () => allServers?.filter(s => (s.type === 'emby' || s.type === 'jellyfin') && !s.deleted_at),
    [allServers],
  )

  const [selectedServer, setSelectedServer] = useState<number>(0)
  const [file, setFile] = useState<File | null>(null)
  const { importing, importProgress, importResult, error, setError, clearImport, runSSEImport } = useSSEImport()

  const modalRef = useModal(onClose)

  useEffect(() => {
    if (servers && servers.length > 0) {
      setSelectedServer(prev => prev === 0 ? servers[0].id : prev)
    }
  }, [servers])

  async function handleImport() {
    if (!selectedServer) {
      setError('Please select a server')
      return
    }
    if (!file) {
      setError('Please select a TSV file')
      return
    }

    const formData = new FormData()
    formData.append('file', file)
    formData.append('server_id', String(selectedServer))

    try {
      const response = await api.uploadSSE(
        '/api/settings/playback-reporting/import',
        formData,
      )
      await runSSEImport(response)
    } catch (err) {
      if ((err as Error).name !== 'AbortError') {
        setError((err as Error).message)
      }
    }
  }

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
      onClick={e => { if (e.target === e.currentTarget) onClose() }}
      role="dialog"
      aria-modal="true"
      aria-label="Import Playback Reporting Data"
    >
      <div
        ref={modalRef}
        className="card w-full max-w-lg max-h-[90vh] overflow-y-auto p-0
                      lg:max-w-xl animate-slide-up"
      >
        <div className="flex items-center justify-between px-6 py-4
                        border-b border-border dark:border-border-dark">
          <h2 className="text-lg font-semibold">Import Playback Reporting Data</h2>
          <button
            onClick={onClose}
            aria-label="Close"
            className="text-muted dark:text-muted-dark hover:text-gray-800
                       dark:hover:text-gray-100 transition-colors text-xl leading-none"
          >
            &times;
          </button>
        </div>

        <div className="px-6 py-5 space-y-4">
          {!servers?.length ? (
            <p className="text-sm text-muted dark:text-muted-dark">
              No Emby or Jellyfin servers configured.
              Add a server in the Servers tab before importing.
            </p>
          ) : (
            <>
              <div>
                <label htmlFor="pbr-server" className="block text-sm font-medium mb-1.5">
                  Server
                </label>
                <select
                  id="pbr-server"
                  value={selectedServer}
                  onChange={e => setSelectedServer(Number(e.target.value))}
                  disabled={importing}
                  className={formSelectClass}
                >
                  {servers.map(srv => (
                    <option key={srv.id} value={srv.id}>
                      {srv.name}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label htmlFor="pbr-file" className="block text-sm font-medium mb-1.5">
                  TSV File
                </label>
                <input
                  id="pbr-file"
                  type="file"
                  accept=".tsv,.txt"
                  disabled={importing}
                  onChange={e => {
                    setFile(e.target.files?.[0] ?? null)
                    clearImport()
                  }}
                  className="block w-full text-sm file:mr-3 file:py-2 file:px-4 file:rounded-lg
                             file:border-0 file:text-sm file:font-medium file:bg-accent
                             file:text-gray-900 hover:file:bg-accent/90 file:cursor-pointer
                             file:transition-colors disabled:opacity-50"
                />
                <p className="text-xs text-muted dark:text-muted-dark mt-1">
                  Export from the Playback Reporting Plugin activity log page.
                  Supports both Emby (12-column) and Jellyfin (9-column) formats.
                </p>
              </div>

              <div className="flex items-center gap-3 pt-2">
                <button
                  type="button"
                  onClick={onClose}
                  className="px-4 py-2.5 text-sm font-medium rounded-lg
                             border border-border dark:border-border-dark
                             hover:border-accent/30 transition-colors"
                >
                  Cancel
                </button>
                <div className="flex-1" />
                <button
                  type="button"
                  onClick={handleImport}
                  disabled={importing || !selectedServer || !file}
                  className="px-5 py-2.5 text-sm font-semibold rounded-lg
                             bg-accent text-gray-900 hover:bg-accent/90
                             disabled:opacity-50 transition-colors"
                >
                  {importing ? 'Importing...' : 'Import'}
                </button>
              </div>

              {importing && importProgress && <ImportProgressBar progress={importProgress} />}

              {error && (
                <div className="text-sm text-red-500 dark:text-red-400 font-mono px-1">
                  {error}
                </div>
              )}

              {importResult && <ImportResultBanner result={importResult} />}
            </>
          )}
        </div>
      </div>
    </div>
  )
}
