import { useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { Dropdown, type DropdownOption } from './Dropdown'
import { ToggleSwitch } from './ToggleSwitch'
import { useTMDBGenres } from '../hooks/useTMDBGenres'
import {
  COUNTRY_OPTIONS,
  LANGUAGE_OPTIONS,
  MAX_CODES,
  MAX_GENRES,
  setMediaType,
  yearOptions,
  type DiscoverCategoryCaps,
  type DiscoverFilters,
  type DiscoverMediaType,
  type DiscoverSort,
} from '../lib/discoverFilters'

export interface SavedDefaultControls {
  hasDefault: boolean
  isDefault: boolean
  onSave: () => void
  onRemove: () => void
}

interface DiscoverFilterBarProps {
  caps: DiscoverCategoryCaps
  filters: DiscoverFilters
  onChange: (next: DiscoverFilters) => void
  onClear: () => void
  activeCount: number
  savedDefault: SavedDefaultControls
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
const textButtonClass = 'text-xs hover:text-accent hover:underline transition-colors'

type CodeField = 'countries' | 'languages'

const CODE_FILTERS: { field: CodeField; label: string; ariaLabel: string; noneLabel: string; options: readonly DropdownOption[] }[] = [
  { field: 'countries', label: 'Country', ariaLabel: 'Countries', noneLabel: 'Any country', options: COUNTRY_OPTIONS },
  { field: 'languages', label: 'Language', ariaLabel: 'Original languages', noneLabel: 'Any language', options: LANGUAGE_OPTIONS },
]

function optionsBeyondCap(options: readonly DropdownOption[], selected: readonly string[], cap: number): string[] {
  if (selected.length < cap) return []
  return options.map(o => o.value).filter(v => !selected.includes(v))
}

interface Chip {
  key: string
  label: string
  remove: () => void
}

function chipsFor(
  prefix: string,
  options: readonly DropdownOption[],
  selected: readonly string[],
  onRemove: (remaining: string[]) => void,
): Chip[] {
  return selected.flatMap(value => {
    const option = options.find(o => o.value === value)
    if (!option) return []
    return [{ key: `${prefix}-${value}`, label: option.label, remove: () => onRemove(selected.filter(v => v !== value)) }]
  })
}

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

export function DiscoverFilterBar({ caps, filters, onChange, onClear, activeCount, savedDefault }: DiscoverFilterBarProps) {
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

  const selectedGenreValues = useMemo(() => filters.genres.map(String), [filters.genres])

  const chips: Chip[] = [
    ...chipsFor('genres', genreOpts, selectedGenreValues, rest => onChange({ ...filters, genres: rest.map(Number) })),
    ...CODE_FILTERS.flatMap(({ field, options }) =>
      chipsFor(field, options, filters[field], rest => onChange({ ...filters, [field]: rest })),
    ),
  ]

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
            disabledOptions={optionsBeyondCap(genreOpts, selectedGenreValues, MAX_GENRES)}
            noneLabel="Any genre"
            allLabel="All genres"
            options={genreOpts}
            selected={selectedGenreValues}
            onChange={values => onChange({ ...filters, genres: values.map(Number) })}
          />
        </div>

        {CODE_FILTERS.map(({ field, label, ariaLabel, noneLabel, options }) => (
          <div key={field} className="flex items-center gap-2">
            <span className={labelClass}>{label}</span>
            <Dropdown
              multi
              aria-label={ariaLabel}
              disabled={disabled}
              disabledOptions={optionsBeyondCap(options, filters[field], MAX_CODES)}
              noneLabel={noneLabel}
              options={options}
              selected={filters[field]}
              onChange={values => onChange({ ...filters, [field]: values })}
            />
          </div>
        ))}

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

        {(activeCount > 0 || savedDefault.hasDefault) && (
          <div className="ml-auto flex items-center gap-3">
            {savedDefault.isDefault && (
              <span className="text-xs text-muted dark:text-muted-dark">Default filters</span>
            )}
            {activeCount > 0 && !savedDefault.isDefault && (
              <button type="button" onClick={savedDefault.onSave} className={textButtonClass}>
                Save as default
              </button>
            )}
            {savedDefault.hasDefault && (
              <button type="button" onClick={savedDefault.onRemove} className={textButtonClass}>
                Remove default
              </button>
            )}
            {activeCount > 0 && (
              <button type="button" onClick={onClear} className={textButtonClass}>
                Clear filters
              </button>
            )}
          </div>
        )}
      </div>

      {chips.length > 0 && (
        <div className="flex flex-wrap gap-2 mt-3">
          {chips.map(chip => (
            <button
              key={chip.key}
              type="button"
              aria-label={`Remove ${chip.label}`}
              onClick={chip.remove}
              className="px-2 py-1 text-xs rounded-full border border-border dark:border-border-dark hover:text-accent hover:border-accent/30 transition-colors"
            >
              {chip.label} &times;
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
