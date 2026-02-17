import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'

vi.mock('../hooks/useFetch', () => ({
  useFetch: vi.fn(),
}))

vi.mock('../hooks/useItemDetails', () => ({
  useItemDetails: vi.fn(() => ({
    data: null,
    loading: false,
    error: null,
  })),
}))

import { useFetch } from '../hooks/useFetch'
import { useItemDetails } from '../hooks/useItemDetails'
import { MaintenanceRulesTab } from '../components/MaintenanceRulesTab'
import type {
  MaintenanceRuleWithCount,
  MaintenanceCandidatesResponse,
  MaintenanceExclusionsResponse,
  LibraryItemCache,
  LibrariesResponse,
} from '../types'

const mockUseFetch = vi.mocked(useFetch)
const mockUseItemDetails = vi.mocked(useItemDetails)

const baseLibraryItem: LibraryItemCache = {
  id: 1,
  server_id: 1,
  library_id: 'lib1',
  item_id: 'item-abc',
  media_type: 'movie',
  title: 'Old Movie',
  year: 2005,
  added_at: '2024-01-01T00:00:00Z',
  synced_at: '2024-06-01T00:00:00Z',
}

const testRule: MaintenanceRuleWithCount = {
  id: 1,
  name: 'Unwatched',
  media_type: 'movie',
  criterion_type: 'unwatched_movie',
  parameters: { days: 90 },
  enabled: true,
  libraries: [{ server_id: 1, library_id: 'lib1' }],
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
  candidate_count: 5,
  exclusion_count: 2,
}

const librariesResponse: LibrariesResponse = {
  libraries: [{
    id: 'lib1',
    server_id: 1,
    server_name: 'Plex',
    server_type: 'plex',
    name: 'Movies',
    type: 'movie',
    item_count: 100,
    child_count: 0,
    grandchild_count: 0,
    total_size: 500000000,
  }],
}

const candidatesResponse: MaintenanceCandidatesResponse = {
  items: [{
    id: 10,
    rule_id: 1,
    library_item_id: 1,
    reason: 'Not watched in 90+ days',
    computed_at: '2024-06-01T00:00:00Z',
    cross_server_count: 0,
    item: baseLibraryItem,
  }],
  total: 1,
  total_size: 4000000000,
  exclusion_count: 2,
  page: 1,
  per_page: 25,
}

const exclusionsResponse: MaintenanceExclusionsResponse = {
  items: [{
    id: 20,
    rule_id: 1,
    library_item_id: 1,
    excluded_by: 'admin',
    excluded_at: '2024-06-01T00:00:00Z',
    item: { ...baseLibraryItem, title: 'Excluded Movie' },
  }],
  total: 1,
  page: 1,
  per_page: 25,
}

function fetchResult<T>(data: T | null): ReturnType<typeof useFetch> {
  return { data, loading: false, error: null, refetch: vi.fn() } as ReturnType<typeof useFetch>
}

function setupFetchMock(candidatesOverride?: MaintenanceCandidatesResponse) {
  mockUseFetch.mockImplementation((url: string | null) => {
    if (!url) return fetchResult(null)
    if (url === '/api/libraries') return fetchResult(librariesResponse)
    if (url.startsWith('/api/maintenance/rules?') || url === '/api/maintenance/rules') {
      return fetchResult({ rules: [testRule] })
    }
    if (url.includes('/candidates')) return fetchResult(candidatesOverride ?? candidatesResponse)
    if (url.includes('/exclusions')) return fetchResult(exclusionsResponse)
    return fetchResult(null)
  })
}

function renderWithRouter(ui: React.ReactElement) {
  return render(<BrowserRouter>{ui}</BrowserRouter>)
}

// Navigate from list -> candidates view by clicking the candidates button
async function navigateToCandidates() {
  // Click the "5 candidates" link or the eye icon to view candidates
  await waitFor(() => {
    expect(screen.getByText('5 candidates')).toBeInTheDocument()
  })
  fireEvent.click(screen.getByText('5 candidates'))
}

// Navigate from list -> candidates -> exclusions view
async function navigateToExclusions() {
  await navigateToCandidates()
  await waitFor(() => {
    expect(screen.getByText('Manage Exclusions')).toBeInTheDocument()
  })
  fireEvent.click(screen.getByText('Manage Exclusions'))
}

describe('MaintenanceRulesTab clickable titles', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    mockUseItemDetails.mockReturnValue({ data: null, loading: false, error: null })
    setupFetchMock()
  })

  describe('CandidatesView', () => {
    it('renders candidate title as a clickable button', async () => {
      renderWithRouter(<MaintenanceRulesTab />)
      await navigateToCandidates()

      await waitFor(() => {
        expect(screen.getByText('Old Movie')).toBeInTheDocument()
      })

      const titleButton = screen.getByRole('button', { name: 'View details for Old Movie' })
      expect(titleButton).toBeInTheDocument()
      expect(titleButton).toHaveClass('hover:text-accent')
    })

    it('opens MediaDetailModal when title is clicked', async () => {
      mockUseItemDetails.mockReturnValue({
        data: {
          id: 'item-abc',
          title: 'Old Movie',
          media_type: 'movie',
          server_id: 1,
          server_type: 'plex',
          server_name: 'Plex',
        },
        loading: false,
        error: null,
      })

      renderWithRouter(<MaintenanceRulesTab />)
      await navigateToCandidates()

      await waitFor(() => {
        expect(screen.getByText('Old Movie')).toBeInTheDocument()
      })

      fireEvent.click(screen.getByRole('button', { name: 'View details for Old Movie' }))

      await waitFor(() => {
        expect(screen.getByRole('dialog')).toBeInTheDocument()
      })
      expect(mockUseItemDetails).toHaveBeenCalledWith(1, 'item-abc')
    })

    it('shows loading spinner in modal while fetching details', async () => {
      mockUseItemDetails.mockReturnValue({ data: null, loading: true, error: null })

      renderWithRouter(<MaintenanceRulesTab />)
      await navigateToCandidates()

      await waitFor(() => {
        expect(screen.getByText('Old Movie')).toBeInTheDocument()
      })

      fireEvent.click(screen.getByRole('button', { name: 'View details for Old Movie' }))

      const dialog = await screen.findByRole('dialog')
      expect(dialog.querySelector('.animate-spin')).toBeInTheDocument()
    })

    it('renders plain text when candidate has no item', async () => {
      setupFetchMock({
        ...candidatesResponse,
        items: [{ ...candidatesResponse.items[0], item: undefined }],
      })

      renderWithRouter(<MaintenanceRulesTab />)
      await navigateToCandidates()

      await waitFor(() => {
        expect(screen.getByText('Unknown')).toBeInTheDocument()
      })
      expect(screen.queryByRole('button', { name: /View details for/ })).not.toBeInTheDocument()
    })
  })

  describe('ExclusionsView', () => {
    it('renders exclusion title as a clickable button', async () => {
      renderWithRouter(<MaintenanceRulesTab />)
      await navigateToExclusions()

      await waitFor(() => {
        expect(screen.getByText('Excluded Movie')).toBeInTheDocument()
      })

      const titleButton = screen.getByRole('button', { name: 'View details for Excluded Movie' })
      expect(titleButton).toBeInTheDocument()
      expect(titleButton).toHaveClass('hover:text-accent')
    })

    it('opens MediaDetailModal when exclusion title is clicked', async () => {
      mockUseItemDetails.mockReturnValue({
        data: {
          id: 'item-abc',
          title: 'Excluded Movie',
          media_type: 'movie',
          server_id: 1,
          server_type: 'plex',
          server_name: 'Plex',
        },
        loading: false,
        error: null,
      })

      renderWithRouter(<MaintenanceRulesTab />)
      await navigateToExclusions()

      await waitFor(() => {
        expect(screen.getByText('Excluded Movie')).toBeInTheDocument()
      })

      fireEvent.click(screen.getByRole('button', { name: 'View details for Excluded Movie' }))

      await waitFor(() => {
        expect(screen.getByRole('dialog')).toBeInTheDocument()
      })
      expect(mockUseItemDetails).toHaveBeenCalledWith(1, 'item-abc')
    })
  })
})
