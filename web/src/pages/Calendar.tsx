import { useState, useMemo } from 'react'
import { useFetch } from '../hooks/useFetch'
import { EmptyState } from '../components/EmptyState'
import type { SonarrEpisode } from '../types'

type CalendarView = 'week' | 'month'

const VIEW_STORAGE_KEY = 'streammon:calendar-view'

function getStoredView(): CalendarView {
  const stored = localStorage.getItem(VIEW_STORAGE_KEY)
  return stored === 'month' ? 'month' : 'week'
}

function formatDate(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

function startOfWeek(date: Date): Date {
  const d = new Date(date)
  d.setDate(d.getDate() - d.getDay())
  d.setHours(0, 0, 0, 0)
  return d
}

function addDays(date: Date, n: number): Date {
  const d = new Date(date)
  d.setDate(d.getDate() + n)
  return d
}

function getDatesInRange(start: Date, end: Date): string[] {
  const dates: string[] = []
  const d = new Date(start)
  while (d <= end) {
    dates.push(formatDate(d))
    d.setDate(d.getDate() + 1)
  }
  return dates
}

export function Calendar() {
  const [view, setView] = useState<CalendarView>(getStoredView)
  const [offset, setOffset] = useState(0)
  const { data: configured } = useFetch<{ configured: boolean }>('/api/sonarr/configured')

  const today = formatDate(new Date())

  const { start, end, label } = useMemo(() => {
    const now = new Date()
    if (view === 'week') {
      const ws = startOfWeek(now)
      ws.setDate(ws.getDate() + offset * 7)
      const we = addDays(ws, 6)
      const fmt = (d: Date) => d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
      return {
        start: formatDate(ws),
        end: formatDate(we),
        label: `${fmt(ws)} \u2013 ${fmt(we)}, ${we.getFullYear()}`,
      }
    }
    const ms = new Date(now.getFullYear(), now.getMonth() + offset, 1)
    const me = new Date(now.getFullYear(), now.getMonth() + offset + 1, 0)
    return {
      start: formatDate(ms),
      end: formatDate(me),
      label: ms.toLocaleDateString('en-US', { month: 'long', year: 'numeric' }),
    }
  }, [view, offset])

  const url = configured?.configured
    ? `/api/sonarr/calendar?start=${start}&end=${end}`
    : null
  const { data: episodes, loading, error } = useFetch<SonarrEpisode[]>(url)

  const dates = useMemo(() => {
    return getDatesInRange(new Date(start + 'T12:00:00'), new Date(end + 'T12:00:00'))
  }, [start, end])

  const grouped = useMemo(() => {
    if (!episodes) return {}
    const byDate: Record<string, SonarrEpisode[]> = {}
    for (const ep of episodes) {
      const d = ep.airDate
      if (!byDate[d]) byDate[d] = []
      byDate[d].push(ep)
    }
    return byDate
  }, [episodes])

  function switchView(v: CalendarView) {
    setView(v)
    setOffset(0)
    localStorage.setItem(VIEW_STORAGE_KEY, v)
  }

  if (configured && !configured.configured) {
    return (
      <EmptyState
        icon="&#128197;"
        title="Sonarr Not Configured"
        description="Connect Sonarr in Settings to view your TV calendar."
      />
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold">Calendar</h1>
          <p className="text-sm text-muted dark:text-muted-dark mt-1">
            Upcoming TV episodes from Sonarr
          </p>
        </div>
        <div className="flex gap-1">
          {(['week', 'month'] as const).map(v => (
            <button
              key={v}
              onClick={() => switchView(v)}
              className={`px-3 py-1.5 rounded text-xs font-medium transition-colors capitalize
                ${view === v
                  ? 'bg-accent/15 text-accent-dim dark:text-accent'
                  : 'text-muted dark:text-muted-dark hover:text-gray-800 dark:hover:text-gray-200'
                }`}
            >
              {v}
            </button>
          ))}
        </div>
      </div>

      <div className="flex items-center justify-between">
        <div className="flex gap-1">
          <button
            onClick={() => setOffset(o => o - 1)}
            className="px-3 py-1.5 text-sm font-medium rounded-lg
                       border border-border dark:border-border-dark
                       hover:border-accent/30 transition-colors"
          >
            &larr;
          </button>
          <button
            onClick={() => setOffset(o => o + 1)}
            className="px-3 py-1.5 text-sm font-medium rounded-lg
                       border border-border dark:border-border-dark
                       hover:border-accent/30 transition-colors"
          >
            &rarr;
          </button>
        </div>
        <h2 className="text-lg font-semibold">{label}</h2>
        <button
          onClick={() => setOffset(0)}
          className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors
            ${offset === 0
              ? 'bg-accent/15 text-accent-dim dark:text-accent'
              : 'border border-border dark:border-border-dark hover:border-accent/30'
            }`}
        >
          Today
        </button>
      </div>

      {loading && (
        <div className="card p-12 text-center">
          <div className="text-muted dark:text-muted-dark animate-pulse">Loading calendar...</div>
        </div>
      )}

      {error && (
        <div className="card p-6 text-center text-red-500 dark:text-red-400">
          Failed to load calendar
        </div>
      )}

      {!loading && !error && episodes && (
        <div className="space-y-6">
          {dates.map(date => {
            const dayEps = grouped[date]
            if (!dayEps?.length && view === 'month') return null
            return (
              <CalendarDay
                key={date}
                date={date}
                episodes={dayEps ?? []}
                isToday={date === today}
              />
            )
          })}
          {episodes.length === 0 && (
            <EmptyState
              icon="&#128250;"
              title="No episodes scheduled"
              description={`Nothing airing ${view === 'week' ? 'this week' : 'this month'}.`}
            />
          )}
        </div>
      )}
    </div>
  )
}

interface CalendarDayProps {
  date: string
  episodes: SonarrEpisode[]
  isToday: boolean
}

function CalendarDay({ date, episodes, isToday }: CalendarDayProps) {
  const dateObj = new Date(date + 'T12:00:00')
  const dayName = dateObj.toLocaleDateString('en-US', { weekday: 'long' })
  const monthDay = dateObj.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })

  return (
    <div>
      <div className="flex items-center gap-3 mb-3">
        <div className={`flex items-center gap-2 ${isToday ? 'text-accent' : ''}`}>
          <h3 className="text-base font-semibold">{dayName}</h3>
          <span className={`text-sm ${isToday ? 'text-accent' : 'text-muted dark:text-muted-dark'}`}>
            {monthDay}
          </span>
        </div>
        {isToday && <span className="badge badge-accent text-[10px]">Today</span>}
      </div>
      {episodes.length === 0 ? (
        <div className="card p-4 text-sm text-muted dark:text-muted-dark">
          No episodes
        </div>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-3">
          {episodes.map(ep => (
            <EpisodeCard key={ep.id} episode={ep} />
          ))}
        </div>
      )}
    </div>
  )
}

interface EpisodeCardProps {
  episode: SonarrEpisode
}

function episodeStatus(ep: SonarrEpisode): { className: string; label: string } {
  if (ep.hasFile) return { className: 'bg-green-500/20 text-green-400', label: 'Downloaded' }
  if (ep.monitored) return { className: 'bg-accent/20 text-accent', label: 'Upcoming' }
  return { className: 'bg-gray-500/20 text-gray-400', label: 'Unmonitored' }
}

function EpisodeCard({ episode }: EpisodeCardProps) {
  const posterUrl = `/api/sonarr/poster/${episode.seriesId}`
  const status = episodeStatus(episode)

  const epCode = `S${String(episode.seasonNumber).padStart(2, '0')}E${String(episode.episodeNumber).padStart(2, '0')}`

  const airTime = new Date(episode.airDateUtc).toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
  })

  return (
    <div className="card overflow-hidden group hover:ring-1 hover:ring-accent/30 transition-all">
      <div className="aspect-[2/3] bg-surface dark:bg-surface-dark relative overflow-hidden">
        <img
          src={posterUrl}
          alt={episode.series.title}
          className="w-full h-full object-cover"
          loading="lazy"
          onError={e => {
            const target = e.currentTarget
            target.style.display = 'none'
          }}
        />
        <div className="absolute inset-0 bg-gradient-to-t from-black/70 via-transparent to-transparent" />
        <div className="absolute top-2 right-2">
          <span className={`inline-flex px-1.5 py-0.5 rounded text-[10px] font-semibold ${status.className}`}>
            {status.label}
          </span>
        </div>
        <div className="absolute bottom-0 left-0 right-0 p-2.5">
          <p className="text-white text-xs font-semibold truncate drop-shadow-lg">
            {episode.series.title}
          </p>
          <p className="text-white/70 text-[10px] font-mono drop-shadow-lg">
            {epCode}
          </p>
        </div>
      </div>
      <div className="p-2.5 space-y-1">
        <p className="text-xs font-medium truncate" title={episode.title}>
          {episode.title}
        </p>
        <div className="flex items-center justify-between text-[10px] text-muted dark:text-muted-dark">
          <span>{airTime}</span>
          {episode.series.network && (
            <span className="truncate ml-1">{episode.series.network}</span>
          )}
        </div>
      </div>
    </div>
  )
}
