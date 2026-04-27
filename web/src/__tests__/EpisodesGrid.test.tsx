import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithRouter } from '../test-utils'
import { EpisodesGrid } from '../components/modals/EpisodesGrid'
import { TMDB_IMG } from '../lib/tmdb'
import type { Episode, TMDBSeasonEpisode } from '../types'

function makeEpisode(overrides: Partial<Episode> = {}): Episode {
  return {
    id: 'ep-1',
    number: 1,
    title: 'Pilot',
    ...overrides,
  }
}

describe('EpisodesGrid', () => {
  it('renders an episode card per server episode', () => {
    const episodes: Episode[] = [
      makeEpisode({ id: 'ep-1', number: 1, title: 'Pilot' }),
      makeEpisode({ id: 'ep-2', number: 2, title: 'Cat in the Bag' }),
    ]
    renderWithRouter(<EpisodesGrid serverId={1} episodes={episodes} pushModal={vi.fn()} />)
    expect(screen.getByText(/Pilot/)).toBeInTheDocument()
    expect(screen.getByText(/Cat in the Bag/)).toBeInTheDocument()
  })

  it('renders empty state when no episodes', () => {
    const { container } = renderWithRouter(
      <EpisodesGrid serverId={1} episodes={[]} pushModal={vi.fn()} />,
    )
    expect(screen.getByText(/no episodes/i)).toBeInTheDocument()
    // No episode buttons rendered
    expect(container.querySelectorAll('button').length).toBe(0)
  })

  it('uses TMDB still when available', () => {
    const episodes: Episode[] = [makeEpisode({ id: 'ep-1', number: 1, title: 'Pilot', thumb_url: '/server/thumb.jpg' })]
    const tmdbEpisodes: TMDBSeasonEpisode[] = [
      { episode_number: 1, still_path: '/tmdb-still.jpg', overview: 'TMDB overview' },
    ]
    const { container } = renderWithRouter(
      <EpisodesGrid serverId={1} episodes={episodes} tmdbEpisodes={tmdbEpisodes} pushModal={vi.fn()} />,
    )
    const img = container.querySelector('img')
    expect(img).not.toBeNull()
    expect(img!.getAttribute('src')).toContain(`${TMDB_IMG}/w300/tmdb-still.jpg`)
  })

  it('falls back to server thumb when no TMDB still', () => {
    const episodes: Episode[] = [makeEpisode({ id: 'ep-1', number: 1, title: 'Pilot', thumb_url: '/library/thumb/abc' })]
    const { container } = renderWithRouter(
      <EpisodesGrid serverId={7} episodes={episodes} pushModal={vi.fn()} />,
    )
    const img = container.querySelector('img')
    expect(img).not.toBeNull()
    expect(img!.getAttribute('src')).toContain('/api/servers/7/thumb/library/thumb/abc')
  })

  it('prefers TMDB overview over server summary', () => {
    const episodes: Episode[] = [
      makeEpisode({ id: 'ep-1', number: 1, title: 'Pilot', summary: 'Server summary' }),
    ]
    const tmdbEpisodes: TMDBSeasonEpisode[] = [
      { episode_number: 1, overview: 'TMDB overview' },
    ]
    renderWithRouter(
      <EpisodesGrid serverId={1} episodes={episodes} tmdbEpisodes={tmdbEpisodes} pushModal={vi.fn()} />,
    )
    expect(screen.getByText('TMDB overview')).toBeInTheDocument()
    expect(screen.queryByText('Server summary')).not.toBeInTheDocument()
  })

  it('clicking an episode card calls pushModal with type=episode', async () => {
    const pushModal = vi.fn()
    const episodes: Episode[] = [
      makeEpisode({ id: 'ep-1', number: 1, title: 'Pilot' }),
      makeEpisode({ id: 'ep-2', number: 2, title: 'Second' }),
    ]
    renderWithRouter(<EpisodesGrid serverId={9} episodes={episodes} pushModal={pushModal} />)
    await userEvent.click(screen.getByText(/Second/))
    expect(pushModal).toHaveBeenCalledWith({ type: 'episode', serverId: 9, itemId: 'ep-2' })
  })
})
