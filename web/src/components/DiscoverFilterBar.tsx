import { useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { Dropdown, type DropdownOption } from './Dropdown'
import { ToggleSwitch } from './ToggleSwitch'
import { useTMDBGenres } from '../hooks/useTMDBGenres'
import {
  MAX_GENRES,
  setMediaType,
  yearOptions,
  type DiscoverCategoryCaps,
  type DiscoverFilters,
  type DiscoverMediaType,
  type DiscoverSort,
} from '../lib/discoverFilters'
import type { TMDBGenre } from '../types'

interface DiscoverFilterBarProps {
  caps: DiscoverCategoryCaps
  filters: DiscoverFilters
  onChange: (next: DiscoverFilters) => void
  onClear: () => void
  activeCount: number
}

const SORT_OPTIONS: DropdownOption<DiscoverSort>[] = [
  { value: 'popularity', label: 'Popularity' },
  { value: 'rating', label: 'Rating' },
  { value: 'newest', label: 'Newest first' },
]

const RATING_OPTIONS: DropdownOption<string>[] = [
  { value: '', label: 'Any rating' },
  { value: '6', label: '6+ rating' },
  { value: '7', label: '7+ rating' },
  { value: '8', label: '8+ rating' },
  { value: '9', label: '9+ rating' },
]

const TYPE_SEGMENTS: { value: DiscoverMediaType | null; label: string }[] = [
  { value: null, label: 'All' },
  { value: 'movie', label: 'Movies' },
  { value: 'tv', label: 'TV' },
]

const labelClass = 'text-xs font-medium text-muted dark:text-muted-dark'

// The selected segment sits on an accent background, so only the unselected ones
// take the accent hover colour; underlining is shared by all three.
function segmentClass(active: boolean) {
  return `px-3 py-1.5 text-sm font-medium transition-colors hover:underline ${
    active ? 'bg-accent text-white' : 'hover:bg-surface dark:hover:bg-surface-dark hover:text-accent'
  }`
}

const ARROW_DELTA: Record<string, number> = {
  ArrowRight: 1,
  ArrowDown: 1,
  ArrowLeft: -1,
  ArrowUp: -1,
}

export function DiscoverFilterBar({ caps, filters, onChange, onClear, activeCount }: DiscoverFilterBarProps) {
  const [expanded, setExpanded] = useState(false)
  const mediaType = caps.mediaType ?? filters.type
  const { genres } = useTMDBGenres(mediaType)
  // Only the server-side filters need a type: hideOwned is applied client-side.
  const disabled = caps.type && filters.type === null
  const segmentRefs = useRef<(HTMLButtonElement | null)[]>([])

  const yearOpts: DropdownOption<string>[] = [
    { value: '', label: 'Any year' },
    ...yearOptions(new Date()).map(year => ({ value: String(year), label: `${year} or newer` })),
  ]

  const genreOpts = useMemo<DropdownOption<string>[]>(
    () => genres.map(g => ({ value: String(g.id), label: g.name })),
    [genres],
  )

  const atGenreCap = filters.genres.length >= MAX_GENRES
  const disabledGenres = useMemo(
    () => (atGenreCap ? genreOpts.map(o => o.value).filter(v => !filters.genres.includes(Number(v))) : []),
    [atGenreCap, genreOpts, filters.genres],
  )

  const selectedGenres = useMemo(
    () => filters.genres.map(id => genres.find(g => g.id === id)).filter((g): g is TMDBGenre => !!g),
    [filters.genres, genres],
  )

  function handleSegmentKey(e: KeyboardEvent<HTMLButtonElement>, index: number) {
    const delta = ARROW_DELTA[e.key]
    if (!delta) return
    e.preventDefault()
    const next = (index + delta + TYPE_SEGMENTS.length) % TYPE_SEGMENTS.length
    onChange(setMediaType(filters, TYPE_SEGMENTS[next].value))
    segmentRefs.current[next]?.focus()
  }

  return (
    <div className="card p-3 mb-4">
      <div className="flex items-center gap-3 sm:hidden">
        <button
          type="button"
          aria-expanded={expanded}
          aria-controls="discover-filter-controls"
          onClick={() => setExpanded(v => !v)}
          className="px-3 py-1.5 text-sm font-medium rounded border border-border dark:border-border-dark hover:text-accent hover:underline transition-colors"
        >
          Filters ({activeCount})
        </button>
      </div>

      <div
        id="discover-filter-controls"
        className={`${expanded ? 'flex' : 'hidden'} sm:flex flex-wrap items-center gap-3 mt-3 sm:mt-0`}
      >
        {caps.type && (
          <div role="radiogroup" aria-label="Media type" className="flex rounded border border-border dark:border-border-dark overflow-hidden">
            {TYPE_SEGMENTS.map((seg, index) => (
              <button
                key={seg.label}
                type="button"
                role="radio"
                ref={el => { segmentRefs.current[index] = el }}
                aria-checked={filters.type === seg.value}
                tabIndex={filters.type === seg.value ? 0 : -1}
                onKeyDown={e => handleSegmentKey(e, index)}
                onClick={() => onChange(setMediaType(filters, seg.value))}
                className={segmentClass(filters.type === seg.value)}
              >
                {seg.label}
              </button>
            ))}
          </div>
        )}

        {caps.year && (
          <div className="flex items-center gap-2">
            <span className={labelClass}>Year</span>
            <Dropdown
              aria-label="Release year"
              disabled={disabled}
              options={yearOpts}
              value={filters.year === null ? '' : String(filters.year)}
              onChange={value => onChange({ ...filters, year: value ? Number(value) : null })}
            />
          </div>
        )}

        <div className="flex items-center gap-2">
          <span className={labelClass}>Genres</span>
          <Dropdown
            multi
            aria-label="Genres"
            disabled={disabled}
            disabledOptions={disabledGenres}
            noneLabel="Any genre"
            allLabel="All genres"
            options={genreOpts}
            selected={filters.genres.map(String)}
            onChange={values => onChange({ ...filters, genres: values.map(Number) })}
          />
        </div>

        <div className="flex items-center gap-2">
          <span className={labelClass}>Sort</span>
          <Dropdown
            aria-label="Sort by"
            disabled={disabled}
            options={SORT_OPTIONS}
            value={filters.sort}
            onChange={value => onChange({ ...filters, sort: value })}
          />
        </div>

        <div className="flex items-center gap-2">
          <span className={labelClass}>Rating</span>
          <Dropdown
            aria-label="Minimum rating"
            disabled={disabled}
            options={RATING_OPTIONS}
            value={filters.rating === null ? '' : String(filters.rating)}
            onChange={value => onChange({ ...filters, rating: value ? Number(value) : null })}
          />
        </div>

        <label className="flex items-center gap-2 cursor-pointer">
          <ToggleSwitch
            enabled={filters.hideOwned}
            onToggle={() => onChange({ ...filters, hideOwned: !filters.hideOwned })}
          />
          <span className={labelClass}>Hide items in my library</span>
        </label>

        {disabled && (
          <span className="text-xs text-muted dark:text-muted-dark">Choose Movies or TV to filter</span>
        )}
      </div>

      {selectedGenres.length > 0 && (
        <div className="flex flex-wrap gap-2 mt-3">
          {selectedGenres.map(genre => (
            <button
              key={genre.id}
              type="button"
              aria-label={`Remove ${genre.name}`}
              onClick={() => onChange({ ...filters, genres: filters.genres.filter(id => id !== genre.id) })}
              className="px-2 py-1 text-xs rounded-full border border-border dark:border-border-dark hover:text-accent hover:border-accent/30 transition-colors"
            >
              {genre.name} &times;
            </button>
          ))}
        </div>
      )}

      {activeCount > 0 && (
        <div className="flex justify-end mt-3">
          <button
            type="button"
            onClick={onClear}
            className="text-xs hover:text-accent hover:underline transition-colors"
          >
            Clear filters
          </button>
        </div>
      )}
    </div>
  )
}
