import { describe, it, expect } from 'vitest'
import { screen } from '@testing-library/react'
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
    expect(screen.getByText(/Season 1/)).toBeDefined()
  })

  it('renders transcode badge when transcoding', () => {
    renderWithRouter(
      <StreamCard stream={{
        ...baseStream,
        video_decision: 'transcode',
        video_codec: 'h264',
        video_resolution: '1080p',
      }} />
    )
    expect(screen.getByText(/transcode/i)).toBeDefined()
    expect(screen.getByText(/h264/i)).toBeDefined()
  })

  it('renders direct play badge', () => {
    renderWithRouter(
      <StreamCard stream={{
        ...baseStream,
        video_decision: 'direct play',
        video_codec: 'hevc',
        video_resolution: '4K',
      }} />
    )
    expect(screen.getByText(/direct play/i)).toBeDefined()
  })
})
