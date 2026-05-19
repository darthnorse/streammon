import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { WatchHistory } from '../components/modals/MediaDetailParts'
import type { ItemDetails, Server } from '../types'

const baseItem = {
  id: 'movie-1',
  title: 'Inception',
  media_type: 'movie',
  server_id: 1,
  server_name: 'Plex 4K',
  server_type: 'plex',
  watch_history: [
    {
      id: 1,
      server_id: 1,
      user_name: 'alice',
      media_type: 'movie',
      title: 'Inception',
      parent_title: '',
      grandparent_title: '',
      year: 2010,
      duration_ms: 0,
      watched_ms: 0,
      player: '',
      platform: '',
      ip_address: '',
      started_at: '2026-05-19T12:00:00Z',
      stopped_at: '2026-05-19T14:00:00Z',
      created_at: '2026-05-19T14:00:00Z',
    },
    {
      id: 2,
      server_id: 2,
      user_name: 'bob',
      media_type: 'movie',
      title: 'Inception',
      parent_title: '',
      grandparent_title: '',
      year: 2010,
      duration_ms: 0,
      watched_ms: 0,
      player: '',
      platform: '',
      ip_address: '',
      started_at: '2026-05-19T13:00:00Z',
      stopped_at: '2026-05-19T15:00:00Z',
      created_at: '2026-05-19T15:00:00Z',
    },
  ],
} as ItemDetails

const servers: Server[] = [
  { id: 1, name: 'Plex 4K', type: 'plex', url: '', enabled: true } as Server,
  { id: 2, name: 'Plex HD', type: 'plex', url: '', enabled: true } as Server,
]

describe('WatchHistory cross-server pill', () => {
  it('does not render a pill for rows from the modal server', () => {
    render(
      <MemoryRouter>
        <WatchHistory item={baseItem} modalServerId={1} servers={servers} />
      </MemoryRouter>
    )
    expect(screen.queryByTestId('server-pill-1')).toBeNull()
  })

  it('renders a pill with the server name for rows from a different server', () => {
    render(
      <MemoryRouter>
        <WatchHistory item={baseItem} modalServerId={1} servers={servers} />
      </MemoryRouter>
    )
    const pill = screen.getByTestId('server-pill-2')
    expect(pill.textContent).toContain('Plex HD')
  })

  it('renders no pills when all rows are from the same server (single-server setup)', () => {
    const single = { ...baseItem, watch_history: baseItem.watch_history!.filter(e => e.server_id === 1) }
    render(
      <MemoryRouter>
        <WatchHistory item={single} modalServerId={1} servers={servers} />
      </MemoryRouter>
    )
    expect(screen.queryByTestId(/server-pill-/)).toBeNull()
  })
})
