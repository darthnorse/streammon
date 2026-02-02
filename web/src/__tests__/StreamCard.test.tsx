import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import { StreamCard } from '../components/StreamCard'
import type { ActiveStream } from '../types'

const baseStream: ActiveStream = {
  session_id: 's1',
  server_id: 1,
  server_name: 'Plex',
  user_name: 'alice',
  media_type: 'movie',
  title: 'Inception',
  parent_title: '',
  grandparent_title: '',
  year: 2010,
  duration_ms: 8880000,
  progress_ms: 4440000,
  player: 'Chrome',
  platform: 'Web',
  ip_address: '10.0.0.1',
  started_at: '2024-01-01T12:00:00Z',
}

describe('StreamCard', () => {
  it('renders user name and title', () => {
    render(<BrowserRouter><StreamCard stream={baseStream} /></BrowserRouter>)
    expect(screen.getByText('alice')).toBeDefined()
    expect(screen.getByText('Inception')).toBeDefined()
  })

  it('renders year', () => {
    render(<BrowserRouter><StreamCard stream={baseStream} /></BrowserRouter>)
    expect(screen.getByText('2010')).toBeDefined()
  })

  it('renders progress bar', () => {
    render(<BrowserRouter><StreamCard stream={baseStream} /></BrowserRouter>)
    const progressBar = screen.getByRole('progressbar')
    expect(progressBar).toBeDefined()
    expect(progressBar.getAttribute('aria-valuenow')).toBe('50')
  })

  it('shows TV episode format with grandparent/parent', () => {
    const tvStream: ActiveStream = {
      ...baseStream,
      media_type: 'episode',
      title: 'Pilot',
      parent_title: 'Season 1',
      grandparent_title: 'Breaking Bad',
    }
    render(<BrowserRouter><StreamCard stream={tvStream} /></BrowserRouter>)
    expect(screen.getByText('Breaking Bad')).toBeDefined()
    expect(screen.getByText(/Season 1/)).toBeDefined()
  })

  it('renders transcode badge when transcoding', () => {
    const transcodeStream: ActiveStream = {
      ...baseStream,
      video_decision: 'transcode',
      video_codec: 'h264',
      video_resolution: '1080p',
    }
    render(<BrowserRouter><StreamCard stream={transcodeStream} /></BrowserRouter>)
    expect(screen.getByText(/transcode/i)).toBeDefined()
    expect(screen.getByText(/h264/i)).toBeDefined()
  })

  it('renders direct play badge', () => {
    const directStream: ActiveStream = {
      ...baseStream,
      video_decision: 'direct play',
      video_codec: 'hevc',
      video_resolution: '4K',
    }
    render(<BrowserRouter><StreamCard stream={directStream} /></BrowserRouter>)
    expect(screen.getByText(/direct play/i)).toBeDefined()
  })
})
