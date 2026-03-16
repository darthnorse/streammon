import { describe, it, expect, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithRouter } from '../test-utils'
import { StreamCard } from '../components/StreamCard'
import { baseStream } from './fixtures'

describe('StreamCard', () => {
  it('renders user name and title', () => {
    renderWithRouter(<StreamCard stream={baseStream} />)
    expect(screen.getByText('alice')).toBeDefined()
    expect(screen.getByText('Inception')).toBeDefined()
  })

  it('renders year', () => {
    renderWithRouter(<StreamCard stream={baseStream} />)
    expect(screen.getByText('2010')).toBeDefined()
  })

  it('renders progress bar', () => {
    renderWithRouter(<StreamCard stream={baseStream} />)
    const progressBar = screen.getByRole('progressbar')
    expect(progressBar).toBeDefined()
    expect(progressBar.getAttribute('aria-valuenow')).toBe('50')
  })

  it('shows TV episode format with grandparent/parent', () => {
    renderWithRouter(
      <StreamCard stream={{
        ...baseStream,
        media_type: 'episode',
        title: 'Pilot',
        parent_title: 'Season 1',
        grandparent_title: 'Breaking Bad',
      }} />
    )
    expect(screen.getByText('Breaking Bad')).toBeDefined()
    expect(screen.getByText(/S1 · Pilot/)).toBeDefined()
  })

  it('renders transcode info when transcoding', () => {
    renderWithRouter(
      <StreamCard stream={{
        ...baseStream,
        video_decision: 'transcode',
        video_codec: 'h264',
        video_resolution: '1080p',
        transcode_video_codec: 'h264',
      }} />
    )
    expect(screen.getByText(/H264/)).toBeDefined()
    expect(screen.getByText(/Video/)).toBeDefined()
  })

  it('renders direct play info', () => {
    renderWithRouter(
      <StreamCard stream={{
        ...baseStream,
        video_decision: 'direct play',
        video_codec: 'hevc',
        video_resolution: '4K',
      }} />
    )
    expect(screen.getAllByText(/Direct Play/).length).toBeGreaterThan(0)
  })

  it('shows extra type badge and parent title for trailer', () => {
    renderWithRouter(
      <StreamCard stream={{
        ...baseStream,
        extra_type: 'trailer',
        title: 'Official Trailer',
        parent_title: 'Dune: Part Two',
      }} />
    )
    expect(screen.getByText('Trailer')).toBeDefined()
    expect(screen.getByText('Dune: Part Two')).toBeDefined()
    expect(screen.getByText('Official Trailer')).toBeDefined()
  })

  describe('clickable titles', () => {
    it('calls onTitleClick with server_id and item_id for a movie', async () => {
      const onClick = vi.fn()
      renderWithRouter(
        <StreamCard
          stream={{ ...baseStream, item_id: 'movie-1' }}
          onTitleClick={onClick}
        />
      )
      await userEvent.click(screen.getByText('Inception'))
      expect(onClick).toHaveBeenCalledWith(1, 'movie-1')
    })

    it('calls onTitleClick with grandparent_item_id when clicking series title', async () => {
      const onClick = vi.fn()
      renderWithRouter(
        <StreamCard
          stream={{
            ...baseStream,
            media_type: 'episode',
            title: 'Pilot',
            parent_title: 'Season 1',
            grandparent_title: 'Breaking Bad',
            item_id: 'ep-1',
            grandparent_item_id: 'show-1',
          }}
          onTitleClick={onClick}
        />
      )
      await userEvent.click(screen.getByText('Breaking Bad'))
      expect(onClick).toHaveBeenCalledWith(1, 'show-1')
    })

    it('calls onTitleClick with item_id when clicking episode subtitle', async () => {
      const onClick = vi.fn()
      renderWithRouter(
        <StreamCard
          stream={{
            ...baseStream,
            media_type: 'episode',
            title: 'Pilot',
            parent_title: 'Season 1',
            grandparent_title: 'Breaking Bad',
            item_id: 'ep-1',
            grandparent_item_id: 'show-1',
          }}
          onTitleClick={onClick}
        />
      )
      await userEvent.click(screen.getByText(/S1 · Pilot/))
      expect(onClick).toHaveBeenCalledWith(1, 'ep-1')
    })

    it('does not make titles clickable when item_id is absent', () => {
      const onClick = vi.fn()
      renderWithRouter(
        <StreamCard stream={baseStream} onTitleClick={onClick} />
      )
      const title = screen.getByText('Inception')
      expect(title.className).not.toContain('cursor-pointer')
    })

    it('does not make series title clickable when grandparent_item_id is absent', () => {
      const onClick = vi.fn()
      renderWithRouter(
        <StreamCard
          stream={{
            ...baseStream,
            media_type: 'episode',
            title: 'Pilot',
            parent_title: 'Season 1',
            grandparent_title: 'Breaking Bad',
            item_id: 'ep-1',
          }}
          onTitleClick={onClick}
        />
      )
      const seriesTitle = screen.getByText('Breaking Bad')
      expect(seriesTitle.className).not.toContain('cursor-pointer')
    })
  })
})
