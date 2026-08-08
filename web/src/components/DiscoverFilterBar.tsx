import { useMemo, useState } from 'react'
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

function segmentClass(active: boolean) {
  return `px-3 py-1.5 text-sm font-medium transition-colors ${
    active ? 'bg-accent text-white' : 'hover:bg-surface dark:hover:bg-surface-dark'
  }`
}

export function DiscoverFilterBar({ caps, filters, onChange, onClear, activeCount }: DiscoverFilterBarProps) {
  const [expanded, setExpanded] = useState(false)
  const mediaType = caps.mediaType ?? filters.type
  const { genres } = useTMDBGenres(mediaType)
  const disabled = caps.type && filters.type === null

  const yearOpts = useMemo<DropdownOption<string>[]>(() => [
    { value: '', label: 'Any year' },
    ...yearOptions(new Date()).map(year => ({ value: String(year), label: `${year} or newer` })),
  ], [])

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
    () => filters.genres.map(id => genres.find(g => g.id === id)).filter((g): g is { id: number; name: string } => !!g),
    [filters.genres, genres],
  )

  return (
    <div className="card p-3 mb-4">
      <div className="flex items-center gap-3 sm:hidden">
        <button
          type="button"
          aria-expanded={expanded}
          aria-controls="discover-filter-controls"
          onClick={() => setExpanded(v => !v)}
          className="px-3 py-1.5 text-sm font-medium rounded border border-border dark:border-border-dark"
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
            {TYPE_SEGMENTS.map(seg => (
              <button
                key={seg.label}
                type="button"
                role="radio"
                aria-checked={filters.type === seg.value}
                onClick={() => onChange(setMediaType(filters, seg.value))}
                className={segmentClass(filters.type === seg.value)}
              >
                {seg.label}
              </button>
            ))}
          </div>
        )}

        {caps.year && (
          <label className="flex items-center gap-2">
            <span className={labelClass}>Year</span>
            <Dropdown
              aria-label="Release year"
              disabled={disabled}
              options={yearOpts}
              value={filters.year === null ? '' : String(filters.year)}
              onChange={value => onChange({ ...filters, year: value ? Number(value) : null })}
            />
          </label>
        )}

        <label className="flex items-center gap-2">
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
        </label>

        <label className="flex items-center gap-2">
          <span className={labelClass}>Sort</span>
          <Dropdown
            aria-label="Sort by"
            disabled={disabled}
            options={SORT_OPTIONS}
            value={filters.sort}
            onChange={value => onChange({ ...filters, sort: value })}
          />
        </label>

        <label className="flex items-center gap-2">
          <span className={labelClass}>Rating</span>
          <Dropdown
            aria-label="Minimum rating"
            disabled={disabled}
            options={RATING_OPTIONS}
            value={filters.rating === null ? '' : String(filters.rating)}
            onChange={value => onChange({ ...filters, rating: value ? Number(value) : null })}
          />
        </label>

        <label className={`flex items-center gap-2 ${disabled ? '' : 'cursor-pointer'}`}>
          <ToggleSwitch
            enabled={filters.hideOwned}
            disabled={disabled}
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
