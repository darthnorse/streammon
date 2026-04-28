import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithRouter } from '../test-utils'
import { EpisodeDetail } from '../components/modals/EpisodeDetail'
import type { ItemDetails } from '../types'

vi.mock('../hooks/useTMDBEnrichment', () => ({
  useTMDBEnrichment: () => ({ movie: null, tv: null, loading: false }),
}))

function makeEpisodeItem(overrides: Partial<ItemDetails> = {}): ItemDetails {
  return {
    id: 'ep-1',
    title: 'Pilot',
    media_type: 'episode',
    server_id: 1,
    server_name: 'My Plex',
    server_type: 'plex',
    level: 'episode',
    series_title: 'Breaking Bad',
    series_id: 'show-1',
    parent_id: 'season-1',
    season_number: 1,
    episode_number: 1,
    ...overrides,
  }
}

describe('EpisodeDetail', () => {
  it('renders the episode title under the show name', () => {
    renderWithRouter(
      <EpisodeDetail
        item={makeEpisodeItem()}
        loading={false}
        onClose={() => {}}
        pushModal={vi.fn()}
        active={true}
      />,
    )
    expect(screen.getByText('Pilot')).toBeInTheDocument()
  })

  it('renders the episode title as the heading when there is no parent show', () => {
    renderWithRouter(
      <EpisodeDetail
        item={makeEpisodeItem({ series_title: undefined, series_id: undefined, parent_id: undefined })}
        loading={false}
        onClose={() => {}}
        pushModal={vi.fn()}
        active={true}
      />,
    )
    expect(screen.getByRole('heading', { name: 'Pilot' })).toBeInTheDocument()
  })

  it('clicking series_title pushes a show entry', async () => {
    const pushModal = vi.fn()
    renderWithRouter(
      <EpisodeDetail
        item={makeEpisodeItem()}
        loading={false}
        onClose={() => {}}
        pushModal={pushModal}
        active={true}
      />,
    )
    const showButton = screen.getByRole('button', { name: 'Breaking Bad' })
    await userEvent.click(showButton)
    expect(pushModal).toHaveBeenCalledWith({ type: 'show', serverId: 1, itemId: 'show-1' })
  })

  it('clicking S{n} pushes a season entry', async () => {
    const pushModal = vi.fn()
    renderWithRouter(
      <EpisodeDetail
        item={makeEpisodeItem()}
        loading={false}
        onClose={() => {}}
        pushModal={pushModal}
        active={true}
      />,
    )
    const seasonButton = screen.getByRole('button', { name: 'S1' })
    await userEvent.click(seasonButton)
    expect(pushModal).toHaveBeenCalledWith({ type: 'season', serverId: 1, itemId: 'season-1' })
  })

  it('does not make series_title clickable when series_id is missing', () => {
    renderWithRouter(
      <EpisodeDetail
        item={makeEpisodeItem({ series_id: undefined })}
        loading={false}
        onClose={() => {}}
        pushModal={vi.fn()}
        active={true}
      />,
    )
    expect(screen.queryByRole('button', { name: 'Breaking Bad' })).not.toBeInTheDocument()
    expect(screen.getByText('Breaking Bad')).toBeInTheDocument()
  })

  it('does not make S{n} clickable when parent_id is missing', () => {
    renderWithRouter(
      <EpisodeDetail
        item={makeEpisodeItem({ parent_id: undefined })}
        loading={false}
        onClose={() => {}}
        pushModal={vi.fn()}
        active={true}
      />,
    )
    expect(screen.queryByRole('button', { name: 'S1' })).not.toBeInTheDocument()
    expect(screen.getByText('S1')).toBeInTheDocument()
  })

  it('shows loading spinner when loading', () => {
    renderWithRouter(
      <EpisodeDetail
        item={null}
        loading={true}
        onClose={() => {}}
        pushModal={vi.fn()}
        active={true}
      />,
    )
    expect(document.querySelector('.animate-spin')).toBeInTheDocument()
  })
})
