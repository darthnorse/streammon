import { describe, it, expect, vi, afterEach } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithRouter, makeAuthContext } from '../test-utils'
import { ViolationsTable } from '../components/ViolationsTable'
import type { RuleViolation } from '../types'

vi.mock('../context/AuthContext', () => ({
  useAuth: vi.fn(),
}))

import { useAuth } from '../context/AuthContext'
const mockUseAuth = vi.mocked(useAuth)

afterEach(() => {
  vi.restoreAllMocks()
})

const violation: RuleViolation = {
  id: 1,
  rule_id: 1,
  rule_name: 'Concurrent Streams',
  user_name: 'alice',
  severity: 'warning',
  message: 'Too many concurrent streams',
  confidence_score: 92,
  occurred_at: '2024-06-15T12:00:00Z',
  created_at: '2024-06-15T12:00:00Z',
}

describe('ViolationsTable', () => {
  it('links the user name to /users/:name for admins', () => {
    mockUseAuth.mockReturnValue(makeAuthContext('admin'))
    renderWithRouter(<ViolationsTable violations={[violation]} />)

    const links = screen.getAllByRole('link', { name: 'alice' })
    expect(links.length).toBeGreaterThan(0)
    links.forEach(link => expect(link).toHaveAttribute('href', '/users/alice'))
  })

  it('renders the user name as plain text (no link) for non-admins, avoiding an AdminRoute dead-end', () => {
    mockUseAuth.mockReturnValue(makeAuthContext('viewer'))
    renderWithRouter(<ViolationsTable violations={[violation]} />)

    expect(screen.queryByRole('link', { name: 'alice' })).toBeNull()
    expect(screen.getAllByText('alice').length).toBeGreaterThan(0)
  })
})
