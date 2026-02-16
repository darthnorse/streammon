import { useState, useEffect, useMemo } from 'react'
import { useFetch } from '../hooks/useFetch'
import { useModal } from '../hooks/useModal'
import { api } from '../lib/api'
import { errorMessage } from '../lib/utils'
import { LibraryPicker } from './LibraryPicker'
import type {
  MaintenanceRuleWithCount,
  CriterionTypeInfo,
  CriterionType,
  MediaType,
  RuleLibrary,
} from '../types'

interface MaintenanceRuleFormProps {
  rule?: MaintenanceRuleWithCount
  onClose: () => void
  onSaved: () => void
}

const fieldClass = `w-full px-3 py-2 rounded-lg border border-border dark:border-border-dark
  bg-panel dark:bg-panel-dark focus:outline-none focus:ring-2 focus:ring-accent`

export function MaintenanceRuleForm({ rule, onClose, onSaved }: MaintenanceRuleFormProps) {
  const isEdit = !!rule
  const modalRef = useModal(onClose)

  const { data: criterionTypes, loading: typesLoading, error: typesError } = useFetch<{ types: CriterionTypeInfo[] }>(
    '/api/maintenance/criterion-types'
  )

  const [name, setName] = useState(rule?.name ?? '')
  const [mediaType, setMediaType] = useState<MediaType | ''>(rule?.media_type ?? '')
  const [criterionType, setCriterionType] = useState<CriterionType | ''>(rule?.criterion_type ?? '')
  const [parameters, setParameters] = useState<Record<string, string | number>>(
    (rule?.parameters as Record<string, string | number>) ?? {}
  )
  const [libraries, setLibraries] = useState<RuleLibrary[]>(rule?.libraries ?? [])
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const availableTypes = useMemo(() => {
    if (!criterionTypes?.types || !mediaType) return []
    return criterionTypes.types.filter(ct => ct.media_types.includes(mediaType as MediaType))
  }, [criterionTypes?.types, mediaType])

  const selectedType = useMemo(
    () => availableTypes.find(ct => ct.type === criterionType),
    [availableTypes, criterionType]
  )

  // Reset criterion type when media type changes (only for new rules)
  useEffect(() => {
    if (!isEdit && mediaType) {
      setCriterionType('')
      setParameters({})
      setLibraries([])
    }
  }, [mediaType, isEdit])

  // Auto-populate defaults when criterion type changes
  useEffect(() => {
    if (!isEdit || (isEdit && criterionType !== rule?.criterion_type)) {
      const currentSelectedType = availableTypes.find(ct => ct.type === criterionType)
      if (currentSelectedType) {
        const defaults: Record<string, string | number> = {}
        for (const param of currentSelectedType.parameters) {
          defaults[param.name] = param.default
        }
        setParameters(defaults)
      } else {
        setParameters({})
      }
    }
  }, [criterionType, isEdit, rule?.criterion_type, availableTypes])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!criterionType || !mediaType) return

    setSaving(true)
    setError(null)

    try {
      if (isEdit && rule) {
        await api.put(`/api/maintenance/rules/${rule.id}`, {
          name,
          criterion_type: criterionType,
          parameters,
          enabled: rule.enabled,
          libraries,
        })
      } else {
        await api.post('/api/maintenance/rules', {
          name,
          media_type: mediaType,
          criterion_type: criterionType,
          parameters,
          enabled: true,
          libraries,
        })
      }
      onSaved()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
      onClick={e => { if (e.target === e.currentTarget) onClose() }}
    >
      <div
        ref={modalRef}
        className="card w-full max-w-lg max-h-[90vh] overflow-y-auto p-0 animate-slide-up"
      >
        <div className="flex items-center justify-between px-6 py-4 border-b border-border dark:border-border-dark">
          <h2 className="text-lg font-semibold">
            {isEdit ? 'Edit Maintenance Rule' : 'New Maintenance Rule'}
          </h2>
          <button
            onClick={onClose}
            aria-label="Close"
            className="text-muted dark:text-muted-dark hover:text-gray-800 dark:hover:text-gray-100 transition-colors text-xl leading-none"
          >
            &times;
          </button>
        </div>

        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-4">
          {error && (
            <div className="p-3 rounded-lg bg-red-500/10 text-red-500 text-sm">
              {error}
            </div>
          )}

          <div>
            <label htmlFor="maint-rule-name" className="block text-sm font-medium mb-1.5">Rule Name</label>
            <input
              id="maint-rule-name"
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
              required
              className={fieldClass}
              placeholder="e.g., Unwatched Movies > 90 days"
            />
          </div>

          <div>
            <label htmlFor="maint-media-type" className="block text-sm font-medium mb-1.5">Media Type</label>
            <select
              id="maint-media-type"
              value={mediaType}
              onChange={e => setMediaType(e.target.value as MediaType)}
              required
              disabled={isEdit}
              className={fieldClass}
            >
              <option value="">Select media type...</option>
              <option value="movie">Movies</option>
              <option value="episode">TV Shows</option>
            </select>
            {isEdit && (
              <p className="text-xs text-muted dark:text-muted-dark mt-1">
                Media type cannot be changed for existing rules.
              </p>
            )}
          </div>

          <div>
            <label htmlFor="maint-criterion-type" className="block text-sm font-medium mb-1.5">Criterion Type</label>
            {typesLoading ? (
              <div className={`${fieldClass} text-muted dark:text-muted-dark`}>
                Loading criterion types...
              </div>
            ) : typesError ? (
              <div className="p-3 rounded-lg bg-red-500/10 text-red-500 text-sm">
                Failed to load criterion types.
              </div>
            ) : (
              <select
                id="maint-criterion-type"
                value={criterionType}
                onChange={e => setCriterionType(e.target.value as CriterionType)}
                required
                disabled={!mediaType}
                className={fieldClass}
              >
                <option value="">Select a criterion...</option>
                {availableTypes.map(ct => (
                  <option key={ct.type} value={ct.type}>
                    {ct.name}
                  </option>
                ))}
              </select>
            )}
            {selectedType && (
              <p className="mt-1 text-sm text-muted dark:text-muted-dark">
                {selectedType.description}
              </p>
            )}
          </div>

          {selectedType && selectedType.parameters.length > 0 && (
            <div className="space-y-4">
              <h3 className="text-sm font-medium">Parameters</h3>
              {selectedType.parameters.map(param => (
                <div key={param.name}>
                  <label className="block text-sm text-muted dark:text-muted-dark mb-1">
                    {param.label}
                  </label>
                  <input
                    type={param.type === 'int' ? 'number' : 'text'}
                    value={parameters[param.name] ?? param.default}
                    onChange={e =>
                      setParameters(prev => {
                        let value: string | number = e.target.value
                        if (param.type === 'int') {
                          const parsed = parseInt(e.target.value, 10)
                          value = isNaN(parsed) ? (param.default as number) : parsed
                        }
                        return { ...prev, [param.name]: value }
                      })
                    }
                    min={param.min}
                    max={param.max}
                    className={fieldClass}
                  />
                </div>
              ))}
            </div>
          )}

          {mediaType && (
            <div>
              <label className="block text-sm font-medium mb-1.5">Libraries</label>
              <p className="text-xs text-muted dark:text-muted-dark mb-2">
                Select which libraries this rule applies to.
              </p>
              <LibraryPicker
                selected={libraries}
                onChange={setLibraries}
                mediaType={mediaType as MediaType}
                disabled={saving}
              />
            </div>
          )}

          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2.5 text-sm font-medium rounded-lg border border-border dark:border-border-dark hover:border-accent/30 transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={saving || !name || !criterionType || !mediaType}
              className="px-5 py-2.5 text-sm font-semibold rounded-lg bg-accent text-gray-900 hover:bg-accent/90 disabled:opacity-50 transition-colors"
            >
              {saving ? (isEdit ? 'Saving...' : 'Creating...') : (isEdit ? 'Save Changes' : 'Create Rule')}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
